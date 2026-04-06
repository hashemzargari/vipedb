package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const hfBaseURL = "https://huggingface.co/hashemzargari/mpmodels/resolve/main"

// ModelSpec defines a downloadable model with its required files.
type ModelSpec struct {
	Name  string
	Dir   string
	Files []string
}

// AvailableModels returns the list of models available for download.
func AvailableModels() []ModelSpec {
	return []ModelSpec{
		{
			Name:  "BAAI/bge-small-en-v1.5 (FP16)",
			Dir:   "bge-small-en-v1.5",
			Files: []string{"model.mpmodel", "vocab.txt"},
		},
		{
			Name:  "intfloat/multilingual-e5-small (FP16)",
			Dir:   "multilingual-e5-small-fp16",
			Files: []string{"model.mpmodel", "vocab.txt"},
		},
	}
}

// DownloadModel downloads a model spec into the models directory.
// progress is called with (bytesWritten, fileName) during download.
func DownloadModel(modelsDir string, spec ModelSpec, progress func(n int64, file string)) error {
	destDir := filepath.Join(modelsDir, spec.Dir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create model dir: %w", err)
	}

	for _, fileName := range spec.Files {
		destPath := filepath.Join(destDir, fileName)

		// Skip if already exists
		if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
			if progress != nil {
				progress(info.Size(), fileName)
			}
			continue
		}

		url := fmt.Sprintf("%s/%s/%s", hfBaseURL, spec.Dir, fileName)
		if err := downloadFile(url, destPath, func(n int64) {
			if progress != nil {
				progress(n, fileName)
			}
		}); err != nil {
			return fmt.Errorf("download %s: %w", fileName, err)
		}
	}
	return nil
}

// IsModelInstalled checks if a model's files exist in the models directory.
func IsModelInstalled(modelsDir, dirName string) bool {
	mpmodel := filepath.Join(modelsDir, dirName, "model.mpmodel")
	vocab := filepath.Join(modelsDir, dirName, "vocab.txt")
	if _, err := os.Stat(mpmodel); err != nil {
		return false
	}
	if _, err := os.Stat(vocab); err != nil {
		return false
	}
	return true
}

func downloadFile(url, dest string, progress func(int64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	// Write to temp file first, then rename (atomic)
	tmpPath := dest + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			nw, writeErr := f.Write(buf[:n])
			if writeErr != nil {
				f.Close()
				os.Remove(tmpPath)
				return writeErr
			}
			written += int64(nw)
			if progress != nil {
				progress(written)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			f.Close()
			os.Remove(tmpPath)
			return readErr
		}
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	return os.Rename(tmpPath, dest)
}
