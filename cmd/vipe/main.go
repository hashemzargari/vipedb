package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hashemzargari/vipedb/internal/config"
	"github.com/hashemzargari/vipedb/internal/embedding"
	"github.com/hashemzargari/vipedb/internal/output"
	"github.com/hashemzargari/vipedb/internal/stream"
	"github.com/hashemzargari/vipedb/internal/version"
	"github.com/hashemzargari/vipedb/pkg/vector"
)

var cfg *config.Config

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", "", "path to config file (default: ~/.vipe/config.yaml)")
	showVersion := flag.Bool("version", false, "show version")
	force := flag.Bool("force", false, "force reindex even if cached")
	verbose := flag.Bool("verbose", false, "enable verbose output")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vipe version %s\n", version.Version)
		return 0
	}

	var err error
	cfg, err = config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}

	if *verbose {
		cfg.General.Verbose = true
	}

	// Print workspace log line so the user always knows which data dir is active.
	ws := config.ResolveWorkspace()
	if ws.IsLocal {
		fmt.Fprintf(os.Stderr, "[vipe] Using local workspace: %s\n", ws.Path)
	} else {
		fmt.Fprintf(os.Stderr, "[vipe] Using global workspace: %s\n", ws.Path)
	}

	if len(flag.Args()) == 0 {
		printUsage()
		return 0
	}

	cmd := flag.Args()[0]
	args := flag.Args()[1:]

	switch cmd {
	case "init":
		return cmdInit(cfg, *configPath)
	case "index":
		return cmdIndex(cfg, args, *force)
	case "search":
		return cmdSearch(cfg, args)
	case "grep":
		return cmdGrep(cfg, args, *force)
	case "stream":
		return cmdStream(cfg, args)
	case "cache":
		return cmdCache(cfg, args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		return 1
	}
}

func printUsage() {
	home := config.VipeHome()
	ws := config.ResolveWorkspace()
	wsKind := "global"
	if ws.IsLocal {
		wsKind = "local"
	}
	fmt.Printf(`vipe %s - AI-powered semantic search & real-time log analyzer

Active workspace (%s): %s

Usage:
  vipe <command> [options]

Commands:
  init              Initialize VipeDB (global by default, --local for project)
  index             Index files or text
  search            Search indexed documents
  grep              Semantic grep (search for pattern in files)
  stream            Real-time persistent log ingestion
  cache             Manage cache (list, clear, clean)

Global Options:
  -force            Force reindex even if file is cached
  -config           Path to config file (default: %s/config.yaml)
  -verbose          Enable verbose output
  -version          Show version

Init Options:
  --local           Create a .vipe workspace in the current directory

Search/Grep Options:
  --json            Output results as strict JSON (for agents & pipelines)

Stream Options:
  --tail <path>           Tail a file instead of reading stdin
  --batch-size <n>        Lines per batch (default: 50)
  --flush-interval <dur>  Batch flush interval (default: 2s)
  --workers <n>           Concurrent embedding workers (default: 4)

Examples:
  vipe init
  vipe init --local
  vipe index file.txt
  vipe search "connection timeout"
  vipe search --json "database error" | jq .
  vipe grep -r "error handling" ./src/
  tail -f /var/log/syslog | vipe stream
  vipe stream --tail /var/log/app.log
  vipe cache list
`, version.Version, wsKind, ws.Path, home)
}

