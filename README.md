# VipeDB

**VipeDB** is an all-in-one semantic search tool that combines a vector database and embedding models in a single executable binary. It's designed to be a drop-in replacement for `grep` with semantic search capabilities — and it's built on a stack that most Python-based tools can't touch.

![VipeDB demo](demo.gif)

## Features

- **Single Binary**: Download and run — no complex setup, no runtime dependencies
- **Built-in Embedding Models**: Includes multilingual-e5-small-fp16 and bge-small-en-v1.5
- **YAML Configuration**: Easy-to-use config file for customizing models and settings
- **Grep-like Interface**: Familiar command-line interface for semantic search
- **Vector Database**: Persistent storage for embeddings with automatic deduplication
- **Intelligent Caching**: SHA256-based file hashing, auto-skip cached files
- **Cache Management**: Manual and automatic cache cleanup with configurable retention
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

### 1. Download the Binary

```bash
# Download and make executable (Linux/macOS)
chmod +x vipe
```

### 2. Download Models

All pre-converted `.mpmodel` files are hosted on Hugging Face:

**[https://huggingface.co/hashemzargari/mpmodels](https://huggingface.co/hashemzargari/mpmodels)**

Each model is available in three quantization variants — pick the one that fits your hardware:

| Variant | Suffix | Description |
|---------|--------|-------------|
| FP32 (default) | _(none)_ | Full precision, highest accuracy |
| FP16 | `-fp16` | Half precision, ~2× smaller, negligible accuracy loss |
| INT8 | `-int8` | 8-bit quantized, smallest size, fastest on CPU |

```bash
# Create models directory
mkdir -p models

# Download a model folder from Hugging Face (example: bge-small-en-v1.5 FP16)
# Visit https://huggingface.co/hashemzargari/mpmodels and download the folder
# for your chosen model, then place it under models/
#
# Expected structure:
# models/
# └── bge-small-en-v1.5-fp16/
#     ├── model.mpmodel
#     └── vocab.txt
```

### 3. Initialize Configuration

```bash
./vipe init
```

This creates a `.vipedb.yaml` configuration file.

## Usage

### Basic Commands

```bash
# Initialize configuration
./vipe init

# Index files or text (with automatic caching)
./vipe index file.txt
./vipe index ./src/           # Index directory recursively
./vipe index "some text"      # Index direct text

# Force reindex even if cached
./vipe index -force file.txt

# Search indexed documents
./vipe search "your query"

# Semantic grep (search in files)
./vipe grep "pattern" file.txt
./vipe grep -r "pattern" ./src/    # Recursive search
./vipe grep -k=5 "pattern" file.txt # Top 5 results
./vipe grep -force "pattern" file.txt # Force reindex target files

# Cache management
./vipe cache list              # List cached files
./vipe cache clear             # Clear all cache
./vipe cache clear "*.log"     # Clear files matching pattern
./vipe cache clean             # Clean expired entries
./vipe cache clean 24h         # Clean entries older than 24h
```

### Command-Line Options

| Flag | Description |
|------|-------------|
| `-config` | Path to config file (default: `.vipedb.yaml`) |
| `-force` | Force reindex even if file is cached |
| `-verbose` | Enable verbose output |
| `-version` | Show version |

### Configuration

Edit `.vipedb.yaml` to customize:

```yaml
models:
  directory: ./models          # Models directory
  default: BAAI/bge-small-en-v1.5  # Default model
  models:
    bge-small: bge-small-en-v1.5
    e5-small: multilingual-e5-small-fp16
    minilm: paraphrase-multilingual-MiniLM-12-v2

index:
  directory: ./.vipedb         # Index storage directory

search:
  default_top_k: 10            # Default number of results
  threshold: 0.0               # Minimum similarity threshold

cache:
  enabled: true                # Enable caching
  directory: ./.vipedb/cache   # Cache storage directory
  retention: 720h              # Cache retention (30 days)
  auto_clean: true             # Auto-clean expired entries

general:
  verbose: false               # Enable verbose output
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

### Examples

#### Index a Codebase

```bash
# Index all source files (cached automatically)
./vipe index ./src/

# Force reindex after code changes
./vipe index -force ./src/

# Search for error handling patterns
./vipe search "handle errors gracefully"
```

#### Semantic Grep

```bash
# Find code related to authentication (only in specified files)
./vipe grep -r "user login" ./src/

# Search documentation with custom result count
./vipe grep -k=10 "API documentation" ./docs/

# Force grep to reindex target files
./vipe grep -force "new feature" main.go
```

#### Cache Management

```bash
# Check what's cached
./vipe cache list
# Output:
# Cached files (2):
#   src/main.go (150 docs, 2026-04-06 10:30)
#   src/utils.go (80 docs, 2026-04-06 10:30)
# Stats: Files: 2, Docs: 230, Retention: 720h0m0s, AutoClean: true

# Clean old entries (older than retention period)
./vipe cache clean

# Clean specific pattern
./vipe cache clear "*.backup"

# Clear all cache to force complete reindex
./vipe cache clear
```

#### Multiple Models

```bash
# Use multilingual model
./vipe --config .vipedb.yaml search "recherche sémantique"
```

## Architecture

```
vipe (CLI)
├── Embedding Service (MemRAG)
│   ├── BGE-small-en-v1.5
│   └── multilingual-e5-small-fp16
├── Vector Store
│   ├── In-memory index (deduplicated)
│   └── Persistent storage (.vipedb/)
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
./vipe index ./src/
# Output: Indexed 500 documents (total: 500)

# Second run - uses cache
./vipe index ./src/
# Output: Indexed 0 documents (skipped 500 cached) (total: 500)

# Force reindex after changes
./vipe index -force ./src/
# Output: Indexed 500 documents (total: 500)
```

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
git clone https://github.com/your-org/vipedb
cd vipedb

# Build binary
go build -o vipe ./cmd/vipe

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
