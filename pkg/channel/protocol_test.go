package channel

import (
	"testing"
)

func TestParseModeSearch(t *testing.T) {
	m, err := ParseMode("search")
	if err != nil {
		t.Fatal(err)
	}
	if m != ModeSearch {
		t.Errorf("expected ModeSearch, got %v", m)
	}
}

func TestParseModeIngest(t *testing.T) {
	m, err := ParseMode("ingest")
	if err != nil {
		t.Fatal(err)
	}
	if m != ModeIngest {
		t.Errorf("expected ModeIngest, got %v", m)
	}
}

func TestParseModeControl(t *testing.T) {
	m, err := ParseMode("control")
	if err != nil {
		t.Fatal(err)
	}
	if m != ModeControl {
		t.Errorf("expected ModeControl, got %v", m)
	}
}

func TestParseModeInvalid(t *testing.T) {
	_, err := ParseMode("invalid")
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestParseModeCaseInsensitive(t *testing.T) {
	m, err := ParseMode("SEARCH")
	if err != nil {
		t.Fatal(err)
	}
	if m != ModeSearch {
		t.Errorf("expected ModeSearch, got %v", m)
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeSearch, "search"},
		{ModeIngest, "ingest"},
		{ModeControl, "control"},
		{ModeUnset, "unset"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.expected {
			t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.expected)
		}
	}
}

func TestParseCommandPing(t *testing.T) {
	cmd, err := ParseCommand("PING")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "PING" {
		t.Errorf("expected PING, got %s", cmd.Action)
	}
}

func TestParseCommandQuery(t *testing.T) {
	cmd, err := ParseCommand(`QUERY messages user_123 "hello world" LIMIT(10) OFFSET(0)`)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "QUERY" {
		t.Errorf("expected QUERY, got %s", cmd.Action)
	}
	if cmd.Collection != "messages" {
		t.Errorf("expected messages, got %s", cmd.Collection)
	}
	if cmd.Bucket != "user_123" {
		t.Errorf("expected user_123, got %s", cmd.Bucket)
	}
	if cmd.Text != "hello world" {
		t.Errorf("expected 'hello world', got %q", cmd.Text)
	}
	if cmd.Limit != 10 {
		t.Errorf("expected limit 10, got %d", cmd.Limit)
	}
	if cmd.Offset != 0 {
		t.Errorf("expected offset 0, got %d", cmd.Offset)
	}
}

func TestParseCommandPush(t *testing.T) {
	cmd, err := ParseCommand(`PUSH messages user_123 conversation_1 "Hello world, how are you?"`)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "PUSH" {
		t.Errorf("expected PUSH, got %s", cmd.Action)
	}
	if cmd.Object != "conversation_1" {
		t.Errorf("expected conversation_1, got %s", cmd.Object)
	}
	if cmd.Text != "Hello world, how are you?" {
		t.Errorf("expected text, got %q", cmd.Text)
	}
}

func TestParseCommandSuggest(t *testing.T) {
	cmd, err := ParseCommand(`SUGGEST messages user_123 "hel" LIMIT(5)`)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "SUGGEST" {
		t.Errorf("expected SUGGEST, got %s", cmd.Action)
	}
	if cmd.Text != "hel" {
		t.Errorf("expected 'hel', got %q", cmd.Text)
	}
	if cmd.Limit != 5 {
		t.Errorf("expected limit 5, got %d", cmd.Limit)
	}
}

func TestParseCommandPop(t *testing.T) {
	cmd, err := ParseCommand(`POP messages user_123 conversation_1 "hello"`)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "POP" {
		t.Errorf("expected POP, got %s", cmd.Action)
	}
	if cmd.Object != "conversation_1" {
		t.Errorf("expected conversation_1, got %s", cmd.Object)
	}
}

func TestParseCommandCount(t *testing.T) {
	cmd, err := ParseCommand("COUNT messages user_123")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "COUNT" {
		t.Errorf("expected COUNT, got %s", cmd.Action)
	}
	if cmd.Collection != "messages" {
		t.Errorf("expected messages, got %s", cmd.Collection)
	}
	if cmd.Bucket != "user_123" {
		t.Errorf("expected user_123, got %s", cmd.Bucket)
	}
}

func TestParseCommandFlushC(t *testing.T) {
	cmd, err := ParseCommand("FLUSHC messages")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "FLUSHC" {
		t.Errorf("expected FLUSHC, got %s", cmd.Action)
	}
	if cmd.Collection != "messages" {
		t.Errorf("expected messages, got %s", cmd.Collection)
	}
}

