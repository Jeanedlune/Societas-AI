package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()
	NewHandler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status body = %q, want ok", body.Status)
	}
}

func TestHealthEndpointRejectsOtherMethods(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	res := httptest.NewRecorder()
	NewHandler().ServeHTTP(res, req)

	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusMethodNotAllowed)
	}
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, addr, &discardWriter{})
	}()

	client := &http.Client{}
	if err := waitForHealth(client, "http://"+addr+"/health"); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run returned error after cancellation: %v", err)
	}
	if _, err := client.Get("http://" + addr + "/health"); err == nil {
		t.Fatal("health endpoint still accepts connections after shutdown")
	}
}

func waitForHealth(client *http.Client, url string) error {
	for range 100 {
		res, err := client.Get(url)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
	return context.DeadlineExceeded
}

type discardWriter struct{}

func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }
