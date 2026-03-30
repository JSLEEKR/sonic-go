// Package channel implements the Sonic Channel Protocol (TCP).
// It supports three modes: search, ingest, and control.
package channel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Mode represents a connection mode.
type Mode int

const (
	ModeUnset   Mode = iota
	ModeSearch       // QUERY, SUGGEST, LIST, PING, HELP, QUIT
	ModeIngest       // PUSH, POP, COUNT, FLUSHC, FLUSHB, FLUSHO, PING, HELP, QUIT
	ModeControl      // TRIGGER, INFO, PING, HELP, QUIT
)

// String returns the mode name.
func (m Mode) String() string {
	switch m {
	case ModeSearch:
		return "search"
	case ModeIngest:
		return "ingest"
	case ModeControl:
		return "control"
	default:
		return "unset"
	}
}

// ParseMode parses a mode string.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(s) {
	case "search":
		return ModeSearch, nil
	case "ingest":
		return ModeIngest, nil
	case "control":
		return ModeControl, nil
	default:
		return ModeUnset, fmt.Errorf("unknown mode: %q", s)
	}
}

// Command represents a parsed protocol command.
type Command struct {
	Action     string
	Collection string
	Bucket     string
	Object     string
	Text       string
	Limit      int
	Offset     int
	Lang       string
}

var (
	limitRe  = regexp.MustCompile(`LIMIT\((\d+)\)`)
	offsetRe = regexp.MustCompile(`OFFSET\((\d+)\)`)
	langRe   = regexp.MustCompile(`LANG\((\w+)\)`)
)

// ParseCommand parses a raw protocol line into a Command.
func ParseCommand(line string) (*Command, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty command")
	}

	cmd := &Command{}

	// Extract inline options before splitting
	if m := limitRe.FindStringSubmatch(line); len(m) > 1 {
		cmd.Limit, _ = strconv.Atoi(m[1])
		line = strings.Replace(line, m[0], "", 1)
	}
	if m := offsetRe.FindStringSubmatch(line); len(m) > 1 {
		cmd.Offset, _ = strconv.Atoi(m[1])
		line = strings.Replace(line, m[0], "", 1)
	}
	if m := langRe.FindStringSubmatch(line); len(m) > 1 {
		cmd.Lang = m[1]
		line = strings.Replace(line, m[0], "", 1)
	}

	line = strings.TrimSpace(line)

	// Extract quoted text (the text payload in PUSH/QUERY)
	if idx := strings.Index(line, "\""); idx >= 0 {
		endIdx := strings.LastIndex(line, "\"")
		if endIdx > idx {
			cmd.Text = line[idx+1 : endIdx]
			line = line[:idx] + line[endIdx+1:]
			line = strings.TrimSpace(line)
		}
	}

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command after parsing")
	}

	cmd.Action = strings.ToUpper(parts[0])

	switch cmd.Action {
	case "QUERY", "SUGGEST":
		if len(parts) < 3 {
			return nil, fmt.Errorf("%s requires collection and bucket", cmd.Action)
		}
		cmd.Collection = parts[1]
		cmd.Bucket = parts[2]
		// For SUGGEST, the text might be unquoted (single word)
		if cmd.Text == "" && len(parts) > 3 {
			cmd.Text = parts[3]
		}

	case "PUSH":
		if len(parts) < 4 {
			return nil, fmt.Errorf("PUSH requires collection, bucket, object")
		}
		cmd.Collection = parts[1]
		cmd.Bucket = parts[2]
		cmd.Object = parts[3]

	case "POP":
		if len(parts) < 4 {
			return nil, fmt.Errorf("POP requires collection, bucket, object")
		}
		cmd.Collection = parts[1]
		cmd.Bucket = parts[2]
		cmd.Object = parts[3]

	case "COUNT":
		if len(parts) < 2 {
			return nil, fmt.Errorf("COUNT requires at least collection")
		}
		cmd.Collection = parts[1]
		if len(parts) > 2 {
			cmd.Bucket = parts[2]
		}
		if len(parts) > 3 {
			cmd.Object = parts[3]
		}

	case "FLUSHC":
		if len(parts) < 2 {
			return nil, fmt.Errorf("FLUSHC requires collection")
		}
		cmd.Collection = parts[1]

	case "FLUSHB":
		if len(parts) < 3 {
			return nil, fmt.Errorf("FLUSHB requires collection and bucket")
		}
		cmd.Collection = parts[1]
		cmd.Bucket = parts[2]

	case "FLUSHO":
		if len(parts) < 4 {
			return nil, fmt.Errorf("FLUSHO requires collection, bucket, object")
		}
		cmd.Collection = parts[1]
		cmd.Bucket = parts[2]
		cmd.Object = parts[3]

	case "LIST":
		if len(parts) > 1 {
			cmd.Collection = parts[1]
		}
		if len(parts) > 2 {
			cmd.Bucket = parts[2]
		}

	case "TRIGGER":
		if len(parts) > 1 {
			cmd.Text = parts[1]
		}

	case "PING", "HELP", "QUIT", "INFO":
		// No additional arguments needed

	case "START":
		if len(parts) < 2 {
			return nil, fmt.Errorf("START requires mode")
		}
		cmd.Text = parts[1] // mode
		if len(parts) > 2 {
			cmd.Object = parts[2] // password
		}

	default:
		return nil, fmt.Errorf("unknown command: %s", cmd.Action)
	}

	return cmd, nil
}

// FormatResponse formats a response to send back over the protocol.
func FormatResponse(parts ...string) string {
	return strings.Join(parts, " ") + "\r\n"
}

// ValidateCommandForMode checks if a command is valid for the given mode.
func ValidateCommandForMode(action string, mode Mode) error {
	common := map[string]bool{"PING": true, "HELP": true, "QUIT": true}
	if common[action] {
		return nil
	}

	switch mode {
	case ModeSearch:
		valid := map[string]bool{"QUERY": true, "SUGGEST": true, "LIST": true}
		if !valid[action] {
			return fmt.Errorf("command %s not available in search mode", action)
		}
	case ModeIngest:
		valid := map[string]bool{"PUSH": true, "POP": true, "COUNT": true, "FLUSHC": true, "FLUSHB": true, "FLUSHO": true}
		if !valid[action] {
			return fmt.Errorf("command %s not available in ingest mode", action)
		}
	case ModeControl:
		valid := map[string]bool{"TRIGGER": true, "INFO": true}
		if !valid[action] {
			return fmt.Errorf("command %s not available in control mode", action)
		}
	default:
		return fmt.Errorf("must START a session first")
	}

	return nil
}