func TestParseCommandFlushB(t *testing.T) {
	cmd, err := ParseCommand("FLUSHB messages user_123")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "FLUSHB" {
		t.Errorf("expected FLUSHB, got %s", cmd.Action)
	}
}

func TestParseCommandFlushO(t *testing.T) {
	cmd, err := ParseCommand("FLUSHO messages user_123 conv_1")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "FLUSHO" {
		t.Errorf("expected FLUSHO, got %s", cmd.Action)
	}
	if cmd.Object != "conv_1" {
		t.Errorf("expected conv_1, got %s", cmd.Object)
	}
}

func TestParseCommandStart(t *testing.T) {
	cmd, err := ParseCommand("START search SecretPassword")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "START" {
		t.Errorf("expected START, got %s", cmd.Action)
	}
	if cmd.Text != "search" {
		t.Errorf("expected 'search', got %q", cmd.Text)
	}
	if cmd.Object != "SecretPassword" {
		t.Errorf("expected SecretPassword, got %q", cmd.Object)
	}
}

func TestParseCommandLang(t *testing.T) {
	cmd, err := ParseCommand(`QUERY messages user_123 "hello" LANG(eng)`)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Lang != "eng" {
		t.Errorf("expected lang 'eng', got %q", cmd.Lang)
	}
}

func TestParseCommandEmpty(t *testing.T) {
	_, err := ParseCommand("")
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestParseCommandUnknown(t *testing.T) {
	_, err := ParseCommand("UNKNOWN arg1 arg2")
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestParseCommandQueryMissingArgs(t *testing.T) {
	_, err := ParseCommand("QUERY")
	if err == nil {
		t.Error("expected error for QUERY without args")
	}
}

func TestParseCommandPushMissingArgs(t *testing.T) {
	_, err := ParseCommand("PUSH messages")
	if err == nil {
		t.Error("expected error for PUSH without enough args")
	}
}

func TestParseCommandTrigger(t *testing.T) {
	cmd, err := ParseCommand("TRIGGER consolidate")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "TRIGGER" {
		t.Errorf("expected TRIGGER, got %s", cmd.Action)
	}
	if cmd.Text != "consolidate" {
		t.Errorf("expected 'consolidate', got %q", cmd.Text)
	}
}

func TestParseCommandList(t *testing.T) {
	cmd, err := ParseCommand("LIST messages")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != "LIST" {
		t.Errorf("expected LIST, got %s", cmd.Action)
	}
	if cmd.Collection != "messages" {
		t.Errorf("expected messages, got %s", cmd.Collection)
	}
}

func TestValidateCommandSearchMode(t *testing.T) {
	valid := []string{"QUERY", "SUGGEST", "LIST", "PING", "HELP", "QUIT"}
	for _, cmd := range valid {
		if err := ValidateCommandForMode(cmd, ModeSearch); err != nil {
			t.Errorf("command %s should be valid in search mode: %v", cmd, err)
		}
	}
	if err := ValidateCommandForMode("PUSH", ModeSearch); err == nil {
		t.Error("PUSH should not be valid in search mode")
	}
}

func TestValidateCommandIngestMode(t *testing.T) {
	valid := []string{"PUSH", "POP", "COUNT", "FLUSHC", "FLUSHB", "FLUSHO", "PING", "HELP", "QUIT"}
	for _, cmd := range valid {
		if err := ValidateCommandForMode(cmd, ModeIngest); err != nil {
			t.Errorf("command %s should be valid in ingest mode: %v", cmd, err)
		}
	}
	if err := ValidateCommandForMode("QUERY", ModeIngest); err == nil {
		t.Error("QUERY should not be valid in ingest mode")
	}
}

func TestValidateCommandControlMode(t *testing.T) {
	valid := []string{"TRIGGER", "INFO", "PING", "HELP", "QUIT"}
	for _, cmd := range valid {
		if err := ValidateCommandForMode(cmd, ModeControl); err != nil {
			t.Errorf("command %s should be valid in control mode: %v", cmd, err)
		}
	}
}

func TestValidateCommandUnsetMode(t *testing.T) {
	if err := ValidateCommandForMode("QUERY", ModeUnset); err == nil {
		t.Error("expected error for unset mode")
	}
}

func TestFormatResponse(t *testing.T) {
	resp := FormatResponse("OK", "3")
	if resp != "OK 3\r\n" {
		t.Errorf("expected 'OK 3\\r\\n', got %q", resp)
	}
}
