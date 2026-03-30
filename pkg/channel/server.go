package channel

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JSLEEKR/sonic-go/pkg/index"
	"github.com/JSLEEKR/sonic-go/pkg/search"
	"github.com/JSLEEKR/sonic-go/pkg/store"
	"github.com/JSLEEKR/sonic-go/pkg/suggest"
)

const (
	protocolVersion = "1.0"
	bufferSize      = 20000
)

// Server is the TCP channel server.
type Server struct {
	addr     string
	password string

	store  *store.Store
	trie   *suggest.Trie
	idx    *index.Index
	engine *search.Engine

	listener   net.Listener
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	connCount  atomic.Int64
	maxBuffer  int
}

// NewServer creates a new channel server.
func NewServer(addr, password string, s *store.Store, t *suggest.Trie, idx *index.Index, eng *search.Engine, maxBuf int) *Server {
	if maxBuf <= 0 {
		maxBuf = bufferSize
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		addr:      addr,
		password:  password,
		store:     s,
		trie:      t,
		idx:       idx,
		engine:    eng,
		ctx:       ctx,
		cancel:    cancel,
		maxBuffer: maxBuf,
	}
}

// Start begins listening for connections.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.addr, err)
	}
	s.listener = ln
	log.Printf("sonic-go channel server listening on %s", s.addr)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-s.ctx.Done():
					return
				default:
					log.Printf("accept error: %v", err)
					continue
				}
			}
			s.connCount.Add(1)
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer s.connCount.Add(-1)
				s.handleConnection(conn)
			}()
		}
	}()

	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	s.cancel()
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	return nil
}

// Addr returns the listener address (useful when using :0 port).
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

