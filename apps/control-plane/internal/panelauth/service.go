package panelauth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// SessionCookieName is the HTTP cookie key expected by panel session middleware.
	SessionCookieName = "panelx_session"

	defaultSessionTTL = 24 * time.Hour
)

var (
	ErrAlreadyConfigured = errors.New("panel admin is already configured")
	ErrBootstrapInvalid  = errors.New("invalid bootstrap token")
	ErrNotConfigured     = errors.New("panel admin is not configured")
	ErrInvalidUsername   = errors.New("username must be 3-32 chars and contain only letters, numbers, dot, underscore, dash")
	ErrInvalidEmail      = errors.New("email is required and must look valid")
	ErrInvalidPassword   = errors.New("password must be at least 8 characters")
	ErrInvalidLogin      = errors.New("invalid username or password")
	ErrInvalidSession    = errors.New("invalid or expired session")
)

var usernameRegexp = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,32}$`)

type persistedState struct {
	Admin *adminRecord `json:"admin,omitempty"`
}

type adminRecord struct {
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AdminProfile is safe to return over API.
type AdminProfile struct {
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Session represents an authenticated web session.
type Session struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Service provides persistent panel-admin bootstrap and cookie-session management.
type Service struct {
	path       string
	sessionTTL time.Duration

	mu       sync.Mutex
	admin    *adminRecord
	sessions map[string]Session
}

// NewService creates a panel auth service and loads persisted admin state if present.
func NewService(path string, sessionTTL time.Duration) (*Service, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("panel auth path is required")
	}
	if sessionTTL <= 0 {
		sessionTTL = defaultSessionTTL
	}

	s := &Service{
		path:       path,
		sessionTTL: sessionTTL,
		sessions:   map[string]Session{},
	}

	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// IsConfigured reports whether panel bootstrap has been completed.
func (s *Service) IsConfigured() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.admin != nil
}

// Profile returns admin profile details if configured.
func (s *Service) Profile() (AdminProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.admin == nil {
		return AdminProfile{}, false
	}
	return s.profileUnsafe(), true
}

// Setup initializes admin credentials exactly once.
// It requires a valid bootstrap token (the installer-provided API token).
func (s *Service) Setup(expectedBootstrapToken, providedBootstrapToken, username, email, password string) (AdminProfile, error) {
	if strings.TrimSpace(expectedBootstrapToken) == "" || strings.TrimSpace(providedBootstrapToken) == "" {
		return AdminProfile{}, ErrBootstrapInvalid
	}
	if strings.TrimSpace(expectedBootstrapToken) != strings.TrimSpace(providedBootstrapToken) {
		return AdminProfile{}, ErrBootstrapInvalid
	}

	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)

	if !usernameRegexp.MatchString(username) {
		return AdminProfile{}, ErrInvalidUsername
	}
	if !looksLikeEmail(email) {
		return AdminProfile{}, ErrInvalidEmail
	}
	if len(password) < 8 {
		return AdminProfile{}, ErrInvalidPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AdminProfile{}, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.admin != nil {
		return AdminProfile{}, ErrAlreadyConfigured
	}

	s.admin = &adminRecord{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.persistLocked(); err != nil {
		s.admin = nil
		return AdminProfile{}, err
	}

	return s.profileUnsafe(), nil
}

// Login validates credentials and returns a new session token.
func (s *Service) Login(username, password string) (Session, error) {
	username = strings.TrimSpace(username)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.admin == nil {
		return Session{}, ErrNotConfigured
	}
	if !strings.EqualFold(s.admin.Username, username) {
		return Session{}, ErrInvalidLogin
	}

	if err := bcrypt.CompareHashAndPassword([]byte(s.admin.PasswordHash), []byte(password)); err != nil {
		return Session{}, ErrInvalidLogin
	}

	s.pruneExpiredLocked()

	token, err := randomToken(32)
	if err != nil {
		return Session{}, fmt.Errorf("generate session token: %w", err)
	}

	session := Session{
		Token:     token,
		Username:  s.admin.Username,
		ExpiresAt: time.Now().UTC().Add(s.sessionTTL),
	}
	s.sessions[token] = session

	return session, nil
}

// ValidateSession returns the session if token exists and is not expired.
func (s *Service) ValidateSession(token string) (Session, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, ErrInvalidSession
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneExpiredLocked()

	session, ok := s.sessions[token]
	if !ok || session.ExpiresAt.Before(time.Now().UTC()) {
		return Session{}, ErrInvalidSession
	}
	return session, nil
}

// Logout invalidates one session token.
func (s *Service) Logout(token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func (s *Service) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read panel auth file: %w", err)
	}
	if len(raw) == 0 {
		return nil
	}

	var state persistedState
	if err := json.Unmarshal(raw, &state); err != nil {
		return fmt.Errorf("decode panel auth file: %w", err)
	}

	if state.Admin != nil {
		s.admin = state.Admin
	}

	return nil
}

func (s *Service) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return fmt.Errorf("create panel auth dir: %w", err)
	}

	state := persistedState{
		Admin: s.admin,
	}

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode panel auth state: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o640); err != nil {
		return fmt.Errorf("write panel auth temp: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace panel auth file: %w", err)
	}

	return nil
}

func (s *Service) pruneExpiredLocked() {
	now := time.Now().UTC()
	for token, session := range s.sessions {
		if session.ExpiresAt.Before(now) {
			delete(s.sessions, token)
		}
	}
}

func (s *Service) profileUnsafe() AdminProfile {
	return AdminProfile{
		Username:  s.admin.Username,
		Email:     s.admin.Email,
		CreatedAt: s.admin.CreatedAt,
		UpdatedAt: s.admin.UpdatedAt,
	}
}

func randomToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func looksLikeEmail(value string) bool {
	if value == "" {
		return false
	}
	parts := strings.Split(value, "@")
	if len(parts) != 2 {
		return false
	}
	return parts[0] != "" && strings.Contains(parts[1], ".")
}
