package config

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// Workspace describes the resolved VipeDB data directory and whether
// it is a local (per-project) workspace or the global home.
type Workspace struct {
	Path    string // Absolute path to the .vipe directory
	IsLocal bool   // true when CWD contains a .vipe directory
}

// ResolveWorkspace determines which .vipe directory to use.
//
// Resolution order:
//  1. VIPE_HOME environment variable (explicit override, always wins)
//  2. ./.vipe directory in the current working directory (local workspace)
//  3. ~/.vipe (global fallback)
func ResolveWorkspace() Workspace {
	// 1. Explicit env override.
	if v := os.Getenv("VIPE_HOME"); v != "" {
		return Workspace{Path: v, IsLocal: false}
	}

	// 2. Local workspace: .vipe in CWD.
	if cwd, err := os.Getwd(); err == nil {
		local := filepath.Join(cwd, ".vipe")
		if info, err := os.Stat(local); err == nil && info.IsDir() {
			return Workspace{Path: local, IsLocal: true}
		}
	}

	// 3. Global fallback: ~/.vipe.
	return Workspace{Path: globalHome(), IsLocal: false}
}

// globalHome returns the global ~/.vipe path without checking for local overrides.
func globalHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		if runtime.GOOS == "windows" {
			home = os.Getenv("USERPROFILE")
		} else {
			home = os.Getenv("HOME")
		}
	}
	return filepath.Join(home, ".vipe")
}

// VipeHome returns the active vipe data directory.
// It respects VIPE_HOME, local .vipe, and global ~/.vipe in that order.
func VipeHome() string {
	return ResolveWorkspace().Path
}

// GlobalHome returns the global ~/.vipe path, ignoring local workspaces.
// Used by "vipe init" (without --local) to always target the global home.
func GlobalHome() string {
	if v := os.Getenv("VIPE_HOME"); v != "" {
		return v
	}
	return globalHome()
}

// ConfigPath returns the default config file path for the active workspace.
func ConfigPath() string {
	return filepath.Join(VipeHome(), "config.yaml")
}

type Config struct {
	Models  ModelsConfig  `yaml:"models"`
	Index   IndexConfig   `yaml:"index"`
	Search  SearchConfig  `yaml:"search"`
	General GeneralConfig `yaml:"general"`
	Cache   CacheConfig   `yaml:"cache"`
}

type ModelsConfig struct {
	Directory    string            `yaml:"directory"`
	DefaultModel string            `yaml:"default"`
	Models       map[string]string `yaml:"models"`
}

type IndexConfig struct {
	Directory string `yaml:"directory"`
}

type SearchConfig struct {
	DefaultTopK int     `yaml:"default_top_k"`
	Threshold   float32 `yaml:"threshold"`
}

type GeneralConfig struct {
	Verbose bool `yaml:"verbose"`
}

type CacheConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Directory    string        `yaml:"directory"`
	Retention    string        `yaml:"retention"`
	RetentionDur time.Duration `yaml:"-"`
	AutoClean    bool          `yaml:"auto_clean"`
}

func Default() *Config {
	return DefaultFor("")
}

// DefaultFor returns a Config with all paths rooted under root.
// If root is empty it defaults to VipeHome().
func DefaultFor(root string) *Config {
	if root == "" {
		root = VipeHome()
	}
	return &Config{
		Models: ModelsConfig{
			Directory:    filepath.Join(root, "models"),
			DefaultModel: "BAAI/bge-small-en-v1.5",
			Models: map[string]string{
				"bge-small":                           "bge-small-en-v1.5",
				"e5-small":                            "multilingual-e5-small-fp16",
				"minilm":                              "paraphrase-multilingual-MiniLM-L12-v2",
				"BAAI/bge-small-en-v1.5":              "bge-small-en-v1.5",
				"intfloat/multilingual-e5-small":      "multilingual-e5-small",
				"intfloat/multilingual-e5-small-fp16": "multilingual-e5-small-fp16",
			},
		},
		Index: IndexConfig{
			Directory: filepath.Join(root, "index"),
		},
		Search: SearchConfig{
			DefaultTopK: 10,
			Threshold:   0.0,
		},
		General: GeneralConfig{
			Verbose: false,
		},
		Cache: CacheConfig{
			Enabled:   true,
			Directory: filepath.Join(root, "cache"),
			Retention: "720h",
			AutoClean: true,
		},
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = ConfigPath()
	}

	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.Cache.Retention != "" {
		cfg.Cache.RetentionDur, _ = time.ParseDuration(cfg.Cache.Retention)
	}

	return cfg, nil
}

// EnsureHome creates the directory structure at the given root.
// If root is empty it defaults to VipeHome().
func EnsureHome(root string) error {
	if root == "" {
		root = VipeHome()
	}
	dirs := []string{
		root,
		filepath.Join(root, "models"),
		filepath.Join(root, "index"),
		filepath.Join(root, "cache"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func (c *Config) ModelPath(modelName string) string {
	if path, ok := c.Models.Models[modelName]; ok {
		return filepath.Join(c.Models.Directory, path)
	}

	if c.Models.DefaultModel != "" {
		if path, ok := c.Models.Models[c.Models.DefaultModel]; ok {
			return filepath.Join(c.Models.Directory, path)
		}
	}

	return ""
}

func (c *Config) ModelDescriptor(modelName string) string {
	if path, ok := c.Models.Models[modelName]; ok {
		switch path {
		case "bge-small-en-v1.5":
			return "BAAI/bge-small-en-v1.5"
		case "multilingual-e5-small-fp16":
			return "intfloat/multilingual-e5-small-fp16"
		case "multilingual-e5-small":
			return "intfloat/multilingual-e5-small"
		case "paraphrase-multilingual-MiniLM-L12-v2":
			return "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
		default:
			return path
		}
	}

	if c.Models.DefaultModel != "" {
		return c.Models.DefaultModel
	}

	return modelName
}
