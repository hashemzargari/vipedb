# VipeDB

**VipeDB** is an all-in-one semantic search tool that combines a vector database and embedding models in a single executable binary. It's designed to be a drop-in replacement for `grep` with semantic search capabilities.

## Features

- **Single Binary**: Download and run - no complex setup
- **Built-in Embedding Models**: Includes multilingual-e5-small-fp16 and bge-small-en-v1.5
- **YAML Configuration**: Easy-to-use config file for customizing models and settings
- **Grep-like Interface**: Familiar command-line interface for semantic search
- **Vector Database**: Persistent storage for embeddings with automatic deduplication
- **Intelligent Caching**: SHA256-based file hashing, auto-skip cached files
- **Cache Management**: Manual and automatic cache cleanup with configurable retention
- **Cross-platform**: Works on Linux, macOS, and Windows (WebAssembly support planned)

## Installation

### 1. Download the Binary

```bash
# Download and make executable (Linux/macOS)
chmod +x vipe
```

### 2. Download Models

Download pre-converted `.mpmodel` files from our [release page](https://github.com/hashemzargari/vipedb/vipedb/releases):

```bash
# Create models directory
mkdir -p models

# Download models from GitHub releases
# Visit: https://github.com/hashemzargari/vipedb/vipedb/releases

# Or download specific models:
# bge-small-english: https://github.com/hashemzargari/vipedb/releases/latest/download/bge-small-en-v1.5.zip
# multilingual-e5-small: https://github.com/hashemzargari/vipedb/releases/latest/download/multilingual-e5-small-fp16.zip

# Extract and place in models/ directory
# Each model directory should contain: model.mpmodel and vocab.txt
#
# Expected structure:
# models/
# ├── bge-small-en-v1.5/
# │   ├── model.mpmodel
# │   └── vocab.txt
# └── multilingual-e5-small-fp16/
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

VipeDB includes support for:

| Model | Dimensions | Max Length | Language |
|-------|-----------|------------|----------|
| BAAI/bge-small-en-v1.5 | 384 | 512 | English |
| intfloat/multilingual-e5-small-fp16 | 384 | 512 | Multilingual |
| sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2 | 384 | 128 | Multilingual |

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

## License

MIT License

## Related Projects

- [MemRAG](https://github.com/GoMemPipe/memrag) - High-performance embedding inference
- [MemPipe](https://github.com/GoMemPipe/mempipe) - ONNX inference engine
