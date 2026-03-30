package channel

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/JSLEEKR/sonic-go/pkg/index"
	"github.com/JSLEEKR/sonic-go/pkg/search"
	"github.com/JSLEEKR/sonic-go/pkg/store"
	"github.com/JSLEEKR/sonic-go/pkg/suggest"
)

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	s := store.New(t.TempDir())
	tr := suggest.NewTrie()
	idx := index.New(s, tr, 1000)
	eng := search.New(s, tr)

	srv := NewServer(":0", "test123", s, tr, idx, eng, 20000)
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	t.Cleanup(func() { srv.Stop() })

	return srv, srv.Addr()
}

func connect(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	reader := bufio.NewReader(conn)

	// Read banner
	banner, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read banner: %v", err)
	}
	if !strings.HasPrefix(banner, "CONNECTED") {
		t.Fatalf("unexpected banner: %s", banner)
	}

	return conn, reader
}

func sendCmd(t *testing.T, conn net.Conn, reader *bufio.Reader, cmd string) string {
	t.Helper()
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err := fmt.Fprintf(conn, "%s\n", cmd)
	if err != nil {
		t.Fatalf("failed to send: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	return strings.TrimSpace(resp)
}

func TestServerBanner(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	banner, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(banner), "CONNECTED") {
		t.Errorf("unexpected banner: %s", banner)
	}
}

func TestServerAuth(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	resp := sendCmd(t, conn, reader, "START search test123")
	if !strings.HasPrefix(resp, "STARTED search") {
		t.Errorf("expected STARTED, got %s", resp)
	}
}

func TestServerAuthFailed(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	resp := sendCmd(t, conn, reader, "START search wrongpassword")
	if !strings.HasPrefix(resp, "ERR") {
		t.Errorf("expected ERR for wrong password, got %s", resp)
	}
}

func TestServerPing(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START search test123")
	resp := sendCmd(t, conn, reader, "PING")
	if resp != "PONG" {
		t.Errorf("expected PONG, got %s", resp)
	}
}

func TestServerHelp(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START search test123")
	resp := sendCmd(t, conn, reader, "HELP")
	if !strings.HasPrefix(resp, "RESULT") {
		t.Errorf("expected RESULT, got %s", resp)
	}
}

func TestServerIngestPush(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START ingest test123")
	resp := sendCmd(t, conn, reader, `PUSH messages user_1 conv_1 "hello world programming"`)
	if !strings.HasPrefix(resp, "OK") {
		t.Errorf("expected OK, got %s", resp)
	}
}

func TestServerIngestCount(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START ingest test123")
	sendCmd(t, conn, reader, `PUSH messages user_1 conv_1 "hello world"`)
	sendCmd(t, conn, reader, `PUSH messages user_1 conv_2 "hello golang"`)

	resp := sendCmd(t, conn, reader, "COUNT messages user_1")
	if !strings.Contains(resp, "2") {
		t.Errorf("expected count 2, got %s", resp)
	}
}

func TestServerIngestFlush(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START ingest test123")
	sendCmd(t, conn, reader, `PUSH messages user_1 conv_1 "hello world"`)
	resp := sendCmd(t, conn, reader, "FLUSHB messages user_1")
	if !strings.HasPrefix(resp, "OK") {
		t.Errorf("expected OK, got %s", resp)
	}

	resp = sendCmd(t, conn, reader, "COUNT messages user_1")
	if !strings.Contains(resp, "0") {
		t.Errorf("expected count 0 after flush, got %s", resp)
	}
}

func TestServerSearchQuery(t *testing.T) {
	_, addr := setupTestServer(t)

	// First push some data via ingest connection
	conn1, reader1 := connect(t, addr)
	sendCmd(t, conn1, reader1, "START ingest test123")
	sendCmd(t, conn1, reader1, `PUSH messages user_1 conv_1 "hello world programming"`)
	sendCmd(t, conn1, reader1, `PUSH messages user_1 conv_2 "hello golang testing"`)
	conn1.Close()

	// Search via search connection
	conn2, reader2 := connect(t, addr)
	defer conn2.Close()
	sendCmd(t, conn2, reader2, "START search test123")

	// Send QUERY - expect PENDING then EVENT
	conn2.SetWriteDeadline(time.Now().Add(2 * time.Second))
	fmt.Fprintf(conn2, "QUERY messages user_1 \"hello\" LIMIT(10)\n")

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	pending, _ := reader2.ReadString('\n')
	if !strings.HasPrefix(strings.TrimSpace(pending), "PENDING") {
		t.Errorf("expected PENDING, got %s", pending)
	}

	event, _ := reader2.ReadString('\n')
	if !strings.HasPrefix(strings.TrimSpace(event), "EVENT QUERY") {
		t.Errorf("expected EVENT QUERY, got %s", event)
	}
}

func TestServerModeRestriction(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START search test123")

	// PUSH should not work in search mode
	resp := sendCmd(t, conn, reader, `PUSH messages user_1 conv_1 "hello"`)
	if !strings.HasPrefix(resp, "ERR") {
		t.Errorf("expected ERR for PUSH in search mode, got %s", resp)
	}
}

func TestServerNotStarted(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	resp := sendCmd(t, conn, reader, "PING")
	// PING before START should work (common command, but need auth first)
	if !strings.HasPrefix(resp, "ERR") {
		t.Errorf("expected ERR before START, got %s", resp)
	}
}

func TestServerQuit(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START search test123")
	resp := sendCmd(t, conn, reader, "QUIT")
	if !strings.HasPrefix(resp, "ENDED") {
		t.Errorf("expected ENDED, got %s", resp)
	}
}

func TestServerControlInfo(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START control test123")
	resp := sendCmd(t, conn, reader, "INFO")
	if !strings.HasPrefix(resp, "RESULT") {
		t.Errorf("expected RESULT, got %s", resp)
	}
	if !strings.Contains(resp, "goroutines") {
		t.Errorf("expected goroutines info, got %s", resp)
	}
}

func TestServerControlTrigger(t *testing.T) {
	_, addr := setupTestServer(t)
	conn, reader := connect(t, addr)
	defer conn.Close()

	sendCmd(t, conn, reader, "START control test123")
	resp := sendCmd(t, conn, reader, "TRIGGER consolidate")
	if !strings.HasPrefix(resp, "OK") {
		t.Errorf("expected OK, got %s", resp)
	}
}

func TestServerConnCount(t *testing.T) {
	srv, addr := setupTestServer(t)
	conn, _ := connect(t, addr)
	defer conn.Close()

	// Give a moment for the connection to register
	time.Sleep(50 * time.Millisecond)
	count := srv.ConnCount()
	if count < 1 {
		t.Errorf("expected at least 1 connection, got %d", count)
	}
}

func TestServerSuggest(t *testing.T) {
	_, addr := setupTestServer(t)

	// Push data
	conn1, reader1 := connect(t, addr)
	sendCmd(t, conn1, reader1, "START ingest test123")
	sendCmd(t, conn1, reader1, `PUSH messages user_1 conv_1 "helicopter landing pad"`)
	conn1.Close()

	// Search
	conn2, reader2 := connect(t, addr)
	defer conn2.Close()
	sendCmd(t, conn2, reader2, "START search test123")

	conn2.SetWriteDeadline(time.Now().Add(2 * time.Second))
	fmt.Fprintf(conn2, "SUGGEST messages user_1 \"hel\" LIMIT(5)\n")

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	pending, _ := reader2.ReadString('\n')
	if !strings.HasPrefix(strings.TrimSpace(pending), "PENDING") {
		t.Errorf("expected PENDING, got %s", pending)
	}

	event, _ := reader2.ReadString('\n')
	if !strings.HasPrefix(strings.TrimSpace(event), "EVENT SUGGEST") {
		t.Errorf("expected EVENT SUGGEST, got %s", event)
	}
}
