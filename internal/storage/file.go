package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/mikkims/ya/internal/auth"
	"github.com/rs/zerolog"
)

type fileRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
	Deleted     bool   `json:"is_deleted,omitempty"`
}

type File struct {
	path         string
	urls         map[string]string
	originalURLs map[string]string
	records      []fileRecord
	nextUUID     int
	mu           sync.RWMutex
	logger       zerolog.Logger
}

func NewFile(path string, logger zerolog.Logger) (*File, error) {
	if path == "" {
		return nil, errors.New("storage file path is empty")
	}

	storage := &File{
		path:         path,
		urls:         make(map[string]string),
		originalURLs: make(map[string]string),
		nextUUID:     1,
		logger:       logger,
	}
	if err := storage.load(); err != nil {
		return nil, err
	}
	return storage, nil
}

func (s *File) Save(ctx context.Context, id, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, exists := s.originalURLs[originalURL]; exists {
		return &OriginalURLExistsError{ID: existingID}
	}
	if _, exists := s.urls[id]; exists {
		return ErrIDExists
	}

	record := fileRecord{
		UUID:        strconv.Itoa(s.nextUUID),
		ShortURL:    id,
		OriginalURL: originalURL,
		UserID:      auth.UserID(ctx),
	}
	records := append(append([]fileRecord(nil), s.records...), record)
	if err := s.persist(records); err != nil {
		return err
	}

	s.records = records
	s.urls[id] = originalURL
	s.originalURLs[originalURL] = id
	s.nextUUID++
	return nil
}

func (s *File) SaveBatch(_ context.Context, urls []URL) ([]URL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make(map[string]struct{}, len(urls))
	pending := make(map[string]URL, len(urls))
	result := make([]URL, len(urls))
	for i, url := range urls {
		if existingID, exists := s.originalURLs[url.OriginalURL]; exists {
			result[i] = URL{ID: existingID, OriginalURL: url.OriginalURL}
			continue
		}
		if existingURL, exists := pending[url.OriginalURL]; exists {
			result[i] = existingURL
			continue
		}
		if _, exists := s.urls[url.ID]; exists {
			return nil, ErrIDExists
		}
		if _, exists := ids[url.ID]; exists {
			return nil, ErrIDExists
		}
		ids[url.ID] = struct{}{}
		pending[url.OriginalURL] = url
		result[i] = url
	}

	records := append([]fileRecord(nil), s.records...)
	for _, url := range pending {
		records = append(records, fileRecord{
			UUID:        strconv.Itoa(s.nextUUID + len(records) - len(s.records)),
			ShortURL:    url.ID,
			OriginalURL: url.OriginalURL,
			UserID:      url.UserID,
		})
	}
	if err := s.persist(records); err != nil {
		return nil, err
	}

	for _, url := range pending {
		s.urls[url.ID] = url.OriginalURL
		s.originalURLs[url.OriginalURL] = url.ID
	}
	s.records = records
	s.nextUUID += len(pending)
	return result, nil
}

func (s *File) Get(_ context.Context, id string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, record := range s.records {
		if record.ShortURL == id && record.Deleted {
			return "", false, ErrURLDeleted
		}
	}
	originalURL, ok := s.urls[id]
	return originalURL, ok, nil
}

func (s *File) GetByUser(_ context.Context, userID string) ([]URL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	urls := make([]URL, 0)
	for _, record := range s.records {
		if record.UserID == userID && !record.Deleted {
			urls = append(urls, URL{ID: record.ShortURL, OriginalURL: record.OriginalURL, UserID: record.UserID})
		}
	}
	return urls, nil
}

func (s *File) Delete(_ context.Context, ids []string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleteIDs := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		deleteIDs[id] = struct{}{}
	}
	records := append([]fileRecord(nil), s.records...)
	for i := range records {
		if _, ok := deleteIDs[records[i].ShortURL]; ok && records[i].UserID == userID {
			records[i].Deleted = true
		}
	}
	if err := s.persist(records); err != nil {
		return err
	}
	s.records = records
	return nil
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
		if err := file.Close(); err != nil {
			s.logger.Error().Err(err).Str("path", s.path).Msg("failed to close storage file")
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
		if existingID, exists := s.originalURLs[record.OriginalURL]; exists {
			return fmt.Errorf("storage file contains duplicate original URL for IDs %q and %q", existingID, record.ShortURL)
		}
		s.originalURLs[record.OriginalURL] = record.ShortURL
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
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Error().Err(err).Str("path", name).Msg("failed to remove temporary storage file")
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
