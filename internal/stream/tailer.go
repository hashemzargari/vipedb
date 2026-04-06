package stream

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"
)

// Tailer monitors a file for appended content, similar to tail -f.
// It polls the file at a fixed interval and handles truncation (log rotation).
type Tailer struct {
	path         string
	pollInterval time.Duration
}

func NewTailer(path string) *Tailer {
	return &Tailer{
		path:         path,
		pollInterval: 100 * time.Millisecond,
	}
}

// Run tails the file and calls emit for each complete line.
// It seeks to the end of the file on start, only processing new content.
// On file truncation (log rotation), it resets to the beginning.
func (t *Tailer) Run(ctx context.Context, emit func(line string)) error {
	f, err := os.Open(t.path)
	if err != nil {
		return err
	}
	defer f.Close()

	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	// Pre-allocate read buffer (32KB) and partial line buffer
	buf := make([]byte, 32*1024)
	partial := make([]byte, 0, 4096)

	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			info, err := os.Stat(t.path)
			if err != nil {
				continue
			}

			// Detect truncation (log rotation)
			if info.Size() < offset {
				offset = 0
				f.Seek(0, io.SeekStart)
				partial = partial[:0]
			}

			// Read all available new data
			for {
				n, readErr := f.Read(buf)
				if n > 0 {
					offset += int64(n)
					data := buf[:n]

					// Prepend any partial line from previous read
					if len(partial) > 0 {
						data = append(partial, data...)
						partial = partial[:0]
					}

					// Extract complete lines
					for {
						idx := bytes.IndexByte(data, '\n')
						if idx < 0 {
							// Save incomplete line for next iteration
							partial = append(partial[:0], data...)
							break
						}
						line := data[:idx]
						data = data[idx+1:]

						// Strip \r for Windows-style line endings
						if len(line) > 0 && line[len(line)-1] == '\r' {
							line = line[:len(line)-1]
						}

						if len(line) > 0 {
							emit(string(line))
						}
					}
				}
				if readErr != nil {
					break
				}
			}
		}
	}
}
