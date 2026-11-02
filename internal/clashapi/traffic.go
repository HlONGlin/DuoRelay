package clashapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// TrafficStream reads the continuous traffic samples exposed by /traffic.
// The stream is safe to close while a read is blocked.
type TrafficStream struct {
	conn      *websocket.Conn
	closeOnce sync.Once
}

// OpenTraffic opens the kernel's /traffic WebSocket endpoint.
func (c *Client) OpenTraffic(ctx context.Context) (*TrafficStream, error) {
	endpoint, err := trafficWebSocketURL(c.baseURL)
	if err != nil {
		return nil, err
	}

	headers := http.Header{}
	if c.secret != "" {
		headers.Set("Authorization", "Bearer "+c.secret)
	}
	conn, response, err := (&websocket.Dialer{}).DialContext(ctx, endpoint, headers)
	if err != nil {
		if response != nil && response.Body != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			return nil, fmt.Errorf("open traffic stream: %s", response.Status)
		}
		return nil, fmt.Errorf("open traffic stream: %w", err)
	}
	return &TrafficStream{conn: conn}, nil
}

// ReadTraffic opens /traffic, reads one sample, and closes the stream.
func (c *Client) ReadTraffic(ctx context.Context) (*TrafficData, error) {
	stream, err := c.OpenTraffic(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	return stream.Read(ctx)
}

// Read reads the next traffic sample from the stream.
func (s *TrafficStream) Read(ctx context.Context) (*TrafficData, error) {
	if s == nil || s.conn == nil {
		return nil, fmt.Errorf("traffic stream is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	readDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = s.Close()
		case <-readDone:
		}
	}()
	_, payload, err := s.conn.ReadMessage()
	close(readDone)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("read traffic sample: %w", err)
	}

	var sample TrafficData
	if err := json.Unmarshal(payload, &sample); err != nil {
		return nil, fmt.Errorf("decode traffic sample: %w", err)
	}
	if sample.Up < 0 || sample.Down < 0 {
		return nil, fmt.Errorf("decode traffic sample: values must be non-negative")
	}
	return &sample, nil
}

// Close closes the traffic stream. It is safe to call more than once.
func (s *TrafficStream) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() { err = s.conn.Close() })
	return err
}

func trafficWebSocketURL(baseURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		if err == nil {
			err = fmt.Errorf("missing scheme or host")
		}
		return "", fmt.Errorf("invalid controller URL: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("invalid controller URL: unsupported scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/traffic"
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
