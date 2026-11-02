package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginFailureLimit = 5
	loginWindow       = 15 * time.Minute
	loginLockout      = 15 * time.Minute
	loginLimiterCap   = 4096
)

type loginAttempt struct {
	windowStart  time.Time
	failures     int
	blockedUntil time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string]loginAttempt)}
}

func (l *loginLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	entry, exists := l.attempts[key]
	if exists && now.Before(entry.blockedUntil) {
		return false, entry.blockedUntil.Sub(now)
	}
	if exists && entry.failures >= loginFailureLimit {
		delete(l.attempts, key)
	}
	return true, 0
}

func (l *loginLimiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	entry, exists := l.attempts[key]
	if !exists || !now.Before(entry.windowStart.Add(loginWindow)) {
		entry = loginAttempt{windowStart: now}
	}
	entry.failures++
	if entry.failures >= loginFailureLimit {
		entry.blockedUntil = now.Add(loginLockout)
	}
	if !exists && len(l.attempts) >= loginLimiterCap {
		l.evictEarliestExpiry()
	}
	l.attempts[key] = entry
}

func (l *loginLimiter) success(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}

func (l *loginLimiter) prune(now time.Time) {
	for key, entry := range l.attempts {
		if !now.Before(entry.blockedUntil) && !now.Before(entry.windowStart.Add(loginWindow)) {
			delete(l.attempts, key)
		}
	}
}

func (l *loginLimiter) evictEarliestExpiry() {
	var oldestKey string
	var oldestExpiry time.Time
	for key, entry := range l.attempts {
		expires := entry.windowStart.Add(loginWindow)
		if entry.blockedUntil.After(expires) {
			expires = entry.blockedUntil
		}
		if oldestKey == "" || expires.Before(oldestExpiry) {
			oldestKey, oldestExpiry = key, expires
		}
	}
	if oldestKey != "" {
		delete(l.attempts, oldestKey)
	}
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
