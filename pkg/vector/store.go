package vector

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu        sync.RWMutex
	documents map[string]Document
	indexDir  string
}

func NewStore(indexDir string) *Store {
	return &Store{
		documents: make(map[string]Document),
		indexDir:  indexDir,
	}
}

func (s *Store) Add(doc Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(doc.Embedding) == 0 {
		return ErrEmptyEmbedding
	}

	s.documents[doc.ID] = doc
	return nil
}

func (s *Store) AddBatch(docs []Document) error {
	for _, doc := range docs {
		if err := s.Add(doc); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Get(id string) (Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.documents[id]
	return doc, ok
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.documents, id)
	return nil
}

func (s *Store) Search(query Vector, topK int) []SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := make([]Document, 0, len(s.documents))
	for _, doc := range s.documents {
		docs = append(docs, doc)
	}

	return Search(query, docs, topK)
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.documents)
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := os.MkdirAll(s.indexDir, 0o755); err != nil {
		return err
	}

	indexFile := filepath.Join(s.indexDir, "index.bin")
	f, err := os.Create(indexFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	return enc.Encode(s.documents)
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	indexFile := filepath.Join(s.indexDir, "index.bin")
	f, err := os.Open(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	return dec.Decode(&s.documents)
}

func (s *Store) Close() error {
	return s.Save()
}
