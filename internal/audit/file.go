package audit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
)

type File struct {
	file *os.File
	mu   sync.Mutex
}

func NewFile(path string) (*File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &File{file: f}, nil
}
func (*File) Name() string { return "file" }
func (f *File) Observe(_ context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := f.file.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
func (f *File) Close() error { f.mu.Lock(); defer f.mu.Unlock(); return f.file.Close() }