func cmdInit(cfg *config.Config, configPath string) int {
	// Check for --local flag in remaining args.
	local := false
	for _, a := range flag.Args()[1:] {
		if a == "--local" {
			local = true
		}
	}

	var targetRoot string
	if local {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
			return 1
		}
		targetRoot = filepath.Join(cwd, ".vipe")
		fmt.Println("Initializing local VipeDB workspace...")
	} else {
		targetRoot = config.GlobalHome()
		fmt.Println("Initializing VipeDB (global)...")
	}

	// Create directory structure.
	if err := config.EnsureHome(targetRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory structure: %v\n", err)
		return 1
	}
	fmt.Printf("  Home directory: %s\n", targetRoot)

	// Build config for the target root and save it.
	initCfg := config.DefaultFor(targetRoot)
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(targetRoot, "config.yaml")
	}
	if err := initCfg.Save(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		return 1
	}
	fmt.Printf("  Config: %s\n", cfgPath)
	fmt.Printf("  Models: %s\n", initCfg.Models.Directory)
	fmt.Printf("  Index:  %s\n", initCfg.Index.Directory)
	fmt.Printf("  Cache:  %s\n", initCfg.Cache.Directory)

	// Check if default model is already present.
	defaultDir := "bge-small-en-v1.5"
	if config.IsModelInstalled(initCfg.Models.Directory, defaultDir) {
		fmt.Printf("\n  Default model already installed: %s\n", defaultDir)
		fmt.Println("\nVipeDB is ready. Run 'vipe search \"your query\"' to get started.")
		return 0
	}

	// Download default model.
	fmt.Println("\nDownloading default model (BAAI/bge-small-en-v1.5)...")
	models := config.AvailableModels()
	spec := models[0] // bge-small-en-v1.5

	var lastFile string
	err := config.DownloadModel(initCfg.Models.Directory, spec, func(n int64, file string) {
		if file != lastFile {
			if lastFile != "" {
				fmt.Println(" done")
			}
			fmt.Printf("  Downloading %s...", file)
			lastFile = file
		}
	})
	if lastFile != "" {
		fmt.Println(" done")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError downloading model: %v\n", err)
		fmt.Fprintln(os.Stderr, "You can manually download models from: https://huggingface.co/hashemzargari/mpmodels")
		fmt.Fprintf(os.Stderr, "Place model files in: %s/<model-name>/\n", initCfg.Models.Directory)
		return 1
	}

	fmt.Println("\nVipeDB is ready. Run 'vipe search \"your query\"' to get started.")
	return 0
}

func createCacheIndex(cfg *config.Config) (*vector.CacheIndex, error) {
	if !cfg.Cache.Enabled {
		return nil, nil
	}

	cacheDir := cfg.Cache.Directory
	if cacheDir == "" {
		cacheDir = filepath.Join(cfg.Index.Directory, "cache")
	}

	return vector.NewCacheIndex(cacheDir, cfg.Cache.RetentionDur, cfg.Cache.AutoClean)
}

func cmdIndex(cfg *config.Config, args []string, force bool) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: vipe index [-force] <file|directory|text>")
		return 1
	}

	modelName := cfg.Models.DefaultModel
	modelPath := cfg.ModelPath(modelName)
	if modelPath == "" {
		fmt.Fprintf(os.Stderr, "Model not found for %s\n", modelName)
		return 1
	}

	descriptorName := cfg.ModelDescriptor(modelName)
	embService, err := embedding.NewService(embedding.ModelConfig{
		DescriptorName: descriptorName,
		ModelDir:       modelPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
		return 1
	}
	defer embService.Close()

	store := vector.NewStore(cfg.Index.Directory)
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		return 1
	}

	cache, err := createCacheIndex(cfg)
	if err != nil && cfg.General.Verbose {
		fmt.Fprintf(os.Stderr, "Warning: cache init error: %v\n", err)
	}

	ctx := context.Background()
	indexed := 0
	skipped := 0

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			if os.IsNotExist(err) {
				count, err := indexText(ctx, embService, store, arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error indexing %s: %v\n", arg, err)
					continue
				}
				indexed += count
				continue
			}
			fmt.Fprintf(os.Stderr, "Error stating %s: %v\n", arg, err)
			continue
		}

		if info.IsDir() {
			count, skip, err := indexPath(ctx, embService, store, cache, arg, force)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error indexing %s: %v\n", arg, err)
				continue
			}
			indexed += count
			skipped += skip
		} else {
			count, skip, err := indexFile(ctx, embService, store, cache, arg, force)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error indexing %s: %v\n", arg, err)
				continue
			}
			indexed += count
			skipped += skip
		}
	}

	if err := store.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving index: %v\n", err)
		return 1
	}

	fmt.Printf("Indexed %d documents", indexed)
	if skipped > 0 {
		fmt.Printf(" (skipped %d cached)", skipped)
	}
	fmt.Printf(" (total: %d)\n", store.Count())
	return 0
}

