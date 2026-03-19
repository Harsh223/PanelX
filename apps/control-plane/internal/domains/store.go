package domains

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Store defines persistence operations for domains.
type Store interface {
	Create(domain Domain) error
	Update(domain Domain) error
	Delete(id string) error
	List() ([]Domain, error)
	GetByID(id string) (Domain, bool, error)
	GetByHostname(hostname string) (Domain, bool, error)
}

// FileStore persists domains to a JSON file while maintaining an in-memory
// hostname index for O(1)-style lookup by hostname.
type FileStore struct {
	path string

	mu            sync.Mutex
	data          map[string]Domain // id -> domain
	hostnameIndex map[string]string // normalized hostname -> id
}

// NewFileStore initializes a file-backed domain store.
func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("domain store path is required")
	}

	s := &FileStore{
		path:          path,
		data:          make(map[string]Domain),
		hostnameIndex: make(map[string]string),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// Create inserts a new domain record.
// Fails if either ID or hostname already exists.
func (s *FileStore) Create(domain Domain) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := strings.TrimSpace(domain.ID)
	if id == "" {
		return errors.New("domain id is required")
	}

	host, err := normalizeHostname(domain.Hostname)
	if err != nil {
		return err
	}

	if _, exists := s.data[id]; exists {
		return fmt.Errorf("domain with id %s already exists", id)
	}
	if existingID, exists := s.hostnameIndex[host]; exists {
		return fmt.Errorf("hostname %s is already mapped to domain id %s", host, existingID)
	}

	domain.ID = id
	domain.Hostname = host
	s.data[id] = cloneDomain(domain)
	s.hostnameIndex[host] = id

	return s.persistLocked()
}

// Update replaces an existing domain record by ID.
// It will reindex hostname changes safely and reject hostname collisions.
func (s *FileStore) Update(domain Domain) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := strings.TrimSpace(domain.ID)
	if id == "" {
		return errors.New("domain id is required")
	}

	existing, exists := s.data[id]
	if !exists {
		return fmt.Errorf("domain with id %s not found", id)
	}

	host, err := normalizeHostname(domain.Hostname)
	if err != nil {
		return err
	}

	if otherID, exists := s.hostnameIndex[host]; exists && otherID != id {
		return fmt.Errorf("hostname %s is already mapped to domain id %s", host, otherID)
	}

	previousHost, _ := normalizeHostname(existing.Hostname)
	if previousHost != host {
		delete(s.hostnameIndex, previousHost)
		s.hostnameIndex[host] = id
	}

	domain.ID = id
	domain.Hostname = host
	s.data[id] = cloneDomain(domain)

	return s.persistLocked()
}

// Delete removes a domain by ID.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("domain id is required")
	}

	existing, exists := s.data[id]
	if !exists {
		return fmt.Errorf("domain with id %s not found", id)
	}

	host, _ := normalizeHostname(existing.Hostname)
	delete(s.data, id)
	delete(s.hostnameIndex, host)

	return s.persistLocked()
}

// List returns all domains sorted by CreatedAt desc then Hostname asc.
func (s *FileStore) List() ([]Domain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Domain, 0, len(s.data))
	for _, item := range s.data {
		out = append(out, cloneDomain(item))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return strings.ToLower(out[i].Hostname) < strings.ToLower(out[j].Hostname)
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})

	return out, nil
}

// GetByID returns one domain by ID.
func (s *FileStore) GetByID(id string) (Domain, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return Domain{}, false, nil
	}

	item, ok := s.data[id]
	if !ok {
		return Domain{}, false, nil
	}
	return cloneDomain(item), true, nil
}

// GetByHostname returns one domain by hostname using the secondary index.
func (s *FileStore) GetByHostname(hostname string) (Domain, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	host, err := normalizeHostname(hostname)
	if err != nil {
		return Domain{}, false, nil
	}

	id, ok := s.hostnameIndex[host]
	if !ok {
		return Domain{}, false, nil
	}

	item, ok := s.data[id]
	if !ok {
		// Defensive self-heal in case of stale index.
		delete(s.hostnameIndex, host)
		return Domain{}, false, nil
	}

	return cloneDomain(item), true, nil
}

func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read domain store: %w", err)
	}
	if len(raw) == 0 {
		return nil
	}

	var entries []Domain
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("decode domain store: %w", err)
	}

	data := make(map[string]Domain, len(entries))
	index := make(map[string]string, len(entries))

	for _, d := range entries {
		id := strings.TrimSpace(d.ID)
		if id == "" {
			return errors.New("domain store contains entry with empty id")
		}

		host, err := normalizeHostname(d.Hostname)
		if err != nil {
			return fmt.Errorf("invalid hostname for id %s: %w", id, err)
		}

		if _, exists := data[id]; exists {
			return fmt.Errorf("duplicate domain id in store: %s", id)
		}
		if existingID, exists := index[host]; exists {
			return fmt.Errorf("duplicate hostname in store: %s (ids: %s, %s)", host, existingID, id)
		}

		d.ID = id
		d.Hostname = host
		data[id] = cloneDomain(d)
		index[host] = id
	}

	s.data = data
	s.hostnameIndex = index
	return nil
}

func (s *FileStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create domain store directory: %w", err)
	}

	entries := make([]Domain, 0, len(s.data))
	for _, item := range s.data {
		entries = append(entries, cloneDomain(item))
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return strings.ToLower(entries[i].Hostname) < strings.ToLower(entries[j].Hostname)
		}
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})

	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode domain store: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o640); err != nil {
		return fmt.Errorf("write domain store temp file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace domain store file: %w", err)
	}

	return nil
}

func normalizeHostname(hostname string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(hostname))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return "", errors.New("hostname is required")
	}
	if strings.Contains(host, " ") {
		return "", errors.New("hostname cannot contain spaces")
	}
	return host, nil
}

func cloneDomain(in Domain) Domain {
	out := in

	out.Health.ResolvedIPs = append([]string(nil), in.Health.ResolvedIPs...)
	out.Health.Warnings = append([]string(nil), in.Health.Warnings...)

	if in.LastSyncedAt != nil {
		t := *in.LastSyncedAt
		out.LastSyncedAt = &t
	}
	if in.TLS.IssuedAt != nil {
		t := *in.TLS.IssuedAt
		out.TLS.IssuedAt = &t
	}
	if in.TLS.ExpiresAt != nil {
		t := *in.TLS.ExpiresAt
		out.TLS.ExpiresAt = &t
	}
	if in.TLS.LastAttemptAt != nil {
		t := *in.TLS.LastAttemptAt
		out.TLS.LastAttemptAt = &t
	}

	return out
}
