package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/mikkims/ya/internal/auth"
	"github.com/mikkims/ya/internal/storage"
)

const (
	idLength        = 8
	alphabet        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	maxSaveAttempts = 3
)

var (
	ErrSaveAttemptsExceeded = errors.New("failed to save short URL after maximum attempts")
	ErrEmptyBatch           = errors.New("URL batch is empty")
	ErrOriginalURLExists    = errors.New("original URL already exists")
	ErrURLDeleted           = storage.ErrURLDeleted
	ErrDeleteWorkerStopped  = errors.New("delete worker is stopped")
)

type DeleteOptions struct {
	BufferSize    int
	FlushInterval time.Duration
}

type deleteRequest struct {
	ids    []string
	userID string
}

type Shortener struct {
	storage             URLStorage
	generateID          func() string
	deleteRequests      chan deleteRequest
	deleteBufferSize    int
	deleteFlushInterval time.Duration
	deleteDone          chan struct{}
	runOnce             sync.Once
}

type URLStorage interface {
	Save(ctx context.Context, id, originalURL string) error
	SaveBatch(ctx context.Context, urls []storage.URL) ([]storage.URL, error)
	Get(ctx context.Context, id string) (string, bool, error)
}

type userURLStorage interface {
	GetByUser(ctx context.Context, userID string) ([]storage.URL, error)
}

type userURLDeleter interface {
	Delete(ctx context.Context, ids []string, userID string) error
}

func (s *Shortener) SaveBatch(ctx context.Context, originalURLs []string) ([]string, error) {
	if len(originalURLs) == 0 {
		return nil, ErrEmptyBatch
	}

	for range maxSaveAttempts {
		urls := make([]storage.URL, len(originalURLs))
		ids := make([]string, len(originalURLs))
		usedIDs := make(map[string]struct{}, len(originalURLs))
		for i, originalURL := range originalURLs {
			id := s.generateUniqueID(usedIDs)
			usedIDs[id] = struct{}{}
			ids[i] = id
			urls[i] = storage.URL{ID: id, OriginalURL: originalURL, UserID: auth.UserID(ctx)}
		}

		savedURLs, err := s.storage.SaveBatch(ctx, urls)
		if errors.Is(err, storage.ErrIDExists) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for i := range savedURLs {
			ids[i] = savedURLs[i].ID
		}
		return ids, nil
	}

	return nil, ErrSaveAttemptsExceeded
}

func NewShortener(storage URLStorage, options ...DeleteOptions) *Shortener {
	deleteOptions := DeleteOptions{BufferSize: 100, FlushInterval: time.Second}
	if len(options) > 0 {
		if options[0].BufferSize > 0 {
			deleteOptions.BufferSize = options[0].BufferSize
		}
		if options[0].FlushInterval > 0 {
			deleteOptions.FlushInterval = options[0].FlushInterval
		}
	}
	return &Shortener{
		storage:             storage,
		generateID:          generateID,
		deleteRequests:      make(chan deleteRequest, deleteOptions.BufferSize),
		deleteBufferSize:    deleteOptions.BufferSize,
		deleteFlushInterval: deleteOptions.FlushInterval,
		deleteDone:          make(chan struct{}),
	}
}

func (s *Shortener) Save(ctx context.Context, originalURL string) (string, error) {
	for range maxSaveAttempts {
		id := s.generateID()
		err := s.storage.Save(ctx, id, originalURL)
		var originalURLExists *storage.OriginalURLExistsError
		if errors.As(err, &originalURLExists) {
			return originalURLExists.ID, ErrOriginalURLExists
		}
		if errors.Is(err, storage.ErrIDExists) {
			continue
		}
		if err != nil {
			return "", err
		}

		return id, nil
	}

	return "", ErrSaveAttemptsExceeded
}

func (s *Shortener) Get(ctx context.Context, id string) (string, bool, error) {
	return s.storage.Get(ctx, id)
}

func (s *Shortener) GetByUser(ctx context.Context, userID string) ([]storage.URL, error) {
	storage, ok := s.storage.(userURLStorage)
	if !ok {
		return nil, errors.New("storage does not support user URLs")
	}
	return storage.GetByUser(ctx, userID)
}

func (s *Shortener) Delete(ctx context.Context, ids []string, userID string) error {
	if _, ok := s.storage.(userURLDeleter); !ok {
		return errors.New("storage does not support URL deletion")
	}
	request := deleteRequest{ids: append([]string(nil), ids...), userID: userID}
	select {
	case <-s.deleteDone:
		return ErrDeleteWorkerStopped
	default:
	}
	select {
	case s.deleteRequests <- request:
		return nil
	case <-s.deleteDone:
		return ErrDeleteWorkerStopped
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Shortener) RunDeleteWorker(ctx context.Context) error {
	var runErr error
	s.runOnce.Do(func() {
		runErr = s.runDeleteWorker(ctx)
	})
	return runErr
}

func (s *Shortener) runDeleteWorker(ctx context.Context) error {
	deleter, ok := s.storage.(userURLDeleter)
	if !ok {
		close(s.deleteDone)
		return nil
	}

	ticker := time.NewTicker(s.deleteFlushInterval)
	defer ticker.Stop()
	buffer := make(map[string][]string)
	buffered := 0
	flush := func(flushCtx context.Context) error {
		for userID, ids := range buffer {
			if err := deleter.Delete(flushCtx, ids, userID); err != nil {
				return fmt.Errorf("flush URL deletions: %w", err)
			}
		}
		clear(buffer)
		buffered = 0
		return nil
	}

	for {
		select {
		case request := <-s.deleteRequests:
			buffer[request.userID] = append(buffer[request.userID], request.ids...)
			buffered += len(request.ids)
			if buffered >= s.deleteBufferSize {
				if err := flush(ctx); err != nil {
					close(s.deleteDone)
					return err
				}
			}
		case <-ticker.C:
			if buffered > 0 {
				if err := flush(ctx); err != nil {
					close(s.deleteDone)
					return err
				}
			}
		case <-ctx.Done():
			close(s.deleteDone)
			for {
				select {
				case request := <-s.deleteRequests:
					buffer[request.userID] = append(buffer[request.userID], request.ids...)
					buffered += len(request.ids)
				default:
					shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
					defer cancel()
					if buffered > 0 {
						return flush(shutdownCtx)
					}
					return nil
				}
			}
		}
	}
}

func generateID() string {
	id := make([]byte, idLength)
	for i := range id {
		id[i] = alphabet[rand.Intn(len(alphabet))]
	}

	return string(id)
}

func (s *Shortener) generateUniqueID(used map[string]struct{}) string {
	for {
		id := s.generateID()
		if _, exists := used[id]; !exists {
			return id
		}
	}
}
