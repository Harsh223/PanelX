package installations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Store defines persistence operations for installations.
type Store interface {
	Create(installation Installation) error
	List() ([]Installation, error)
	GetByID(id string) (Installation, bool, error)
}

// FileStore persists installations to a JSON file.
type FileStore struct {
	path string
	mu   sync.Mutex
	data map[string]Installation
}

// NewFileStore initializes a file-backed installation store.
func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path: path,
		data: map[string]Installation{},
	}

	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileStore) Create(installation Installation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[installation.ID]; exists {
		return fmt.Errorf("installation with id %s already exists", installation.ID)
	}

	s.data[installation.ID] = installation
	return s.persist()
}

func (s *FileStore) List() ([]Installation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Installation, 0, len(s.data))
	for _, item := range s.data {
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})

	return out, nil
}

func (s *FileStore) GetByID(id string) (Installation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.data[id]
	return item, ok, nil
}

func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read installation store: %w", err)
	}

	if len(raw) == 0 {
		return nil
	}

	var entries []Installation
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("decode installation store: %w", err)
	}

	for _, entry := range entries {
		s.data[entry.ID] = entry
	}
	return nil
}

func (s *FileStore) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create installation store directory: %w", err)
	}

	entries := make([]Installation, 0, len(s.data))
	for _, item := range s.data {
		entries = append(entries, item)
	}

	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode installation store: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o640); err != nil {
		return fmt.Errorf("write installation store temp file: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace installation store file: %w", err)
	}

	return nil
}
