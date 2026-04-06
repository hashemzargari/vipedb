package stream

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashemzargari/vipedb/internal/embedding"
	"github.com/hashemzargari/vipedb/pkg/vector"
)

type Config struct {
	BatchSize     int
	FlushInterval time.Duration
	Workers       int
	Source        string
	Verbose       bool
}

func DefaultConfig() Config {
	return Config{
		BatchSize:     50,
		FlushInterval: 2 * time.Second,
		Workers:       4,
		Source:        "stdin",
	}
}

type lineEntry struct {
	text   string
	source string
	ts     time.Time
}

type Streamer struct {
	pool     *embedding.Pool
	store    *vector.Store
	config   Config
	lines    chan lineEntry
	wg       sync.WaitGroup
	ingested uint64
	batches  uint64
}

func New(pool *embedding.Pool, store *vector.Store, config Config) *Streamer {
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 2 * time.Second
	}
	if config.Workers <= 0 {
		config.Workers = 4
	}

	return &Streamer{
		pool:   pool,
		store:  store,
		config: config,
		lines:  make(chan lineEntry, config.BatchSize*4),
	}
}

// AddLine sends a line into the ingestion pipeline. Non-blocking on context cancellation.
func (s *Streamer) AddLine(ctx context.Context, text, source string) {
	select {
	case s.lines <- lineEntry{text: text, source: source, ts: time.Now()}:
	case <-ctx.Done():
	}
}

// Run starts the batch collector and worker goroutines.
func (s *Streamer) Run(ctx context.Context) {
	batchCh := make(chan []lineEntry, s.config.Workers)

	s.wg.Add(1)
	go s.batchCollector(ctx, batchCh)

	for i := 0; i < s.config.Workers; i++ {
		s.wg.Add(1)
		go s.worker(batchCh)
	}
}

func (s *Streamer) batchCollector(ctx context.Context, out chan<- []lineEntry) {
	defer s.wg.Done()
	defer close(out)

	// Pre-allocate batch buffer, reuse backing array
	batch := make([]lineEntry, 0, s.config.BatchSize)
	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		b := make([]lineEntry, len(batch))
		copy(b, batch)
		out <- b
		batch = batch[:0]
	}

	for {
		select {
		case line := <-s.lines:
			batch = append(batch, line)
			if len(batch) >= s.config.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			// Drain remaining buffered lines for graceful shutdown
			for {
				select {
				case line := <-s.lines:
					batch = append(batch, line)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (s *Streamer) worker(batches <-chan []lineEntry) {
	defer s.wg.Done()

	for batch := range batches {
		s.processBatch(batch)
	}
}

func (s *Streamer) processBatch(batch []lineEntry) {
	for _, entry := range batch {
		// Use Background context so shutdown doesn't cancel in-flight embeddings
		emb, err := s.pool.Embed(context.Background(), entry.text)
		if err != nil {
			if s.config.Verbose {
				fmt.Fprintf(os.Stderr, "[stream] embed error: %v\n", err)
			}
			continue
		}

		doc := vector.Document{
			ID:        fmt.Sprintf("stream:%s:%d", entry.source, entry.ts.UnixNano()),
			Content:   entry.text,
			Embedding: emb,
			Metadata: map[string]string{
				"source":    entry.source,
				"timestamp": entry.ts.Format(time.RFC3339Nano),
			},
		}

		s.store.Add(doc)
		atomic.AddUint64(&s.ingested, 1)
	}

	// Atomically flush batch to persistent storage
	if err := s.store.Save(); err != nil {
		if s.config.Verbose {
			fmt.Fprintf(os.Stderr, "[stream] save error: %v\n", err)
		}
	}
	atomic.AddUint64(&s.batches, 1)

	if s.config.Verbose {
		fmt.Fprintf(os.Stderr, "[stream] flushed batch (%d lines, total ingested: %d)\n",
			len(batch), atomic.LoadUint64(&s.ingested))
	}
}

// Wait blocks until all workers and the batch collector have finished.
func (s *Streamer) Wait() {
	s.wg.Wait()
}

// Stats returns the current ingestion counters.
func (s *Streamer) Stats() (ingested, batches uint64) {
	return atomic.LoadUint64(&s.ingested), atomic.LoadUint64(&s.batches)
}
