# VipeDB

**VipeDB** is an all-in-one semantic search tool that combines a vector database and embedding models in a single executable binary. It's designed to be a drop-in replacement for `grep` with semantic search capabilities.

## Features

- **Single Binary**: Download and run - no complex setup
- **Built-in Embedding Models**: Includes multilingual-e5-small-fp16 and bge-small-en-v1.5
- **YAML Configuration**: Easy-to-use config file for customizing models and settings
- **Grep-like Interface**: Familiar command-line interface for semantic search
- **Vector Database**: Persistent storage for embeddings
- **Cross-platform**: Works on Linux, macOS, and Windows (WebAssembly support planned)

## Installation

### 1. Download the Binary

```bash
# Download and make executable (Linux/macOS)
chmod +x vipe
```

### 2. Download Models (Optional)

Models are included by default. To add more models:

```bash
# Create models directory
mkdir -p models

# Download and place model files in models/ directory
# Each model should have: model.mpmodel and vocab.txt
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

# Index files or text
./vipe index file.txt
./vipe index ./src/           # Index directory recursively
./vipe index "some text"      # Index direct text

# Search indexed documents
./vipe search "your query"

# Semantic grep (search in files)
./vipe grep "pattern" file.txt
./vipe grep -r "pattern" ./src/    # Recursive search
./vipe grep -k=5 "pattern" file.txt # Top 5 results
```

### Configuration

Edit `.vipedb.yaml` to customize:

```yaml
models:
  directory: ./models          # Models directory
  default: BAAI/bge-small-en-v1.5  # Default model
  models:
    bge-small: bge-small-en-v1.5
    e5-small: multilingual-e5-small-fp16
    minilm: paraphrase-multilingual-MiniLM-L12-v2

index:
  directory: ./.vipedb         # Index storage directory

search:
  default_top_k: 10            # Default number of results
  threshold: 0.0               # Minimum similarity threshold

general:
  verbose: false               # Enable verbose output
```

### Examples

#### Index a Codebase

```bash
# Index all source files
./vipe index ./src/

# Search for error handling patterns
./vipe search "handle errors gracefully"
```

#### Semantic Grep

```bash
# Find code related to authentication
./vipe grep -r "user login" ./src/

# Search documentation
./vipe grep -k=10 "API documentation" ./docs/
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
│   ├── In-memory index
│   └── Persistent storage (.vipedb/)
└── Search Engine
    ├── Cosine similarity
    └── Top-K retrieval
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

## WebAssembly Support

WebAssembly build for browser extensions:

```bash
# Build for WebAssembly
GOOS=js GOARCH=wasm go build -o vipe.wasm ./cmd/vipe-wasm
```

## License

MIT License

## Related Projects

- [MemRAG](https://github.com/GoMemPipe/memrag) - High-performance embedding inference
- [MemPipe](https://github.com/GoMemPipe/mempipe) - ONNX inference engine
