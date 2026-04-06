package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

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
	return &Config{
		Models: ModelsConfig{
			Directory:    "./models",
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
			Directory: "./.vipedb",
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
			Directory: "./.vipedb/cache",
			Retention: "720h",
			AutoClean: true,
		},
	}
}

func Load(path string) (*Config, error) {
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
