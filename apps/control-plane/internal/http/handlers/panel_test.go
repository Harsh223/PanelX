package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Harsh223/PanelX/apps/control-plane/internal/installations"
	"github.com/Harsh223/PanelX/apps/control-plane/internal/systemstatus"
)

type installationsStoreStub struct {
	listItems []installations.Installation
	listErr   error
}

func (s *installationsStoreStub) Create(installation installations.Installation) error {
	return nil
}

func (s *installationsStoreStub) List() ([]installations.Installation, error) {
	return s.listItems, s.listErr
}

func (s *installationsStoreStub) GetByID(id string) (installations.Installation, bool, error) {
	return installations.Installation{}, false, nil
}

func TestPanelHandlerListInstallationsMethodGuard(t *testing.T) {
	t.Parallel()

	handler := NewPanelHandler(nil, nil, nil, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/v1/installations", nil)
	rr := httptest.NewRecorder()

	handler.ListInstallations(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestPanelHandlerListInstallationsSuccess(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	store := &installationsStoreStub{
		listItems: []installations.Installation{
			{
				ID:          "inst-1",
				Type:        "wordpress",
				Domain:      "example.com",
				InstallPath: "/",
				SitePath:    "/var/www/panelx/sites/example.com/public_html",
				URL:         "http://example.com",
				AdminURL:    "http://example.com/wp-admin",
				DBName:      "wp_example",
				DBUser:      "wp_user",
				Status:      "active",
				Message:     "WordPress installed successfully",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}

	installSvc := installations.NewService(store, nil)
	handler := NewPanelHandler(installSvc, nil, nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/v1/installations", nil)
	rr := httptest.NewRecorder()

	handler.ListInstallations(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var payload struct {
		Items []installations.Installation `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(payload.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(payload.Items))
	}
	if payload.Items[0].Domain != "example.com" {
		t.Fatalf("domain = %q, want %q", payload.Items[0].Domain, "example.com")
	}
}

func TestPanelHandlerListInstallationsStoreError(t *testing.T) {
	t.Parallel()

	store := &installationsStoreStub{
		listErr: errors.New("list failed"),
	}
	installSvc := installations.NewService(store, nil)
	handler := NewPanelHandler(installSvc, nil, nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/v1/installations", nil)
	rr := httptest.NewRecorder()

	handler.ListInstallations(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestPanelHandlerInstallWordPressMethodGuard(t *testing.T) {
	t.Parallel()

	handler := NewPanelHandler(nil, nil, nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/v1/wordpress/install", nil)
	rr := httptest.NewRecorder()

	handler.InstallWordPress(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestPanelHandlerVPSStatusMethodGuard(t *testing.T) {
	t.Parallel()

	handler := NewPanelHandler(nil, nil, nil, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/v1/system/status", nil)
	rr := httptest.NewRecorder()

	handler.VPSStatus(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestPanelHandlerVPSStatusUnavailableWhenServiceMissing(t *testing.T) {
	t.Parallel()

	handler := NewPanelHandler(nil, nil, nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/v1/system/status", nil)
	rr := httptest.NewRecorder()

	handler.VPSStatus(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rr.Body.String(), "system_status_unavailable") {
		t.Fatalf("response body missing expected code, got %q", rr.Body.String())
	}
}

func TestPanelHandlerVPSStatusSuccess(t *testing.T) {
	t.Parallel()

	statusSvc := systemstatus.NewService(t.TempDir(), "")
	handler := NewPanelHandler(nil, nil, statusSvc, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/v1/system/status", nil)
	rr := httptest.NewRecorder()

	handler.VPSStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var payload struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
		CPU       struct {
			Cores int `json:"cores"`
		} `json:"cpu"`
		WordPress struct {
			SitesRoot string `json:"sitesRoot"`
		} `json:"wordpress"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v body=%q", err, rr.Body.String())
	}

	if payload.Status == "" {
		t.Fatalf("status should not be empty")
	}
	if payload.Timestamp == "" {
		t.Fatalf("timestamp should not be empty")
	}
	if payload.CPU.Cores < 1 {
		t.Fatalf("cpu cores = %d, want >= 1", payload.CPU.Cores)
	}
	if payload.WordPress.SitesRoot == "" {
		t.Fatalf("wordpress.sitesRoot should not be empty")
	}
}
