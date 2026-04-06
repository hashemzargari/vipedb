# VipeDB

**VipeDB** is an all-in-one semantic search tool that combines a vector database and embedding models in a single executable binary. It's designed to be a drop-in replacement for `grep` with semantic search capabilities — and it's built on a stack that most Python-based tools can't touch.

![VipeDB demo](demo.gif)

## Features

- **Single Binary**: Download and run — no complex setup, no runtime dependencies
- **Auto-Setup**: `vipe init` creates `~/.vipe/`, downloads the default model, and you're ready
- **Local Workspace Override**: Drop a `.vipe` directory in any project to get project-scoped index, cache, and config — automatically detected
- **Transparent Workspace Logging**: Every command prints which workspace (local or global) is active, so there's never confusion
- **Built-in Embedding Models**: Includes multilingual-e5-small-fp16 and bge-small-en-v1.5
- **Docker Ready**: Ship as a sidecar container for log analysis alongside your services
- **Real-time Log Streaming**: Continuously ingest logs via `stdin` pipe or file tailing — model loaded once, stays hot in memory
- **Batched Worker Pool**: Lines are buffered and embedded in concurrent batches (configurable batch size, flush interval, and worker count) for high-throughput ingestion
- **Agent-Friendly JSON Output**: `--json` flag on `search` and `grep` emits strict JSON arrays — pipe directly into `jq`, monitoring dashboards, or LLM agents
- **Colored Terminal UX**: Similarity scores are color-coded (green ≥0.75, yellow ≥0.50, red <0.50) with source highlights for humans
- **Grep-like Interface**: Familiar command-line interface for semantic search
- **Vector Database**: Persistent storage for embeddings with atomic writes and automatic deduplication
- **Intelligent Caching**: SHA256-based file hashing, auto-skip cached files
- **Cache Management**: Manual and automatic cache cleanup with configurable retention
- **YAML Configuration**: Easy-to-use config file for customizing models and settings
- **Cross-platform**: Works on Linux, macOS, and Windows (WebAssembly support planned)

## Why VipeDB? (Under the Hood)

Most semantic search tools are wrappers around Python runtimes, PyTorch, and heavy CUDA stacks. VipeDB takes a completely different path.

### Powered by [MemPipe](https://github.com/GoMemPipe/mempipe) — Zero-GC, Arena-Backed Inference

At its core, VipeDB runs on **MemPipe**: a zero-dependency, zero-allocation, arena-backed pipeline and ONNX inference engine written **purely in Go**. No Python. No C++ bindings. No CGo. No PyTorch.

A single `make([]byte, N)` allocates the entire working set. Every subsequent tensor read/write is a raw pointer dereference — no GC, no interface boxing, no hidden allocations.

Key engine properties:

| Property | Detail |
|----------|--------|
| **Zero allocations** | Verified `0 allocs/op` on every hot path by CI |
| **33 neural-network operators** | Full transformer support, including GELU, LayerNorm, BatchedMatMul, etc. |
| **Hardware-accelerated MatMul** | SIMD 4×4 micro-kernel on native; WebGPU compute shader on WASM |
| **Custom `.mpmodel` format** | INT8/FP16 quantization, ONNX-sourced — no runtime conversion overhead |
| **Zero external dependencies** | Pure Go — `go get` and you're done |
| **Deterministic execution** | Same inputs → same outputs, always |

This is the reason VipeDB can ship as a single binary and still outperform Python-based tools with significantly lower memory usage.

### Powered by [MemRAG](https://github.com/GoMemPipe/memrag) — High-Performance Embedding Inference

The embedding layer is handled by **MemRAG**: a Go library purpose-built for zero-allocation embedding inference in retrieval-augmented generation (RAG) applications. It runs directly on MemPipe and exposes a clean, high-performance API for generating text embeddings.

What makes it fast:

