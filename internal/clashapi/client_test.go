package clashapi

import (
	"context"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	// This is a placeholder test
	// Real tests will be added in P0-04 when we have actual kernels running

	client := NewClient("http://127.0.0.1:9090", "")
	if client == nil {
		t.Fatal("Failed to create client")
	}

	if client.baseURL != "http://127.0.0.1:9090" {
		t.Errorf("Expected baseURL http://127.0.0.1:9090, got %s", client.baseURL)
	}
}

func TestClientTimeout(t *testing.T) {
	client := NewClient("http://127.0.0.1:9999", "")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// This should fail quickly since nothing is listening
	_, err := client.GetVersion(ctx)
	if err == nil {
		t.Error("Expected error when connecting to non-existent server")
	}
}
