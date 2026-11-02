package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const (
	CookieName = "duorelay_session"
	SessionTTL = 7 * 24 * time.Hour
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$`)

var (
	ErrNotConfigured = errors.New("authentication is not configured")
	ErrInvalid       = errors.New("invalid username or password")
	ErrUnauthorized  = errors.New("authentication required")
)

type Service struct {
	Store *store.Store
}

type Status struct {
	Configured    bool   `json:"configured"`
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
}

func New(s *store.Store) *Service { return &Service{Store: s} }

func (s *Service) Status(ctx context.Context, r *http.Request) (Status, error) {
	count, err := s.Store.AuthUserCount(ctx)
	if err != nil {
		return Status{}, err
	}
	status := Status{Configured: count > 0}
	if !status.Configured {
		return status, nil
	}
	username, err := s.Authenticate(ctx, r)
	if err == nil {
		status.Authenticated = true
		status.Username = username
	}
	return status, nil
}

func (s *Service) Register(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("%w: username must be 3-64 characters using letters, numbers, dot, dash or underscore", store.ErrInvalid)
	}
	if len(password) < 8 || len(password) > 128 {
		return fmt.Errorf("%w: password must be 8-128 characters", store.ErrInvalid)
	}
	count, err := s.Store.AuthUserCount(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: account already configured", store.ErrConflict)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.Store.CreateAuthUser(ctx, uuid.NewString(), username, string(hash))
}

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)
	hash, err := s.Store.AuthPasswordHash(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", ErrInvalid
		}
		return "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", ErrInvalid
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expiresAt := time.Now().UTC().Add(SessionTTL)
	if err := s.Store.CreateAuthSession(ctx, hashToken(token), username, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) Authenticate(ctx context.Context, r *http.Request) (string, error) {
	token := ""
	if cookie, err := r.Cookie(CookieName); err == nil {
		token = cookie.Value
	}
	if token == "" {
		const prefix = "Bearer "
		if value := r.Header.Get("Authorization"); strings.HasPrefix(value, prefix) {
			token = strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
	}
	if token == "" {
		return "", ErrUnauthorized
	}
	username, err := s.Store.AuthSessionUser(ctx, hashToken(token), time.Now().UTC())
	if err != nil {
		return "", ErrUnauthorized
	}
	return username, nil
}

func (s *Service) Logout(ctx context.Context, r *http.Request) error {
	token := ""
	if cookie, err := r.Cookie(CookieName); err == nil {
		token = cookie.Value
	}
	if token == "" {
		const prefix = "Bearer "
		if value := r.Header.Get("Authorization"); strings.HasPrefix(value, prefix) {
			token = strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
	}
	if token == "" {
		return nil
	}
	return s.Store.DeleteAuthSession(ctx, hashToken(token))
}

func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(SessionTTL / time.Second),
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
