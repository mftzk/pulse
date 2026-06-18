package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbe_2xxIsUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := Do(context.Background(), Target{URL: srv.URL, TimeoutMs: 2000})
	if !res.Up {
		t.Fatalf("expected up, got down: %s", res.Error)
	}
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
}

func TestProbe_5xxIsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	res := Do(context.Background(), Target{URL: srv.URL, TimeoutMs: 2000})
	if res.Up {
		t.Fatal("expected down for 500")
	}
	if res.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", res.StatusCode)
	}
}

func TestProbe_ExpectedStatusMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
	}))
	defer srv.Close()

	// when we explicitly expect 418, it should count as up
	res := Do(context.Background(), Target{URL: srv.URL, ExpectedStatus: 418, TimeoutMs: 2000})
	if !res.Up {
		t.Fatalf("expected up when status matches expected, got: %s", res.Error)
	}
}

func TestProbe_ExpectedClassMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // 404
	}))
	defer srv.Close()

	// expecting the 4xx class -> a 404 should count as up
	up := Do(context.Background(), Target{URL: srv.URL, ExpectedStatus: 4, TimeoutMs: 2000})
	if !up.Up {
		t.Fatalf("expected up for 404 with 4xx class, got: %s", up.Error)
	}
	// expecting the 5xx class -> a 404 should count as down
	down := Do(context.Background(), Target{URL: srv.URL, ExpectedStatus: 5, TimeoutMs: 2000})
	if down.Up {
		t.Fatal("expected down for 404 with 5xx class")
	}
}

func TestProbe_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := Do(context.Background(), Target{URL: srv.URL, TimeoutMs: 50})
	if res.Up {
		t.Fatal("expected down due to timeout")
	}
	if res.Error == "" {
		t.Fatal("expected an error message on timeout")
	}
}

func TestProbe_ConnectionRefused(t *testing.T) {
	// nothing is listening on this port
	res := Do(context.Background(), Target{URL: "http://127.0.0.1:1", TimeoutMs: 500})
	if res.Up {
		t.Fatal("expected down for connection refused")
	}
}
