package audit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
)

// File записывает события аудита в файл.
type File struct {
	file *os.File
	mu   sync.Mutex
}

// NewFile открывает файл аудита для добавления; вызывающий код должен вызвать Close.
func NewFile(path string) (*File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &File{file: f}, nil
}

// Name возвращает имя получателя.
func (*File) Name() string { return "file" }

// Observe записывает событие в файл в формате JSON; контекст не используется.
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

// Close закрывает файл и возвращает ошибку закрытия.
func (f *File) Close() error { f.mu.Lock(); defer f.mu.Unlock(); return f.file.Close() }
