package clashapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/holi/duorelay/internal/clashapi"
)

func TestTrafficStreamRejectsInvalidSample(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"up":-1,"down":2}`)); err != nil {
			t.Errorf("write sample: %v", err)
		}
	}))
	defer server.Close()

	client := clashapi.NewClient(server.URL, "", clashapi.KernelSingBox)
	defer client.Close()
	_, err := client.ReadTraffic(context.Background())
	if err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("ReadTraffic error = %v, want invalid non-negative sample", err)
	}
}

func TestTrafficStreamContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	client := clashapi.NewClient(server.URL, "", clashapi.KernelSingBox)
	defer client.Close()
	stream, err := client.OpenTraffic(context.Background())
	if err != nil {
		t.Fatalf("OpenTraffic: %v", err)
	}
	defer stream.Close()

	ctx, cancel := context.WithCancel(context.Background())
	readDone := make(chan error, 1)
	go func() {
		_, readErr := stream.Read(ctx)
		readDone <- readErr
	}()
	cancel()

	select {
	case readErr := <-readDone:
		if !errors.Is(readErr, context.Canceled) {
			t.Fatalf("Read error = %v, want context cancellation", readErr)
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not stop after context cancellation")
	}
}