// ConnCount returns the current number of active connections.
func (s *Server) ConnCount() int64 {
	return s.connCount.Load()
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReaderSize(conn, s.maxBuffer)
	mode := ModeUnset
	authenticated := false

	// Send banner
	banner := fmt.Sprintf("CONNECTED <sonic-go v%s>", protocolVersion)
	conn.Write([]byte(banner + "\r\n"))

	for {
		select {
		case <-s.ctx.Done():
			conn.Write([]byte("ERR server shutting down\r\n"))
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		cmd, err := ParseCommand(line)
		if err != nil {
			conn.Write([]byte(fmt.Sprintf("ERR %s\r\n", err.Error())))
			continue
		}

		// Handle START (authentication)
		if cmd.Action == "START" {
			if authenticated {
				conn.Write([]byte("ERR already started\r\n"))
				continue
			}
			m, err := ParseMode(cmd.Text)
			if err != nil {
				conn.Write([]byte(fmt.Sprintf("ERR %s\r\n", err.Error())))
				continue
			}
			if s.password != "" && cmd.Object != s.password {
				conn.Write([]byte("ERR authentication failed\r\n"))
				continue
			}
			mode = m
			authenticated = true
			resp := fmt.Sprintf("STARTED %s protocol(%s) buffer(%d)",
				mode.String(), protocolVersion, s.maxBuffer)
			conn.Write([]byte(resp + "\r\n"))
			continue
		}

		if !authenticated {
			conn.Write([]byte("ERR not started, use START command\r\n"))
			continue
		}

		// Validate command for current mode
		if err := ValidateCommandForMode(cmd.Action, mode); err != nil {
			conn.Write([]byte(fmt.Sprintf("ERR %s\r\n", err.Error())))
			continue
		}

		response := s.executeCommand(cmd, mode)
		conn.Write([]byte(response))

		// Close connection on QUIT
		if cmd.Action == "QUIT" {
			return
		}
	}
}

const maxIdentifierLen = 256

func validateIdentifiers(cmd *Command) error {
	if len(cmd.Collection) > maxIdentifierLen {
		return fmt.Errorf("collection name too long (max %d)", maxIdentifierLen)
	}
	if len(cmd.Bucket) > maxIdentifierLen {
		return fmt.Errorf("bucket name too long (max %d)", maxIdentifierLen)
	}
	if len(cmd.Object) > maxIdentifierLen {
		return fmt.Errorf("object name too long (max %d)", maxIdentifierLen)
	}
	return nil
}

func (s *Server) executeCommand(cmd *Command, mode Mode) string {
	if err := validateIdentifiers(cmd); err != nil {
		return fmt.Sprintf("ERR %s\r\n", err.Error())
	}

	switch cmd.Action {
	case "PING":
		return "PONG\r\n"
	case "QUIT":
		return "ENDED quit\r\n"
	case "HELP":
		return s.helpText(mode)

	// Search mode
	case "QUERY":
		return s.handleQuery(cmd)
	case "SUGGEST":
		return s.handleSuggest(cmd)
	case "LIST":
		return s.handleList(cmd)

	// Ingest mode
	case "PUSH":
		return s.handlePush(cmd)
	case "POP":
		return s.handlePop(cmd)
	case "COUNT":
		return s.handleCount(cmd)
	case "FLUSHC":
		return s.handleFlushC(cmd)
	case "FLUSHB":
		return s.handleFlushB(cmd)
	case "FLUSHO":
		return s.handleFlushO(cmd)

	// Control mode
	case "TRIGGER":
		return s.handleTrigger(cmd)
	case "INFO":
		return s.handleInfo()

	default:
		return fmt.Sprintf("ERR unknown command %s\r\n", cmd.Action)
	}
}

func (s *Server) handleQuery(cmd *Command) string {
	eventID := randomEventID()
	opts := search.QueryOptions{
		Limit:  cmd.Limit,
		Offset: cmd.Offset,
	}
	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	results := s.engine.Query(cmd.Collection, cmd.Bucket, cmd.Text, opts)
	resultStr := strings.Join(results, " ")

	// Sonic protocol: send PENDING then EVENT
	pending := fmt.Sprintf("PENDING %s\r\n", eventID)
	event := fmt.Sprintf("EVENT QUERY %s %s\r\n", eventID, resultStr)

	return pending + event
}

func (s *Server) handleSuggest(cmd *Command) string {
	eventID := randomEventID()
	limit := cmd.Limit
	if limit <= 0 {
		limit = 5
	}

	results := s.engine.Suggest(cmd.Collection, cmd.Bucket, cmd.Text, limit)
	resultStr := strings.Join(results, " ")

	pending := fmt.Sprintf("PENDING %s\r\n", eventID)
	event := fmt.Sprintf("EVENT SUGGEST %s %s\r\n", eventID, resultStr)

	return pending + event
}

func (s *Server) handleList(cmd *Command) string {
	if cmd.Collection == "" {
		collections := s.store.ListCollections()
		return fmt.Sprintf("RESULT %s\r\n", strings.Join(collections, " "))
	}
	buckets := s.store.ListBuckets(cmd.Collection)
	return fmt.Sprintf("RESULT %s\r\n", strings.Join(buckets, " "))
}

func (s *Server) handlePush(cmd *Command) string {
	if cmd.Text == "" {
		return "ERR PUSH requires text\r\n"
	}
	count := s.idx.Push(cmd.Collection, cmd.Bucket, cmd.Object, cmd.Text)
	return fmt.Sprintf("OK %d\r\n", count)
}

func (s *Server) handlePop(cmd *Command) string {
	count := s.idx.Pop(cmd.Collection, cmd.Bucket, cmd.Object, cmd.Text)
	return fmt.Sprintf("OK %d\r\n", count)
}

func (s *Server) handleCount(cmd *Command) string {
	if cmd.Bucket == "" {
		// Count all buckets in collection
		buckets := s.store.ListBuckets(cmd.Collection)
		return fmt.Sprintf("RESULT %d\r\n", len(buckets))
	}
	count := s.idx.Count(cmd.Collection, cmd.Bucket)
	return fmt.Sprintf("RESULT %d\r\n", count)
}

func (s *Server) handleFlushC(cmd *Command) string {
	count := s.idx.FlushCollection(cmd.Collection)
	return fmt.Sprintf("OK %d\r\n", count)
}

func (s *Server) handleFlushB(cmd *Command) string {
	count := s.idx.FlushBucket(cmd.Collection, cmd.Bucket)
	return fmt.Sprintf("OK %d\r\n", count)
}

func (s *Server) handleFlushO(cmd *Command) string {
	count := s.idx.FlushObject(cmd.Collection, cmd.Bucket, cmd.Object)
	return fmt.Sprintf("OK %d\r\n", count)
}

func (s *Server) handleTrigger(cmd *Command) string {
	switch strings.ToLower(cmd.Text) {
	case "consolidate":
		// Save store to disk
		if err := s.store.SaveToDisk(); err != nil {
			return fmt.Sprintf("ERR consolidate failed: %s\r\n", err.Error())
		}
		return "OK\r\n"
	default:
		return fmt.Sprintf("ERR unknown trigger: %s\r\n", cmd.Text)
	}
}

func (s *Server) handleInfo() string {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	info := fmt.Sprintf("RESULT uptime(0) clients_connected(%d) "+
		"heap_alloc(%d) heap_sys(%d) goroutines(%d)",
		s.ConnCount(),
		memStats.HeapAlloc,
		memStats.HeapSys,
		runtime.NumGoroutine(),
	)
	return info + "\r\n"
}

func (s *Server) helpText(mode Mode) string {
	switch mode {
	case ModeSearch:
		return "RESULT QUERY SUGGEST LIST PING HELP QUIT\r\n"
	case ModeIngest:
		return "RESULT PUSH POP COUNT FLUSHC FLUSHB FLUSHO PING HELP QUIT\r\n"
	case ModeControl:
		return "RESULT TRIGGER INFO PING HELP QUIT\r\n"
	default:
		return "RESULT START PING HELP QUIT\r\n"
	}
}

func randomEventID() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
