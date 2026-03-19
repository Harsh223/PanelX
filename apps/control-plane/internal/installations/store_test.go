package installations

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStore_CreateListGet(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), ".panelx", "installations.json")
	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	older := testInstallation("inst-older", "example.com", "/", time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC))
	newer := testInstallation("inst-newer", "blog.example.com", "/blog", time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC))

	if err := store.Create(older); err != nil {
		t.Fatalf("Create(older) error = %v", err)
	}
	if err := store.Create(newer); err != nil {
		t.Fatalf("Create(newer) error = %v", err)
	}

	// Duplicate ID should fail.
	if err := store.Create(newer); err == nil {
		t.Fatalf("Create(duplicate) expected error, got nil")
	}

	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("List() len = %d, want 2", len(items))
	}

	// List should be sorted by CreatedAt descending (newest first).
	if items[0].ID != newer.ID {
		t.Fatalf("List()[0].ID = %q, want %q", items[0].ID, newer.ID)
	}
	if items[1].ID != older.ID {
		t.Fatalf("List()[1].ID = %q, want %q", items[1].ID, older.ID)
	}

	got, ok, err := store.GetByID(newer.ID)
	if err != nil {
		t.Fatalf("GetByID(existing) error = %v", err)
	}
	if !ok {
		t.Fatalf("GetByID(existing) ok = false, want true")
	}
	if got.ID != newer.ID || got.Domain != newer.Domain || got.InstallPath != newer.InstallPath {
		t.Fatalf("GetByID(existing) = %+v, want ID=%q Domain=%q InstallPath=%q", got, newer.ID, newer.Domain, newer.InstallPath)
	}

	_, ok, err = store.GetByID("does-not-exist")
	if err != nil {
		t.Fatalf("GetByID(missing) error = %v", err)
	}
	if ok {
		t.Fatalf("GetByID(missing) ok = true, want false")
	}
}

func TestFileStore_PersistenceAcrossInstances(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), ".panelx", "installations.json")
	initial, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore(initial) error = %v", err)
	}

	item := testInstallation("inst-persist", "persist.example.com", "/shop", time.Date(2026, 2, 10, 8, 30, 0, 0, time.UTC))
	if err := initial.Create(item); err != nil {
		t.Fatalf("initial.Create() error = %v", err)
	}

	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("store file not written at %q: %v", storePath, err)
	}

	loaded, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore(loaded) error = %v", err)
	}

	got, ok, err := loaded.GetByID(item.ID)
	if err != nil {
		t.Fatalf("loaded.GetByID() error = %v", err)
	}
	if !ok {
		t.Fatalf("loaded.GetByID() ok = false, want true")
	}

	if got.ID != item.ID ||
		got.Type != item.Type ||
		got.Domain != item.Domain ||
		got.InstallPath != item.InstallPath ||
		got.SitePath != item.SitePath ||
		got.URL != item.URL ||
		got.AdminURL != item.AdminURL ||
		got.DBName != item.DBName ||
		got.DBUser != item.DBUser ||
		got.Status != item.Status ||
		got.Message != item.Message ||
		!got.CreatedAt.Equal(item.CreatedAt) ||
		!got.UpdatedAt.Equal(item.UpdatedAt) {
		t.Fatalf("loaded item mismatch:\n got:  %+v\n want: %+v", got, item)
	}

	items, err := loaded.List()
	if err != nil {
		t.Fatalf("loaded.List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("loaded.List() len = %d, want 1", len(items))
	}
	if items[0].ID != item.ID {
		t.Fatalf("loaded.List()[0].ID = %q, want %q", items[0].ID, item.ID)
	}
}

func testInstallation(id, domain, installPath string, createdAt time.Time) Installation {
	return Installation{
		ID:          id,
		Type:        "wordpress",
		Domain:      domain,
		InstallPath: installPath,
		SitePath:    filepath.ToSlash(filepath.Join("/var/www/panelx/sites", domain, "public_html")),
		URL:         "http://" + domain,
		AdminURL:    "http://" + domain + "/wp-admin",
		DBName:      "db_" + id,
		DBUser:      "user_" + id,
		Status:      "active",
		Message:     "ok",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
}