func indexPath(ctx context.Context, embService *embedding.Service, store *vector.Store, cache *vector.CacheIndex, path string, force bool) (int, int, error) {
	total := 0
	skipped := 0
	err := filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fileInfo.IsDir() && isTextFile(filePath) {
			count, skip, err := indexFile(ctx, embService, store, cache, filePath, force)
			total += count
			skipped += skip
			return err
		}
		return nil
	})
	return total, skipped, err
}

func indexFile(ctx context.Context, embService *embedding.Service, store *vector.Store, cache *vector.CacheIndex, path string, force bool) (int, int, error) {
	if cache != nil && !force {
		if entry, ok := cache.Get(path); ok {
			if cfg.General.Verbose {
				fmt.Printf("Using cached: %s (%d docs)\n", path, entry.DocCount)
			}
			return 0, 1, nil
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(string(content), "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		embedding, err := embService.Embed(ctx, line)
		if err != nil {
			return count, 0, err
		}

		doc := vector.Document{
			ID:        fmt.Sprintf("%s:%s", path, line),
			Content:   line,
			Embedding: embedding,
			Metadata: map[string]string{
				"source": path,
			},
		}

		if err := store.Add(doc); err != nil {
			if cfg.General.Verbose {
				fmt.Printf("Warning: duplicate skipped: %s\n", line)
			}
		}
		count++
	}

	if cache != nil {
		if err := cache.Set(path, count); err != nil && cfg.General.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: cache set error: %v\n", err)
		}
	}

	return count, 0, nil
}

func indexText(ctx context.Context, embService *embedding.Service, store *vector.Store, text string) (int, error) {
	embedding, err := embService.Embed(ctx, text)
	if err != nil {
		return 0, err
	}

	doc := vector.Document{
		ID:        fmt.Sprintf("text:%s", text),
		Content:   text,
		Embedding: embedding,
		Metadata:  make(map[string]string),
	}

	if err := store.Add(doc); err != nil {
		return 0, err
	}

	return 1, nil
}

func cmdSearch(cfg *config.Config, args []string) int {
	// Parse --json flag from args
	var jsonOutput bool
	args, jsonOutput = extractFlag(args, "--json")

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: vipe search [--json] <query>")
		return 1
	}

	query := strings.Join(args, " ")

	modelName := cfg.Models.DefaultModel
	modelPath := cfg.ModelPath(modelName)
	if modelPath == "" {
		fmt.Fprintf(os.Stderr, "Model not found for %s\n", modelName)
		return 1
	}

	descriptorName := cfg.ModelDescriptor(modelName)
	embService, err := embedding.NewService(embedding.ModelConfig{
		DescriptorName: descriptorName,
		ModelDir:       modelPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
		return 1
	}
	defer embService.Close()

	store := vector.NewStore(cfg.Index.Directory)
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		return 1
	}

	ctx := context.Background()
	queryEmb, err := embService.Embed(ctx, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error embedding query: %v\n", err)
		return 1
	}

	results := store.Search(queryEmb, cfg.Search.DefaultTopK)
	if len(results) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("No results found")
		}
		return 0
	}

	output.PrintSearchResults(os.Stdout, results, jsonOutput)
	return 0
}

