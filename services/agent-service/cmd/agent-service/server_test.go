package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeDrainsInflightRequestOnShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	server := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			_, _ = io.WriteString(w, "complete")
		}),
	}
	defer func() { _ = server.Close() }()
	served := make(chan error, 1)
	go func() { served <- serve(ctx, server, listener) }()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+listener.Addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	response := make(chan string, 1)
	client := &http.Client{Timeout: 3 * time.Second}
	go func() {
		result, err := client.Do(request)
		if err != nil {
			response <- err.Error()
			return
		}
		defer func() { _ = result.Body.Close() }()
		body, err := io.ReadAll(result.Body)
		if err != nil {
			response <- err.Error()
			return
		}
		response <- string(body)
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("request did not reach server")
	}
	cancel()
	close(release)
	if body := <-response; body != "complete" {
		t.Fatalf("in-flight response lost during shutdown: %s", body)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not finish shutdown")
	}
}
