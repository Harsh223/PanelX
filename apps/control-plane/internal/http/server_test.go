package httpserver

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Harsh223/PanelX/apps/control-plane/internal/panelauth"
)

func TestAuthTokenMiddleware_AcceptsXPanelXToken(t *testing.T) {
	t.Parallel()

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	handler := authTokenMiddleware("test-token", nil, next)

	req := httptest.NewRequest(http.MethodGet, "/v1/installations", nil)
	req.Header.Set("X-PanelX-Token", "test-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatalf("next handler was not called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestAuthTokenMiddleware_AcceptsAuthorizationBearerToken(t *testing.T) {
	t.Parallel()

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := authTokenMiddleware("test-token", nil, next)

	req := httptest.NewRequest(http.MethodGet, "/v1/installations", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatalf("next handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestAuthTokenMiddleware_AcceptsValidSessionCookie(t *testing.T) {
	t.Parallel()

	authSvc, sessionToken := newAuthedSession(t)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := authTokenMiddleware("test-token", authSvc, next)

	req := httptest.NewRequest(http.MethodGet, "/v1/installations", nil)
	req.AddCookie(&http.Cookie{
		Name:  panelauth.SessionCookieName,
		Value: sessionToken,
	})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatalf("next handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestAuthTokenMiddleware_RejectsMissingTokenAndSession(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called for missing auth")
	})

	handler := authTokenMiddleware("test-token", nil, next)

	req := httptest.NewRequest(http.MethodGet, "/v1/installations", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want %q", got, "application/json")
	}
	const wantBody = `{"code":"unauthorized","message":"invalid or missing admin token/session"}`
	if strings.TrimSpace(rr.Body.String()) != wantBody {
		t.Fatalf("body = %q, want %q", strings.TrimSpace(rr.Body.String()), wantBody)
	}
}

func TestAuthTokenMiddleware_RejectsInvalidTokenAndInvalidSession(t *testing.T) {
	t.Parallel()

	authSvc, _ := newAuthedSession(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called for invalid auth")
	})

	handler := authTokenMiddleware("test-token", authSvc, next)

	req := httptest.NewRequest(http.MethodGet, "/v1/installations", nil)
	req.Header.Set("X-PanelX-Token", "wrong-token")
	req.AddCookie(&http.Cookie{
		Name:  panelauth.SessionCookieName,
		Value: "bad-session",
	})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	const wantBody = `{"code":"unauthorized","message":"invalid or missing admin token/session"}`
	if strings.TrimSpace(rr.Body.String()) != wantBody {
		t.Fatalf("body = %q, want %q", strings.TrimSpace(rr.Body.String()), wantBody)
	}
}

func newAuthedSession(t *testing.T) (*panelauth.Service, string) {
	t.Helper()

	authPath := filepath.Join(t.TempDir(), "panel-auth.json")
	svc, err := panelauth.NewService(authPath, time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := svc.Setup("bootstrap-token", "bootstrap-token", "admin", "admin@example.com", "supersecure123"); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	session, err := svc.Login("admin", "supersecure123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	return svc, session.Token
}
