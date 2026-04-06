package embedding

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoMemPipe/memrag/model"
	"github.com/GoMemPipe/memrag/model/descriptors"
	"github.com/GoMemPipe/memrag/pipeline"
)

type ModelConfig struct {
	DescriptorName string
	ModelDir       string
}

type Service struct {
	mu     sync.Mutex
	pipe   *pipeline.EmbeddingPipeline
	desc   model.Descriptor
	config ModelConfig
	closed bool
}

var (
	modelCache = make(map[string]*Service)
	cacheMu    sync.RWMutex
)

func NewService(config ModelConfig) (*Service, error) {
	cacheMu.RLock()
	if cached, ok := modelCache[config.ModelDir]; ok {
		cacheMu.RUnlock()
		return cached, nil
	}
	cacheMu.RUnlock()

	desc, ok := descriptors.Get(config.DescriptorName)
	if !ok {
		return nil, fmt.Errorf("descriptor %q not found", config.DescriptorName)
	}

	assets, err := model.LoadAssetsFromDir(config.ModelDir)
	if err != nil {
		return nil, fmt.Errorf("load assets: %w", err)
	}

	pipe, err := pipeline.New(desc, assets)
	if err != nil {
		return nil, fmt.Errorf("create pipeline: %w", err)
	}

	svc := &Service{
		pipe:   pipe,
		desc:   desc,
		config: config,
	}

	cacheMu.Lock()
	modelCache[config.ModelDir] = svc
	cacheMu.Unlock()

	return svc, nil
}

func (s *Service) Embed(ctx context.Context, text string) ([]float32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrServiceClosed
	}

	return s.pipe.Embed(ctx, text)
}

func (s *Service) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := s.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

func (s *Service) Dimension() int {
	return s.desc.EmbeddingDimension()
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	s.pipe.Close()

	cacheMu.Lock()
	delete(modelCache, s.config.ModelDir)
	cacheMu.Unlock()

	return nil
}