func cmdGrep(cfg *config.Config, args []string, force bool) int {
	var jsonOutput bool
	args, jsonOutput = extractFlag(args, "--json")

	recursive := false
	topK := cfg.Search.DefaultTopK

	for i, arg := range args {
		if arg == "-r" || arg == "--recursive" {
			recursive = true
			args = append(args[:i], args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-k=") {
			fmt.Sscanf(arg, "-k=%d", &topK)
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: vipe grep [-r] [-k=N] [-force] <pattern> [files...]")
		return 1
	}

	pattern := args[0]
	files := args[1:]

	modelName := cfg.Models.DefaultModel
	modelPath := cfg.ModelPath(modelName)
	if modelPath == "" {
		fmt.Fprintf(os.Stderr, "Model not found for %s\n", modelName)
		return 1
	}

	descriptorName := cfg.ModelDescriptor(modelName)
	embService, err := embedding.NewService(embedding.ModelConfig{
		DescriptorName: descriptorName,
		ModelDir:       modelPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
		return 1
	}
	defer embService.Close()

	cache, err := createCacheIndex(cfg)
	if err != nil && cfg.General.Verbose {
		fmt.Fprintf(os.Stderr, "Warning: cache init error: %v\n", err)
	}

	store := vector.NewStore(cfg.Index.Directory)
	if err := store.Load(); err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
			return 1
		}
		if len(files) == 0 {
			fmt.Fprintln(os.Stderr, "No index found. Use 'vipe index' to index files first.")
			return 1
		}
		store = vector.NewStore(cfg.Index.Directory)
	}

	ctx := context.Background()

	if len(files) > 0 {
		for _, file := range files {
			var count int
			if recursive {
				count, _, err = indexPath(ctx, embService, store, cache, file, force)
			} else {
				count, _, err = indexFile(ctx, embService, store, cache, file, force)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: error indexing %s: %v\n", file, err)
			}
			if cfg.General.Verbose {
				fmt.Printf("Indexed %d docs from %s\n", count, file)
			}
		}
	}

	queryEmb, err := embService.Embed(ctx, pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error embedding query: %v\n", err)
		return 1
	}

	allResults := store.Search(queryEmb, topK*10)

	var results []vector.SearchResult
	fileSet := make(map[string]bool)
	for _, f := range files {
		absPath, _ := filepath.Abs(f)
		fileSet[f] = true
		fileSet[absPath] = true
	}

	for _, r := range allResults {
		if len(files) == 0 {
			results = append(results, r)
			continue
		}
		source := ""
		if s, ok := r.Metadata["source"]; ok {
			source = s
		}
		if fileSet[source] {
			results = append(results, r)
		}
		if len(results) >= topK {
			break
		}
	}

	if len(results) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		}
		return 0
	}

	output.PrintGrepResults(os.Stdout, results, jsonOutput)
	return 0
}

func cmdCache(cfg *config.Config, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: vipe cache <list|clear|clean> [pattern]")
		return 1
	}

	cache, err := createCacheIndex(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening cache: %v\n", err)
		return 1
	}

	switch args[0] {
	case "list":
		entries := cache.List()
		if len(entries) == 0 {
			fmt.Println("Cache is empty")
			return 0
		}
		fmt.Printf("Cached files (%d):\n", len(entries))
		for _, e := range entries {
			fmt.Printf("  %s (%d docs, %s)\n", e.FilePath, e.DocCount, e.IndexedAt.Format("2006-01-02 15:04"))
		}
		fmt.Printf("\nStats: %s\n", cache.Stats())

	case "clear":
		if len(args) > 1 {
			removed, err := cache.RemovePattern(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
				return 1
			}
			fmt.Printf("Cleared %d entries matching %q\n", len(removed), args[1])
		} else {
			if err := cache.RemoveAll(); err != nil {
				fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
				return 1
			}
			fmt.Println("Cache cleared")
		}

	case "clean":
		age := cfg.Cache.RetentionDur
		if age <= 0 {
			age = 24 * time.Hour
		}
		if len(args) > 1 {
			age, _ = time.ParseDuration(args[1])
		}
		cleaned := cache.ClearOlderThan(age)
		fmt.Printf("Cleaned %d expired entries\n", cleaned)

	default:
		fmt.Fprintf(os.Stderr, "Unknown cache command: %s\n", args[0])
		return 1
	}

	return 0
}