- **Zero-Allocation Hot Path**: Pre-allocated buffers for tokenizer and pooling operations — GC pressure is eliminated by design
- **Dynamic Sequence Length**: The engine reshapes to the actual token count, so short inputs are processed faster — no wasted compute on padding
- **Multiple Pooling Strategies**: Mean pooling, CLS pooling, and raw output — pick what your model needs
- **Concurrent Inference**: Thread-safe engine pool with bounded concurrency via semaphores, ready for high-throughput workloads
- **Extensible Operator Registry**: Pluggable operator system for custom inference operations
- **Multiple Tokenizer Support**: WordPiece (BERT), BPE, and SentencePiece tokenizers built in

### The net result

> A zero-dependency, ultra-low memory footprint, single-binary semantic search engine — no Python environment to manage, no CUDA drivers to install, no 10 GB PyTorch download.

You get production-grade embedding inference that starts in milliseconds, consumes a fraction of the RAM of traditional tools, and fits in your CI pipeline without a second thought.

## Installation

### Option A: Binary (Recommended)

```bash
# Download and make executable (Linux/macOS)
chmod +x vipe

# Initialize — creates ~/.vipe/ and downloads the default model automatically
./vipe init
```

That's it. `vipe init` handles everything:
- Creates `~/.vipe/` directory (config, models, index, cache)
- Downloads the default `bge-small-en-v1.5` model from Hugging Face
- Writes `~/.vipe/config.yaml`

