// Package main demonstrates a simple SSE client that connects to the
// Watchtower /v1/events endpoint and prints received events to stdout.
//
// Usage: go run main.go --addr localhost:8080 --token 12345
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
)

const readBufSize = 4096

// ErrUnexpectedStatus indicates the SSE endpoint returned a non-200 response.
var ErrUnexpectedStatus = errors.New("unexpected SSE endpoint status")

// EventStream represents the operations needed to watch events.
type EventStream interface {
	Events(ctx context.Context) error
}

// streamer implements EventStream by connecting to an SSE endpoint.
type streamer struct {
	url   string
	token string
}

// NewStreamer returns a configured EventStream.
func NewStreamer(url, token string) EventStream {
	return &streamer{url: url, token: token}
}

// Events reads from the SSE endpoint and prints payloads to stdout.
func (s *streamer) Events(ctx context.Context) error {
	resp, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = s.validateStatus(resp)
	if err != nil {
		return err
	}

	fmt.Printf("Connected (status %d). Waiting for events...\n", resp.StatusCode)

	return s.readStream(ctx, resp.Body)
}

// connect performs the HTTP request with optional bearer auth.
func (s *streamer) connect(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		s.url,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	client := &http.Client{Timeout: 0}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting: %w", err)
	}

	return resp, nil
}

// validateStatus returns ErrUnexpectedStatus if the response is not 200.
func (s *streamer) validateStatus(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	return fmt.Errorf(
		"%w: %d %s",
		ErrUnexpectedStatus, resp.StatusCode, strings.TrimSpace(string(body)),
	)
}

// readStream copies response body chunks to stdout until EOF or cancel.
func (s *streamer) readStream(ctx context.Context, body io.Reader) error {
	buf := make([]byte, readBufSize)

	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			fmt.Printf("%s", string(buf[:n]))
		}

		if readErr == nil {
			continue
		}

		if errors.Is(readErr, io.EOF) {
			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("stream cancelled: %w", ctx.Err())
		}

		return fmt.Errorf("reading stream: %w", readErr)
	}
}

// parseFlags returns the events endpoint URL and token from CLI flags.
func parseFlags() (string, string) {
	addr := flag.String(
		"addr",
		"http://localhost:3000/v1/events",
		"Watchtower events endpoint URL (e.g. http://localhost:3000/v1/events)",
	)
	token := flag.String(
		"token",
		"",
		"Events access token",
	)

	flag.Parse()

	return *addr, *token
}

// run coordinates signal handling and event streaming.
func run(stream EventStream) error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer stop()

	log.Printf("Waiting for events...")

	errCh := make(chan error, 1)
	go func() {
		errCh <- stream.Events(ctx)
	}()

	select {
	case <-ctx.Done():
		log.Println("Shutting down...")

		return nil
	case err := <-errCh:
		return err
	}
}

func main() {
	url, token := parseFlags()

	stream := NewStreamer(url, token)

	err := run(stream)
	if err != nil {
		log.Fatal(err)
	}
}
