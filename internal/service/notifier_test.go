package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubInviteNotifier struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (s *stubInviteNotifier) NotifyInvite(ctx context.Context, toEmail string, teamID uint64, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.err
}

func (s *stubInviteNotifier) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *stubInviteNotifier) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestHTTPInviteNotifier(t *testing.T) {
	t.Parallel()

	noop := NewHTTPInviteNotifier("", time.Second)
	if err := noop.NotifyInvite(context.Background(), "user@example.com", 1, "code"); err != nil {
		t.Fatalf("noop notifier should not fail: %v", err)
	}

	brokenURLNotifier := NewHTTPInviteNotifier("://bad-url", time.Second)
	if err := brokenURLNotifier.NotifyInvite(context.Background(), "user@example.com", 1, "code"); err == nil {
		t.Fatal("expected notifier error for invalid URL")
	}

	failing := &HTTPInviteNotifier{
		url: "http://notify.local",
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("oops")),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}
	if err := failing.NotifyInvite(context.Background(), "user@example.com", 1, "code"); err == nil {
		t.Fatal("expected notifier status error")
	}

	success := &HTTPInviteNotifier{
		url: "http://notify.local",
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}
	if err := success.NotifyInvite(context.Background(), "user@example.com", 1, "code"); err != nil {
		t.Fatalf("expected success notifier call, err=%v", err)
	}
}

func TestCircuitBreakerNotifier(t *testing.T) {
	t.Parallel()

	stub := &stubInviteNotifier{err: errors.New("upstream down")}
	cb := NewCircuitBreakerNotifier(stub, 2, 50*time.Millisecond)

	if err := cb.NotifyInvite(context.Background(), "a@b.c", 1, "code"); err == nil {
		t.Fatal("expected first call to fail")
	}
	if err := cb.NotifyInvite(context.Background(), "a@b.c", 1, "code"); err == nil {
		t.Fatal("expected second call to fail and open circuit")
	}
	if err := cb.NotifyInvite(context.Background(), "a@b.c", 1, "code"); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("third call error = %v, want %v", err, ErrCircuitOpen)
	}
	if stub.Calls() != 2 {
		t.Fatalf("underlying notifier calls = %d, want 2", stub.Calls())
	}

	time.Sleep(60 * time.Millisecond)
	stub.setErr(nil)
	if err := cb.NotifyInvite(context.Background(), "a@b.c", 1, "code"); err != nil {
		t.Fatalf("call after reset should pass, err=%v", err)
	}
	if stub.Calls() != 3 {
		t.Fatalf("underlying notifier calls = %d, want 3", stub.Calls())
	}
}

func TestCircuitBreakerNotifierDefaults(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakerNotifier(nil, 0, 0)
	if cb.threshold != 3 {
		t.Fatalf("default threshold = %d, want 3", cb.threshold)
	}
	if cb.resetTimeout != 30*time.Second {
		t.Fatalf("default reset timeout = %v, want %v", cb.resetTimeout, 30*time.Second)
	}
	if err := cb.NotifyInvite(context.Background(), "user@example.com", 1, "code"); err != nil {
		t.Fatalf("default notifier should pass, err=%v", err)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