You can run `vipe` from **any directory** — all data lives in `~/.vipe/` (global) unless a local `.vipe/` workspace exists in the current directory (see [Local Workspaces](#local-workspaces) below).

### Option B: Docker

```bash
# Build the image
docker build -t vipedb .

# Initialize (downloads models into the volume)
docker run --rm -v vipe-data:/data/.vipe vipedb init

# Index files
docker run --rm -v vipe-data:/data/.vipe -v $(pwd):/workspace vipedb index /workspace/src/

# Search
docker run --rm -v vipe-data:/data/.vipe vipedb search "connection timeout"

# Stream logs from another container
docker logs -f my-app 2>&1 | docker run --rm -i -v vipe-data:/data/.vipe vipedb stream
```

### Option C: Build from Source

```bash
git clone https://github.com/hashemzargari/vipedb
cd vipedb
go build -o vipe ./cmd/vipe
./vipe init
```

### Additional Models

All pre-converted `.mpmodel` files are hosted on Hugging Face:

**[https://huggingface.co/hashemzargari/mpmodels](https://huggingface.co/hashemzargari/mpmodels)**

Each model is available in three quantization variants:

| Variant | Suffix | Description |
|---------|--------|-------------|
| FP32 (default) | _(none)_ | Full precision, highest accuracy |
| FP16 | `-fp16` | Half precision, ~2× smaller, negligible accuracy loss |
| INT8 | `-int8` | 8-bit quantized, smallest size, fastest on CPU |

To add a model manually:

```bash
# Download into ~/.vipe/models/<model-name>/
# Each model needs: model.mpmodel + vocab.txt
ls ~/.vipe/models/bge-small-en-v1.5/
# model.mpmodel  vocab.txt
```

## Usage

### Quick Start

```bash
# 1. Initialize (one-time — downloads model, creates ~/.vipe/)
vipe init
# [vipe] Using global workspace: /home/you/.vipe

# 2. Index some files (from any directory)
vipe index ./src/
# [vipe] Using global workspace: /home/you/.vipe
# Indexed 120 documents (total: 120)

# 3. Search
vipe search "connection timeout"
# [vipe] Using global workspace: /home/you/.vipe
# 1. [Score: 0.9142] src/server.go ...
```

Every command prints which workspace is active so you always know where your data lives.

### Indexing

```bash
# Index a single file
vipe index file.txt

# Index a directory recursively
vipe index ./src/

# Index direct text
vipe index "some text to remember"

# Force reindex even if cached
vipe index -force file.txt
vipe index -force ./src/
```

### Searching

```bash
# Semantic search across all indexed documents
vipe search "your query"

# JSON output for scripts and agents
vipe search --json "database error"

# Pipe JSON into jq
vipe search --json "timeout" | jq '.[0]'
```

**Human output** (default) — colored, scored, and easy to scan:

```
  1. [Score: 0.9142] src/server.go
     connection timed out after 30s, retrying...

  2. [Score: 0.7831] src/client.go
     dial tcp: connection refused
```

**Agent output** (`--json`) — strict JSON array, no colors, no noise:

```json
[
  {
    "rank": 1,
    "text": "connection timed out after 30s, retrying...",
    "score": 0.9142,
    "source": "src/server.go",
    "document_id": "src/server.go:connection timed out after 30s, retrying..."
  }
]
```

### Semantic Grep

```bash
# Search in specific files
vipe grep "user login" file.txt

# Recursive search in a directory
vipe grep -r "error handling" ./src/

# Limit results
vipe grep -k=5 "authentication" ./src/

# JSON output
vipe grep --json -r "null pointer" ./src/

# Force reindex target files before searching
vipe grep -force "new feature" main.go
```

### Real-time Log Streaming

The `stream` command loads the embedding model **once** into memory and then continuously ingests text — ideal for real-time log analysis, monitoring pipelines, and autonomous agents.

#### Pipe from stdin

```bash
# Stream system logs
tail -f /var/log/syslog | vipe stream

# Stream Docker container logs
docker logs -f my-app 2>&1 | vipe stream

# Stream journald
journalctl -f | vipe stream

# Stream Kubernetes pod logs
kubectl logs -f deployment/my-app | vipe stream
```

#### Tail a file directly

```bash
# Monitor an application log file (no external piping needed)
vipe stream --tail /var/log/app.log

# Monitor with custom batch settings
vipe stream --tail /var/log/app.log --batch-size 100 --flush-interval 5s
```

#### Tuning the Stream Pipeline

| Flag | Default | Description |
|------|---------|-------------|
| `--tail <path>` | _(stdin)_ | Tail a file instead of reading stdin |
| `--batch-size <n>` | `50` | Number of lines collected before triggering an embedding batch |
| `--flush-interval <dur>` | `2s` | Max time to wait before flushing a partial batch |
| `--workers <n>` | `4` | Number of concurrent embedding workers in the EnginePool |

```bash
# High-throughput: large batches, more workers
tail -f /var/log/nginx/access.log | vipe stream --batch-size 200 --workers 8

# Low-latency: small batches, fast flush
vipe stream --tail /var/log/app.log --batch-size 10 --flush-interval 500ms
```

**How it works:**

1. Lines flow in from `stdin` or the tailed file
2. A batch collector buffers lines until `--batch-size` is reached **or** `--flush-interval` fires — whichever comes first
3. The batch is dispatched to the worker pool, where `--workers` concurrent embedding pipelines process lines in parallel
4. Embeddings are atomically flushed to the persistent `~/.vipe/index/` storage (temp file → fsync → rename)
5. On `SIGTERM`/`SIGINT`, the remaining buffer is drained and flushed before exit

While `stream` is running, you can open another terminal and search the ingested logs in real-time:

```bash
# Terminal 1: stream logs
tail -f /var/log/syslog | vipe stream

# Terminal 2: search what's been ingested
vipe search "out of memory"
vipe search --json "segfault" | jq '.[].score'
```

### Cache Management

```bash
vipe cache list              # List cached files
vipe cache clear             # Clear all cache
vipe cache clear "*.log"     # Clear files matching pattern
vipe cache clean             # Clean expired entries
vipe cache clean 24h         # Clean entries older than 24h
```

### Global Options

| Flag | Description |
|------|-------------|
| `-config` | Path to config file (default: `<workspace>/config.yaml`) |
| `-force` | Force reindex even if file is cached |
| `-verbose` | Enable verbose output (shows stats, batch flushes, etc.) |
| `-version` | Show version |

### Local Workspaces

By default, VipeDB stores everything in the **global** `~/.vipe/` directory. But you can create a **local workspace** for any project:

```bash
# Create a local workspace in the current project
vipe init --local
# [vipe] Using local workspace: /path/to/project/.vipe
# Initializing local VipeDB workspace...
```

Once a `.vipe/` directory exists in the current working directory, **all commands automatically use it**:

```bash
cd my-project/
vipe index ./src/
# [vipe] Using local workspace: /home/you/my-project/.vipe

vipe search "error handling"
# [vipe] Using local workspace: /home/you/my-project/.vipe
```

If no `.vipe/` exists in the CWD, VipeDB falls back to the global `~/.vipe/`.

**Resolution order:**

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | `VIPE_HOME` env var | Explicit override — always wins |
| 2 | `./.vipe/` | Local workspace in the current directory |
| 3 | `~/.vipe/` | Global fallback |

Add `.vipe/` to your `.gitignore` to keep project workspaces out of version control:

```bash
echo '.vipe/' >> .gitignore
```

### Where Data Lives

VipeDB data is centralized in one directory — either local or global:

```
.vipe/                       # or ~/.vipe/ for global
├── config.yaml              # Configuration
├── models/                  # Downloaded embedding models
│   └── bge-small-en-v1.5/
│       ├── model.mpmodel
│       └── vocab.txt
├── index/                   # Vector index (persistent embeddings)
│   └── index.bin
└── cache/                   # File cache (SHA256 hashes)
    └── cache.bin
```

The structure is identical for both local and global workspaces.

### Configuration

Edit `~/.vipe/config.yaml` to customize:

```yaml
models:
  directory: ~/.vipe/models        # Models directory
  default: BAAI/bge-small-en-v1.5  # Default model
  models:
    bge-small: bge-small-en-v1.5
    e5-small: multilingual-e5-small-fp16
    minilm: paraphrase-multilingual-MiniLM-12-v2

index:
  directory: ~/.vipe/index         # Index storage directory

search:
  default_top_k: 10                # Default number of results
  threshold: 0.0                   # Minimum similarity threshold

cache:
  enabled: true                    # Enable caching
  directory: ~/.vipe/cache         # Cache storage directory
  retention: 720h                  # Cache retention (30 days)
  auto_clean: true                 # Auto-clean expired entries

general:
  verbose: false                   # Enable verbose output
```

### Cache Configuration

The cache system automatically tracks indexed files using SHA256 hashes:

- **enabled**: Enable or disable caching (default: true)
- **directory**: Where cache metadata is stored
- **retention**: How long to keep cache entries (format: `24h`, `720h`, etc.)
- **auto_clean**: Automatically remove expired entries on startup

```yaml
cache:
  enabled: true
  retention: 168h    # 7 days
  auto_clean: true
```

### Recipes

#### DevOps: Monitor Production Logs in Real-time

```bash
# Terminal 1: continuously ingest logs
tail -f /var/log/nginx/access.log | vipe stream --batch-size 100 --workers 8

# Terminal 2: query when an alert fires
vipe search --json "502 bad gateway" | jq '.[] | select(.score > 0.7)'
```

#### CI/CD: Search Build Logs for Failures

```bash
# Pipe build output into VipeDB
make build 2>&1 | vipe stream

# Then query the results
vipe search --json "undefined reference" | jq '.[0].text'
```

#### Agent Integration: LLM-powered Log Analysis

```bash
# Stream logs, then let an LLM agent search semantically
vipe stream --tail /var/log/app.log &

# Agent queries via JSON, pipes results to another LLM
RESULTS=$(vipe search --json "authentication failure")
echo "$RESULTS" | llm "Summarize these auth failures and suggest fixes"
```

#### Multilingual Search

```bash
# Configure multilingual-e5-small in ~/.vipe/config.yaml, then:
vipe search "recherche sémantique"
vipe search "意味検索"
```

#### Index a Codebase

```bash
# Index all source files (cached automatically)
vipe index ./src/

# Force reindex after code changes
vipe index -force ./src/

# Search for error handling patterns
vipe search "handle errors gracefully"
```

## Architecture

```
vipe (CLI)
├── Workspace Resolution
│   ├── VIPE_HOME env var (explicit override)
│   ├── ./.vipe/ (local project workspace)
│   └── ~/.vipe/ (global fallback)
├── Workspace (.vipe/)
│   ├── config.yaml
│   ├── models/
│   ├── index/
│   └── cache/
├── Embedding Service (MemRAG)
│   ├── Single-use Service (index, search, grep)
│   └── EnginePool (stream) — N concurrent pipelines, channel-based
│       ├── BGE-small-en-v1.5
│       └── multilingual-e5-small-fp16
├── Stream Pipeline
│   ├── stdin reader / File Tailer (poll-based, rotation-safe)
│   ├── Batch Collector (size-triggered + time-triggered flush)
│   └── Worker Pool (concurrent embed → atomic store flush)
├── Vector Store
│   ├── In-memory index (RWMutex, deduplicated)
│   ├── Persistent storage (<workspace>/index/index.bin)
│   └── Atomic save (temp file → fsync → rename)
├── Output Formatter
│   ├── Colored terminal (fatih/color)
│   └── Strict JSON (--json)
├── Cache Index
│   ├── SHA256 file hashing
│   ├── ModTime/Size validation
│   └── Retention policy
└── Search Engine
    ├── Cosine similarity
    └── Source file filtering
```

## How Caching Works

1. **First Index**: File is read, hashed with SHA256, embeddings generated, stored in index
2. **Subsequent Index**: File's hash is compared with cache; if unchanged, skip indexing
3. **Force Index**: Use `-force` flag to bypass cache and reindex
4. **Expiration**: Entries older than `retention` period are auto-cleaned (if `auto_clean: true`)

```bash
# First run - indexes everything
vipe index ./src/
# Output: Indexed 500 documents (total: 500)

# Second run - uses cache
vipe index ./src/
# Output: Indexed 0 documents (skipped 500 cached) (total: 500)

# Force reindex after changes
vipe index -force ./src/
# Output: Indexed 500 documents (total: 500)
```

## Thread Safety & Concurrent Access

VipeDB is designed for concurrent use:

- **Vector Store** uses `sync.RWMutex` — multiple `search` processes can read simultaneously while `stream` writes
- **Atomic Save** ensures the index file is never corrupted, even if a `search` reads during a `stream` flush (temp file → fsync → rename)
- **EnginePool** distributes embedding pipelines across workers via a buffered channel — zero mutex contention on the hot path
- **Graceful Shutdown** on `SIGTERM`/`SIGINT` drains the line buffer and flushes the final batch before exiting

## Model Support

All models are available at [huggingface.co/hashemzargari/mpmodels](https://huggingface.co/hashemzargari/mpmodels) in FP32, FP16, and INT8 variants.

| Model | Dimensions | Max Length | Language | Variants |
|-------|-----------|------------|----------|----------|
| BAAI/bge-small-en-v1.5 | 384 | 512 | English | fp32, fp16, int8 |
| BAAI/bge-large-en-v1.5 | 1024 | 512 | English | fp32, fp16, int8 |
| intfloat/multilingual-e5-small | 384 | 512 | Multilingual | fp32, fp16, int8 |
| intfloat/multilingual-e5-large | 1024 | 512 | Multilingual | fp32, fp16, int8 |
| nomic-ai/nomic-embed-text-v1.5 | 768 | 8192 | English | fp32, fp16, int8 |
| sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2 | 384 | 128 | Multilingual | fp32, fp16, int8 |

## Building from Source

```bash
# Clone repository
git clone https://github.com/hashemzargari/vipedb
cd vipedb

# Build binary
go build -o vipe ./cmd/vipe

# Initialize (downloads model + creates ~/.vipe/)
./vipe init
# [vipe] Using global workspace: /home/you/.vipe

# Run tests
go test ./...
```

### Building for Different Platforms

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o vipe-linux-amd64 ./cmd/vipe

# macOS
GOOS=darwin GOARCH=arm64 go build -o vipe-darwin-arm64 ./cmd/vipe

# Windows
GOOS=windows GOARCH=amd64 go build -o vipe-windows-amd64.exe ./cmd/vipe

# WebAssembly
GOOS=js GOARCH=wasm go build -o vipe.wasm ./cmd/vipe
```

### Docker

```bash
# Build the image
docker build -t vipedb .

# Run init (downloads model into the volume)
docker run --rm -v vipe-data:/data/.vipe vipedb init
# [vipe] Using global workspace: /data/.vipe
```

## Docker: Sidecar Log Analyzer

Run VipeDB alongside your application containers to get real-time semantic search over your logs.

### Standalone Sidecar

```bash
# 1. Build the VipeDB image
docker build -t vipedb .

# 2. Initialize (one-time — downloads the model)
docker run --rm -v vipe-data:/data/.vipe vipedb init

# 3. Stream logs from any container
docker logs -f my-app 2>&1 | docker run --rm -i -v vipe-data:/data/.vipe vipedb stream

# 4. Search (in another terminal)
docker run --rm -v vipe-data:/data/.vipe vipedb search "connection refused"
docker run --rm -v vipe-data:/data/.vipe vipedb search --json "timeout" | jq .
```

### Docker Compose

Add VipeDB as a service in your `docker-compose.yml`:

```yaml
services:
  my-app:
    image: your-app:latest

  vipedb:
    build: .
    image: vipedb:latest
    volumes:
      - vipe-data:/data/.vipe
    entrypoint: ["tini", "--"]
    command: ["sleep", "infinity"]
    restart: unless-stopped

volumes:
  vipe-data:
```

Then use it:

```bash
# Initialize VipeDB
docker compose exec vipedb vipe init

# Stream logs from your app into VipeDB
docker compose logs -f my-app | docker compose exec -T vipedb vipe stream

# Search your app's logs semantically
docker compose exec vipedb vipe search "database connection pool exhausted"
docker compose exec vipedb vipe search --json "OOM" | jq '.[].score'
```

### Kubernetes Sidecar

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
spec:
  volumes:
    - name: vipe-data
      emptyDir: {}
    - name: log-volume
      emptyDir: {}
  containers:
    - name: app
      image: your-app:latest
      volumeMounts:
        - name: log-volume
          mountPath: /var/log/app
    - name: vipedb
      image: vipedb:latest
      command: ["vipe", "stream", "--tail", "/var/log/app/app.log"]
      volumeMounts:
        - name: vipe-data
          mountPath: /data/.vipe
        - name: log-volume
          mountPath: /var/log/app
          readOnly: true
```

## Related Projects

- **[MemPipe](https://github.com/GoMemPipe/mempipe)** — Zero-GC, arena-backed pipeline and ONNX inference engine for Go. Pure Go, zero external dependencies, `0 allocs/op` verified by CI. Powers VipeDB's model execution layer with 33 neural-network operators and hardware-accelerated MatMul (SIMD / WebGPU).

- **[MemRAG](https://github.com/GoMemPipe/memrag)** — High-performance, zero-allocation embedding inference library for Go, purpose-built for RAG applications. Leverages MemPipe to run ONNX-based embedding models with minimal memory overhead. Provides the tokenizers, pooling strategies, and concurrent engine pool that VipeDB uses to generate embeddings at speed.

## Support the Project

VipeDB, MemPipe, and MemRAG are free, open-source projects. If they saved you CPU credits, infrastructure costs, or just a lot of headache, consider supporting the work that makes it possible.

[![Buy Me a Coffee](https://img.shields.io/badge/Buy%20Me%20a%20Coffee-gomempipe-orange?style=flat&logo=buy-me-a-coffee)](https://buymeacoffee.com/gomempipe)

We also accept cryptocurrency donations:

- **Bitcoin**: `bc1qy5yg97y6utrxm84erfhvyjg8e0saqg83ae6286`

Your support helps keep the project maintained, documented, and growing. Every contribution — big or small — is genuinely appreciated.

## License

MIT License
