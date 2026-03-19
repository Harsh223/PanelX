package filesvc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Service provides constrained file operations per site.
type Service struct {
	sitesRoot string
}

// Entry is a file listing item.
type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// NewService creates a file service rooted at sitesRoot.
func NewService(sitesRoot string) *Service {
	return &Service{sitesRoot: sitesRoot}
}

// List returns files/directories under a relative path in a domain root.
func (s *Service) List(domain, relativePath string) ([]Entry, error) {
	resolved, err := s.resolve(domain, relativePath)
	if err != nil {
		return nil, err
	}

	items, err := os.ReadDir(resolved)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			Name:  item.Name(),
			Path:  filepath.ToSlash(filepath.Join(relativePath, item.Name())),
			IsDir: item.IsDir(),
			Size:  info.Size(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

// Read reads a text file content under domain site root.
func (s *Service) Read(domain, relativePath string) (string, error) {
	resolved, err := s.resolve(domain, relativePath)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

// Write writes content to a text file under domain site root.
func (s *Service) Write(domain, relativePath, content string) error {
	resolved, err := s.resolve(domain, relativePath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	if err := os.WriteFile(resolved, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Delete deletes file or directory recursively.
func (s *Service) Delete(domain, relativePath string) error {
	resolved, err := s.resolve(domain, relativePath)
	if err != nil {
		return err
	}

	if resolved == filepath.Join(s.sitesRoot, domain, "public_html") {
		return errors.New("refusing to delete site root")
	}

	if err := os.RemoveAll(resolved); err != nil {
		return fmt.Errorf("delete path: %w", err)
	}
	return nil
}

func (s *Service) resolve(domain, relativePath string) (string, error) {
	if strings.TrimSpace(domain) == "" {
		return "", errors.New("domain is required")
	}

	siteRoot := filepath.Join(s.sitesRoot, domain, "public_html")
	cleanRel := filepath.Clean("/" + strings.TrimSpace(relativePath))
	cleanRel = strings.TrimPrefix(cleanRel, "/")

	resolved := filepath.Join(siteRoot, cleanRel)
	resolved = filepath.Clean(resolved)
	siteRootClean := filepath.Clean(siteRoot)

	if resolved != siteRootClean && !strings.HasPrefix(resolved, siteRootClean+string(os.PathSeparator)) {
		return "", errors.New("path escapes site root")
	}

	return resolved, nil
}
