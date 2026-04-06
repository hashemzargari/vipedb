package embedding

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoMemPipe/memrag/model"
	"github.com/GoMemPipe/memrag/model/descriptors"
	"github.com/GoMemPipe/memrag/pipeline"
)

// Pool provides concurrent embedding via multiple pipeline instances.
// Uses a channel-based resource pool for zero-contention pipeline access.
type Pool struct {
	mu        sync.Mutex
	pipelines []*pipeline.EmbeddingPipeline
	pipes     chan *pipeline.EmbeddingPipeline
	desc      model.Descriptor
	closed    bool
}

func NewPool(config ModelConfig, size int) (*Pool, error) {
	if size <= 0 {
		size = 1
	}

	desc, ok := descriptors.Get(config.DescriptorName)
	if !ok {
		return nil, fmt.Errorf("descriptor %q not found", config.DescriptorName)
	}

	pool := &Pool{
		pipelines: make([]*pipeline.EmbeddingPipeline, 0, size),
		pipes:     make(chan *pipeline.EmbeddingPipeline, size),
		desc:      desc,
	}

	for i := 0; i < size; i++ {
		assets, err := model.LoadAssetsFromDir(config.ModelDir)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("load assets for pool member %d: %w", i, err)
		}
		pipe, err := pipeline.New(desc, assets)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("create pipeline for pool member %d: %w", i, err)
		}
		pool.pipelines = append(pool.pipelines, pipe)
		pool.pipes <- pipe
	}

	return pool, nil
}

// Embed acquires a pipeline from the pool, runs inference, and returns it.
func (p *Pool) Embed(ctx context.Context, text string) ([]float32, error) {
	select {
	case pipe := <-p.pipes:
		result, err := pipe.Embed(ctx, text)
		p.pipes <- pipe
		return result, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pool) Dimension() int {
	return p.desc.EmbeddingDimension()
}

func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	for _, pipe := range p.pipelines {
		if pipe != nil {
			pipe.Close()
		}
	}
	return nil
}
