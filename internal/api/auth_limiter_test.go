package api

import (
	"fmt"
	"testing"
	"time"
)

func TestLoginLimiterBlocksAndExpires(t *testing.T) {
	limiter := newLoginLimiter()
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	for i := 0; i < loginFailureLimit; i++ {
		limiter.fail("192.0.2.1", now)
	}
	if allowed, wait := limiter.allow("192.0.2.1", now); allowed || wait <= 0 {
		t.Fatalf("locked account allowed=%v wait=%v", allowed, wait)
	}
	if allowed, _ := limiter.allow("192.0.2.2", now); !allowed {
		t.Fatal("rate limit leaked to a different client")
	}
	if allowed, _ := limiter.allow("192.0.2.1", now.Add(loginLockout)); !allowed {
		t.Fatal("client remained blocked after lockout expiry")
	}
}

func TestLoginLimiterCapacityDoesNotBlockNewClients(t *testing.T) {
	limiter := newLoginLimiter()
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	for i := 0; i < loginLimiterCap; i++ {
		limiter.fail(fmt.Sprintf("192.0.%d.%d", i/256, i%256), now)
	}
	if allowed, _ := limiter.allow("198.51.100.1", now); !allowed {
		t.Fatal("a new client was denied because the limiter table is full")
	}
	for i := 0; i < loginFailureLimit; i++ {
		limiter.fail("198.51.100.1", now)
	}
	if allowed, wait := limiter.allow("198.51.100.1", now); allowed || wait <= 0 {
		t.Fatalf("new client was not tracked at capacity: allowed=%v wait=%v", allowed, wait)
	}
	if got := len(limiter.attempts); got != loginLimiterCap {
		t.Fatalf("limiter retained %d entries, want cap %d", got, loginLimiterCap)
	}
}