func cmdStream(cfg *config.Config, args []string) int {
	fs := flag.NewFlagSet("stream", flag.ContinueOnError)
	tailPath := fs.String("tail", "", "path to file to tail")
	batchSize := fs.Int("batch-size", 50, "lines per batch")
	flushInterval := fs.Duration("flush-interval", 2*time.Second, "batch flush interval")
	workers := fs.Int("workers", 4, "concurrent embedding workers")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	modelName := cfg.Models.DefaultModel
	modelPath := cfg.ModelPath(modelName)
	if modelPath == "" {
		fmt.Fprintf(os.Stderr, "Model not found for %s\n", modelName)
		return 1
	}

	descriptorName := cfg.ModelDescriptor(modelName)
	fmt.Fprintf(os.Stderr, "[stream] Loading EnginePool (%d workers, model: %s)...\n", *workers, descriptorName)

	pool, err := embedding.NewPool(embedding.ModelConfig{
		DescriptorName: descriptorName,
		ModelDir:       modelPath,
	}, *workers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating engine pool: %v\n", err)
		return 1
	}
	defer pool.Close()

	store := vector.NewStore(cfg.Index.Directory)
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		return 1
	}

	source := "stdin"
	if *tailPath != "" {
		source = *tailPath
	}

	sCfg := stream.Config{
		BatchSize:     *batchSize,
		FlushInterval: *flushInterval,
		Workers:       *workers,
		Source:        source,
		Verbose:       cfg.General.Verbose,
	}

	str := stream.New(pool, store, sCfg)

	// Signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\n[stream] Shutting down, flushing final buffer...\n")
		cancel()
	}()

	str.Run(ctx)

	fmt.Fprintf(os.Stderr, "[stream] Listening on %s (batch=%d, interval=%s, workers=%d)\n",
		source, *batchSize, *flushInterval, *workers)

	// Verbose stats ticker
	if cfg.General.Verbose {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					ingested, batches := str.Stats()
					fmt.Fprintf(os.Stderr, "[stream] stats: %d lines ingested, %d batches flushed\n", ingested, batches)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Input source: either tail a file or read stdin
	if *tailPath != "" {
		tailer := stream.NewTailer(*tailPath)
		if err := tailer.Run(ctx, func(line string) {
			str.AddLine(ctx, line, *tailPath)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "[stream] tail error: %v\n", err)
			cancel()
		}
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		// Pre-allocate scanner buffer to reduce GC pressure
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if ctx.Err() != nil {
				break
			}
			line := scanner.Text()
			if line != "" {
				str.AddLine(ctx, line, "stdin")
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "[stream] stdin error: %v\n", err)
		}
		// stdin closed (pipe ended), trigger shutdown
		cancel()
	}

	str.Wait()

	ingested, batches := str.Stats()
	fmt.Fprintf(os.Stderr, "[stream] Stopped. Ingested: %d lines, Batches flushed: %d, Total docs: %d\n",
		ingested, batches, store.Count())
	return 0
}

func extractFlag(args []string, flag string) ([]string, bool) {
	for i, arg := range args {
		if arg == flag {
			return append(args[:i], args[i+1:]...), true
		}
	}
	return args, false
}

func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExts := map[string]bool{
		".txt": true, ".md": true, ".go": true, ".py": true, ".js": true, ".log": true,
		".ts": true, ".rs": true, ".c": true, ".cpp": true, ".h": true,
		".java": true, ".rb": true, ".sh": true, ".yaml": true, ".yml": true,
		".json": true, ".xml": true, ".html": true, ".css": true, ".sql": true,
	}
	return textExts[ext]
}
