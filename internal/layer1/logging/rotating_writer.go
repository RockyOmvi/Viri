package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type RotatingFileWriter struct {
	mu         sync.Mutex
	dir        string
	basename   string
	maxSize    int64
	maxBackups int
	current    *os.File
	size       int64
	date       string
}

func NewRotatingFileWriter(dir, basename string, maxSizeMB int, maxBackups int) (*RotatingFileWriter, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	w := &RotatingFileWriter{
		dir:        dir,
		basename:   basename,
		maxSize:    int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
		date:       time.Now().Format("2006-01-02"),
	}

	if err := w.rotate(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *RotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if today != w.date || w.size+int64(len(p)) > w.maxSize {
		w.current.Close()
		w.date = today
		w.size = 0
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.current.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		return w.current.Close()
	}
	return nil
}

func (w *RotatingFileWriter) rotate() error {
	if w.current != nil {
		w.current.Close()
	}

	filename := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.basename, w.date))
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	info, err := f.Stat()
	if err == nil {
		w.size = info.Size()
	}

	w.current = f
	w.cleanup()
	return nil
}

func (w *RotatingFileWriter) cleanup() {
	if w.maxBackups <= 0 {
		return
	}

	pattern := filepath.Join(w.dir, w.basename+"-*.log")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	sort.Strings(matches)

	if len(matches) > w.maxBackups {
		for _, f := range matches[:len(matches)-w.maxBackups] {
			os.Remove(f)
		}
	}
}
