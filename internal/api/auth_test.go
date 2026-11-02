package api

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/auth"
)

func TestAuthenticationLifecycle(t *testing.T) {
	s := apiForTest(t)
	s.Auth = auth.New(s.Config.Store)
	call(t, s, http.MethodGet, "/api/auth/status", "", http.StatusOK)
	call(t, s, http.MethodGet, "/api/config", "", http.StatusPreconditionRequired)
	call(t, s, http.MethodPost, "/api/auth/register", `{"username":"admin","password":"correct-horse-battery"}`, http.StatusCreated)
	call(t, s, http.MethodGet, "/api/config", "", http.StatusUnauthorized)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"correct-horse-battery"}`))
	req.Header.Set("Content-Type", "application/json")
	login := httptest.NewRecorder()
	s.ServeHTTP(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "duorelay_session" || cookies[0].Value == "" {
		t.Fatalf("missing session cookie: %#v", cookies)
	}
	if cookies[0].Secure {
		t.Fatal("local HTTP login unexpectedly marked its cookie Secure")
	}

	protected := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	protected.AddCookie(cookies[0])
	protectedResponse := httptest.NewRecorder()
	s.ServeHTTP(protectedResponse, protected)
	if protectedResponse.Code != http.StatusOK {
		t.Fatalf("authenticated config status = %d: %s", protectedResponse.Code, protectedResponse.Body.String())
	}

	logout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logout.AddCookie(cookies[0])
	s.ServeHTTP(httptest.NewRecorder(), logout)
	if _, err := s.Auth.Authenticate(context.Background(), protected); err == nil {
		t.Fatal("session remained valid after logout")
	}
}

func TestLoginRateLimit(t *testing.T) {
	s := apiForTest(t)
	s.Auth = auth.New(s.Config.Store)
	call(t, s, http.MethodPost, "/api/auth/register", "{\"username\":\"admin\",\"password\":\"correct-horse-battery\"}", http.StatusCreated)
	for i := 0; i < 5; i++ {
		call(t, s, http.MethodPost, "/api/auth/login", "{\"username\":\"admin\",\"password\":\"wrong-password\"}", http.StatusUnauthorized)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{\"username\":\"admin\",\"password\":\"wrong-password\"}"))
	req.RemoteAddr = "192.0.2.1:1234"
	response := httptest.NewRecorder()
	s.ServeHTTP(response, req)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
		t.Fatalf("rate limited response = %d, Retry-After %q", response.Code, response.Header().Get("Retry-After"))
	}
}

func TestSecureSessionCookie(t *testing.T) {
	for _, tc := range []struct {
		name       string
		cookieFlag bool
		tls        bool
	}{
		{name: "native TLS", tls: true},
		{name: "TLS reverse proxy", cookieFlag: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := apiForTest(t)
			s.Auth = auth.New(s.Config.Store)
			s.CookieSecure = tc.cookieFlag
			call(t, s, http.MethodPost, "/api/auth/register", "{\"username\":\"admin\",\"password\":\"correct-horse-battery\"}", http.StatusCreated)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{\"username\":\"admin\",\"password\":\"correct-horse-battery\"}"))
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			response := httptest.NewRecorder()
			s.ServeHTTP(response, req)
			if response.Code != http.StatusOK {
				t.Fatalf("login status = %d: %s", response.Code, response.Body.String())
			}
			cookies := response.Result().Cookies()
			if len(cookies) != 1 || !cookies[0].Secure {
				t.Fatalf("secure session cookie missing: %#v", cookies)
			}
		})
	}
}
