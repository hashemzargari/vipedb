package vector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileCacheEntry struct {
	FilePath  string
	Hash      string
	ModTime   time.Time
	Size      int64
	DocCount  int
	IndexedAt time.Time
}

type CacheIndex struct {
	mu         sync.RWMutex
	entries    map[string]*FileCacheEntry
	cacheDir   string
	retention  time.Duration
	autoClean  bool
}

func NewCacheIndex(cacheDir string, retention time.Duration, autoClean bool) (*CacheIndex, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	cache := &CacheIndex{
		entries:   make(map[string]*FileCacheEntry),
		cacheDir:  cacheDir,
		retention: retention,
		autoClean: autoClean,
	}

	if err := cache.Load(); err != nil {
		return nil, err
	}

	if autoClean {
		cache.CleanExpired()
	}

	return cache, nil
}

func (c *CacheIndex) ComputeHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c *CacheIndex) Get(filePath string) (*FileCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[filePath]
	if !ok {
		return nil, false
	}

	if c.retention > 0 && time.Since(entry.IndexedAt) > c.retention {
		return nil, false
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, false
	}

	if info.ModTime() != entry.ModTime || info.Size() != entry.Size {
		currentHash, err := c.ComputeHash(filePath)
		if err != nil {
			return nil, false
		}

		if currentHash != entry.Hash {
			return nil, false
		}
	}

	return entry, true
}

func (c *CacheIndex) Set(filePath string, docCount int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	hash, err := c.ComputeHash(filePath)
	if err != nil {
		return err
	}

	c.entries[filePath] = &FileCacheEntry{
		FilePath:  filePath,
		Hash:      hash,
		ModTime:   info.ModTime(),
		Size:      info.Size(),
		DocCount:  docCount,
		IndexedAt: time.Now(),
	}

	return c.Save()
}

func (c *CacheIndex) Remove(filePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, filePath)
	return c.Save()
}

func (c *CacheIndex) RemovePattern(pattern string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var removed []string
	for path := range c.entries {
		matched, err := filepath.Match(pattern, path)
		if err != nil {
			continue
		}
		if matched {
			removed = append(removed, path)
			delete(c.entries, path)
		}
	}

	if err := c.Save(); err != nil {
		return nil, err
	}

	return removed, nil
}

func (c *CacheIndex) RemoveAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*FileCacheEntry)
	return c.Save()
}

func (c *CacheIndex) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.retention <= 0 {
		return 0
	}

	var cleaned int
	now := time.Now()
	for path, entry := range c.entries {
		if now.Sub(entry.IndexedAt) > c.retention {
			delete(c.entries, path)
			cleaned++
		}
	}

	if cleaned > 0 {
		c.Save()
	}

	return cleaned
}

func (c *CacheIndex) ClearOlderThan(age time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cleaned int
	now := time.Now()
	for path, entry := range c.entries {
		if now.Sub(entry.IndexedAt) > age {
			delete(c.entries, path)
			cleaned++
		}
	}

	if cleaned > 0 {
		c.Save()
	}

	return cleaned
}

func (c *CacheIndex) List() []FileCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]FileCacheEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, *entry)
	}

	return entries
}

func (c *CacheIndex) Save() error {
	cacheFile := filepath.Join(c.cacheDir, "cache.bin")
	f, err := os.Create(cacheFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	return enc.Encode(c.entries)
}

func (c *CacheIndex) Load() error {
	cacheFile := filepath.Join(c.cacheDir, "cache.bin")
	f, err := os.Open(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	return dec.Decode(&c.entries)
}

func (c *CacheIndex) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalDocs int
	for _, entry := range c.entries {
		totalDocs += entry.DocCount
	}

	return CacheStats{
		TotalFiles: len(c.entries),
		TotalDocs:  totalDocs,
		CacheDir:   c.cacheDir,
		Retention:  c.retention,
		AutoClean:  c.autoClean,
	}
}

type CacheStats struct {
	TotalFiles int
	TotalDocs  int
	CacheDir   string
	Retention  time.Duration
	AutoClean  bool
}

func (s CacheStats) String() string {
	retentionStr := "none"
	if s.Retention > 0 {
		retentionStr = s.Retention.String()
	}
	return fmt.Sprintf("Files: %d, Docs: %d, Retention: %s, AutoClean: %v",
		s.TotalFiles, s.TotalDocs, retentionStr, s.AutoClean)
}
