package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type fileRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type File struct {
	path     string
	urls     map[string]string
	records  []fileRecord
	nextUUID int
	mu       sync.RWMutex
}

func NewFile(path string) (*File, error) {
	if path == "" {
		return nil, errors.New("storage file path is empty")
	}

	storage := &File{
		path:     path,
		urls:     make(map[string]string),
		nextUUID: 1,
	}
	if err := storage.load(); err != nil {
		return nil, err
	}
	return storage, nil
}

func (s *File) Save(id, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[id]; exists {
		return ErrIDExists
	}

	record := fileRecord{
		UUID:        strconv.Itoa(s.nextUUID),
		ShortURL:    id,
		OriginalURL: originalURL,
	}
	records := append(append([]fileRecord(nil), s.records...), record)
	if err := s.persist(records); err != nil {
		return err
	}

	s.records = records
	s.urls[id] = originalURL
	s.nextUUID++
	return nil
}

func (s *File) Get(id string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	originalURL, ok := s.urls[id]
	return originalURL, ok
}

func (s *File) load() error {
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open storage file: %w", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&s.records); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode storage file: %w", err)
	}
	if s.records == nil {
		return errors.New("storage file must contain a JSON array")
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("storage file contains multiple JSON values")
		}
		return fmt.Errorf("decode storage file: %w", err)
	}

	uuids := make(map[string]struct{}, len(s.records))
	for _, record := range s.records {
		if record.UUID == "" || record.ShortURL == "" || record.OriginalURL == "" {
			return errors.New("storage file contains an incomplete record")
		}
		if _, exists := uuids[record.UUID]; exists {
			return fmt.Errorf("storage file contains duplicate UUID %q", record.UUID)
		}
		uuids[record.UUID] = struct{}{}
		if _, exists := s.urls[record.ShortURL]; exists {
			return fmt.Errorf("storage file contains duplicate short URL %q", record.ShortURL)
		}
		s.urls[record.ShortURL] = record.OriginalURL
		uuid, err := strconv.Atoi(record.UUID)
		if err != nil || uuid < 1 {
			return fmt.Errorf("storage file contains invalid UUID %q", record.UUID)
		}
		if uuid >= s.nextUUID {
			s.nextUUID = uuid + 1
		}
	}
	return nil
}

func (s *File) persist(records []fileRecord) error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	temporary, err := os.CreateTemp(directory, ".short-url-storage-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary storage file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {

		}
	}(temporaryPath)

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(records); err != nil {
		err := temporary.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("encode storage file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		err := temporary.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("sync storage file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close storage file: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace storage file: %w", err)
	}
	return nil
}
