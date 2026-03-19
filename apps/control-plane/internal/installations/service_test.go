package installations

import (
	"errors"
	"testing"
	"time"
)

type stubStore struct {
	listItems []Installation
	listErr   error

	getItem  Installation
	getFound bool
	getErr   error
	lastGet  string
}

func (s *stubStore) Create(installation Installation) error {
	return nil
}

func (s *stubStore) List() ([]Installation, error) {
	return s.listItems, s.listErr
}

func (s *stubStore) GetByID(id string) (Installation, bool, error) {
	s.lastGet = id
	return s.getItem, s.getFound, s.getErr
}

func TestServiceListReturnsStoreItems(t *testing.T) {
	now := time.Now().UTC()
	expected := []Installation{
		{ID: "inst-1", Domain: "example.com", CreatedAt: now},
		{ID: "inst-2", Domain: "blog.example.com", CreatedAt: now},
	}
	store := &stubStore{listItems: expected}
	svc := NewService(store, nil)

	got, err := svc.List()
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(got) != len(expected) {
		t.Fatalf("List() len = %d, want %d", len(got), len(expected))
	}
	for i := range expected {
		if got[i].ID != expected[i].ID || got[i].Domain != expected[i].Domain {
			t.Fatalf("List()[%d] = %+v, want %+v", i, got[i], expected[i])
		}
	}
}

func TestServiceListPropagatesStoreError(t *testing.T) {
	expectedErr := errors.New("store list failure")
	store := &stubStore{listErr: expectedErr}
	svc := NewService(store, nil)

	_, err := svc.List()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("List() error = %v, want %v", err, expectedErr)
	}
}

func TestServiceGetTrimsIDAndReturnsStoreResult(t *testing.T) {
	expected := Installation{ID: "inst-123", Domain: "example.com"}
	store := &stubStore{
		getItem:  expected,
		getFound: true,
	}
	svc := NewService(store, nil)

	got, found, err := svc.Get("  inst-123  ")
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if !found {
		t.Fatalf("Get() found = false, want true")
	}
	if store.lastGet != "inst-123" {
		t.Fatalf("Get() passed id = %q, want %q", store.lastGet, "inst-123")
	}
	if got.ID != expected.ID || got.Domain != expected.Domain {
		t.Fatalf("Get() = %+v, want %+v", got, expected)
	}
}

func TestServiceGetPropagatesStoreError(t *testing.T) {
	expectedErr := errors.New("store get failure")
	store := &stubStore{
		getErr: expectedErr,
	}
	svc := NewService(store, nil)

	_, _, err := svc.Get("inst-err")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Get() error = %v, want %v", err, expectedErr)
	}
}
