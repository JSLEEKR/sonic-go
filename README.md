# sonic-go

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-152-brightgreen?style=for-the-badge)](.)
[![Original](https://img.shields.io/badge/Original-Sonic_(Rust)-orange?style=for-the-badge)](https://github.com/valeriansaliou/sonic)

> **Fast, lightweight, schema-less search backend — reimplemented in Go.**

sonic-go is a Go reimplementation of [Sonic](https://github.com/valeriansaliou/sonic), the ~21K-star Rust search backend used by Crisp.chat. It provides an identifier-based search index: you push text associated with object IDs, then query to get matching IDs back. The actual documents live in your database — sonic-go just tells you _which_ ones match.

## Why This Exists

Sonic is an elegant piece of software — a search backend that does one thing well with minimal resources. But it's written in Rust and depends on RocksDB, making deployment and contribution harder for teams not in the Rust ecosystem.

sonic-go reimplements the core architecture in pure Go with zero external dependencies (beyond `gopkg.in/yaml.v3` for config). No RocksDB, no CGo, no complex build chains. Just `go build` and run.

**Key improvements over the original:**

| Feature | Sonic (Rust) | sonic-go |
|---------|-------------|----------|
| Build deps | RocksDB + 20 crates | Pure Go + 1 dep (yaml) |
| CGo required | Yes (RocksDB) | No |
| Cross-compile | Difficult | `GOOS=linux go build` |
| Typo correction | FST Levenshtein | Trie + Levenshtein |
| Storage | RocksDB (external) | In-memory + JSON persistence |
| Protocol | Sonic Channel (TCP) | Sonic Channel (TCP) — compatible |

## Architecture

```
                     TCP :1491
                        |
               +--------+--------+
               |  Channel Server  |
               |  (3 modes)       |
               +--------+--------+
               | search | ingest | control |
               +--------+--------+---------+
                        |
          +-------------+-------------+
          |             |             |
     +---------+   +---------+   +---------+
     | Lexer   |   | Store   |   | Suggest |
     | (token- |   | (in-mem |   | (trie)  |
     |  izer)  |   |  + disk)|   |         |
     +---------+   +---------+   +---------+
                        |
               +--------+--------+
               |  Index Engine    |
               |  (inverted idx)  |
               +-----------------+
```

### Data Hierarchy

```
Collection (e.g., "messages")
  -> Bucket (e.g., "user_123")
    -> Object (e.g., "conversation_456")
      -> Text (indexed words)
```

Collections contain buckets, buckets contain objects. Each object has associated text that gets tokenized and indexed. When you search, you get back object IDs — not the text itself.

## Installation

```bash
go install github.com/JSLEEKR/sonic-go/cmd/sonic@latest
```

Or build from source:

```bash
git clone https://github.com/JSLEEKR/sonic-go.git
cd sonic-go
go build -o sonic ./cmd/sonic/
```

## Quick Start

### 1. Start the Server

```bash
# With defaults (port 1491, password "SecretPassword")
./sonic

# With custom config
./sonic -addr :1491 -password MySecret -data ./my-data

# With config file
./sonic -config config.yaml
```

### 2. Connect and Ingest Data

```bash
# Connect via TCP (using netcat, telnet, or any TCP client)
nc localhost 1491

# You'll see:
# CONNECTED <sonic-go v1.0>

# Start in ingest mode
START ingest SecretPassword
# STARTED ingest protocol(1.0) buffer(20000)

# Push some data
PUSH messages user_1 conv_1 "Hello world, how are you doing today?"
# OK 4

PUSH messages user_1 conv_2 "The world is a beautiful place"
# OK 3

PUSH messages user_1 conv_3 "Programming in Go is fun"
# OK 3

QUIT
```

### 3. Search

```bash
nc localhost 1491
START search SecretPassword

# Query for "world"
QUERY messages user_1 "world"
# PENDING abc12345
# EVENT QUERY abc12345 conv_2 conv_1

# Suggest completions
SUGGEST messages user_1 "hel" LIMIT(5)
# PENDING xyz67890
# EVENT SUGGEST xyz67890 hello

QUIT
```

### 4. Administration

```bash
nc localhost 1491
START control SecretPassword

# Get server info
INFO
# RESULT uptime(0) clients_connected(1) heap_alloc(1234) goroutines(5)

# Persist data to disk
TRIGGER consolidate
# OK

QUIT
```

## Configuration

### YAML Config File

```yaml
server:
  log_level: info

store:
  data_dir: ./sonic-data
  retain_word_objects: 1000
  flush_interval: 5m
  consolidate_interval: 30s

channel:
  listen_addr: ":1491"
  auth_password: "SecretPassword"
  max_buffer_size: 20000
  search_pool_size: 4
```

### CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to YAML config file | (none) |
| `-addr` | Listen address | `:1491` |
| `-data` | Data directory | `./sonic-data` |
| `-password` | Auth password | `SecretPassword` |
| `-version` | Show version | |

CLI flags override config file values.

## Protocol Reference

sonic-go implements the Sonic Channel Protocol over TCP (port 1491 by default). Commands are newline-terminated.

### Connection Flow

```
Client                          Server
  |                                |
  |  <--- CONNECTED <sonic-go>    |
  |                                |
  |  START search <password> --->  |
  |  <--- STARTED search ...      |
  |                                |
  |  QUERY col bkt "text" ------> |
  |  <--- PENDING <event_id>      |
  |  <--- EVENT QUERY <id> <oids> |
  |                                |
  |  QUIT -----------------------> |
  |  <--- ENDED quit              |
```

### Search Mode Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `QUERY` | `QUERY <col> <bkt> "<text>" [LIMIT(n)] [OFFSET(n)]` | Search for matching objects |
| `SUGGEST` | `SUGGEST <col> <bkt> "<prefix>" [LIMIT(n)]` | Auto-complete suggestions |
| `LIST` | `LIST [<col> [<bkt>]]` | List collections or buckets |

### Ingest Mode Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `PUSH` | `PUSH <col> <bkt> <oid> "<text>"` | Index text for an object |
| `POP` | `POP <col> <bkt> <oid> ["<text>"]` | Remove terms (or entire object) |
| `COUNT` | `COUNT <col> [<bkt>]` | Count objects or buckets |
| `FLUSHC` | `FLUSHC <col>` | Flush entire collection |
| `FLUSHB` | `FLUSHB <col> <bkt>` | Flush a bucket |
| `FLUSHO` | `FLUSHO <col> <bkt> <oid>` | Flush a single object |

### Control Mode Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `TRIGGER` | `TRIGGER consolidate` | Save data to disk |
| `INFO` | `INFO` | Server runtime information |

### Common Commands (All Modes)

| Command | Description |
|---------|-------------|
| `PING` | Health check (returns `PONG`) |
| `HELP` | List available commands |
| `QUIT` | Close connection |

### Inline Options

| Option | Syntax | Default |
|--------|--------|---------|
| Limit | `LIMIT(n)` | 10 |
| Offset | `OFFSET(n)` | 0 |
| Language | `LANG(code)` | auto-detect |

## Package Structure

```
sonic-go/
├── cmd/sonic/           # CLI entry point
│   └── main.go
├── internal/
│   └── config/          # YAML configuration
│       ├── config.go
│       └── config_test.go
├── pkg/
│   ├── lexer/           # Text tokenization & stopwords
│   │   ├── lexer.go
│   │   ├── stopwords.go
│   │   └── lexer_test.go
│   ├── store/           # In-memory KV store + JSON persistence
│   │   ├── store.go
│   │   └── store_test.go
│   ├── index/           # Inverted index engine
│   │   ├── index.go
│   │   └── index_test.go
│   ├── search/          # Query engine (AND intersection, typo correction)
│   │   ├── search.go
│   │   └── search_test.go
│   ├── suggest/         # Trie-based auto-complete + Levenshtein fuzzy
│   │   ├── trie.go
│   │   └── trie_test.go
│   └── channel/         # TCP protocol server (3 modes)
│       ├── protocol.go
│       ├── server.go
│       ├── protocol_test.go
│       └── server_test.go
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

## Core Algorithms

### Inverted Index

For each document pushed, text is tokenized into terms. Each term is hashed (FNV-1a 32-bit) and mapped to a list of Internal Item IDs (IIDs). IIDs are monotonically increasing within each bucket.

```
Push "Hello world programming" for object "conv_1":
  1. Tokenize: ["hello", "world", "programming"]
  2. Resolve OID "conv_1" -> IID 1
  3. Hash terms: hello->0xABC, world->0xDEF, programming->0x123
  4. Store: term_0xABC -> [1], term_0xDEF -> [1], term_0x123 -> [1]
  5. Insert words into suggest trie
```

### Search (AND Intersection)

Query tokenizes the search text, looks up each term's IID list, and intersects them:

```
Query "hello world":
  1. Tokenize: ["hello", "world"]
  2. Lookup: hello -> [3, 2, 1], world -> [2, 1]
  3. Intersect: [2, 1]
  4. Resolve: IID 2 -> "conv_2", IID 1 -> "conv_1"
  5. Return: ["conv_2", "conv_1"]
```

### Auto-Complete (Trie)

Prefix-based suggestions using an in-memory trie, partitioned by collection:bucket:

```
Suggest "hel":
  1. Navigate trie: h -> e -> l
  2. Collect all words below: ["hello", "help", "helicopter"]
  3. Sort by insertion count (most popular first)
  4. Return top-N
```

### Typo Correction (Levenshtein)

Fuzzy matching with dynamic edit distance based on word length:

| Word Length | Max Edit Distance |
|------------|-------------------|
| 1-3 chars | 0 (exact only) |
| 4-6 chars | 1 |
| 7-9 chars | 2 |
| 10+ chars | 3 |

## Tokenization

The lexer performs:

1. **Lowercasing** — Unicode-aware
2. **Word splitting** — UAX29-style (split on non-letter/non-digit)
3. **Stopword removal** — 150+ English stopwords
4. **Length filtering** — Min 2 chars, max 64 chars
5. **Deduplication** — By FNV-1a hash

## Storage

sonic-go uses an in-memory store with JSON persistence:

- All index data lives in memory for fast access
- `TRIGGER consolidate` saves state to disk as JSON
- On startup, loads from `store.json` if it exists
- Data directory configurable via `-data` flag or config

For production use with large datasets, consider running behind a process manager that handles restarts, and use `TRIGGER consolidate` periodically to persist state.

## Compatibility

sonic-go implements the same TCP protocol as the original Sonic, so existing Sonic client libraries should work with minimal changes. Tested protocol commands:

- `START`, `QUIT`, `PING`, `HELP`
- `PUSH`, `POP`, `COUNT`, `FLUSHC`, `FLUSHB`, `FLUSHO`
- `QUERY`, `SUGGEST`, `LIST`
- `TRIGGER`, `INFO`
- Inline options: `LIMIT()`, `OFFSET()`, `LANG()`

## Testing

```bash
# Run all tests
go test ./... -count=1

# Verbose output
go test -v ./... -count=1

# Run specific package
go test -v ./pkg/lexer/

# Run with race detector
go test -race ./...
```

### Test Coverage by Package

| Package | Tests | Coverage |
|---------|-------|----------|
| `pkg/lexer` | 20 | Tokenization, stopwords, hashing, edge cases |
| `pkg/store` | 27 | CRUD, persistence, isolation, flush, counters |
| `pkg/suggest` | 22 | Trie ops, fuzzy search, Levenshtein, isolation |
| `pkg/search` | 13 | Query, intersection, pagination, suggest |
| `pkg/index` | 17 | Push, pop, flush, trie cleanup, multi-bucket/collection |
| `pkg/channel` | 44 | Protocol parsing, TCP server, all 3 modes |
| `internal/config` | 9 | Defaults, validation, YAML loading |
| **Total** | **152** | |

## Differences from Original Sonic

| Aspect | Sonic (Rust) | sonic-go |
|--------|-------------|----------|
| Language | Rust | Go |
| Storage | RocksDB + FST | In-memory + JSON |
| Hashing | xxHash32 | FNV-1a 32-bit |
| Auto-complete | FST (finite-state transducer) | Trie |
| Fuzzy matching | fst-levenshtein | Custom Levenshtein |
| Config format | TOML | YAML |
| CJK tokenizers | Jieba + Lindera | Not included |
| Dependencies | ~25 crates | 1 (yaml.v3) |
| Binary size | ~8MB | ~7MB |

## License

MIT License. See [LICENSE](LICENSE).

## Credits

- **Original Sonic**: [valeriansaliou/sonic](https://github.com/valeriansaliou/sonic) — The Rust search backend this project reimplements.
- **JSLEEKR**: Reimplementation in Go as part of the V2 Reimplement & Compare pipeline.
