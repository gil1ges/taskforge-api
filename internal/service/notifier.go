package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("notifier circuit open")

type InviteNotifier interface {
	NotifyInvite(ctx context.Context, toEmail string, teamID uint64, code string) error
}

type NoopInviteNotifier struct{}

func (NoopInviteNotifier) NotifyInvite(ctx context.Context, toEmail string, teamID uint64, code string) error {
	return nil
}

type HTTPInviteNotifier struct {
	client *http.Client
	url    string
}

func NewHTTPInviteNotifier(url string, timeout time.Duration) InviteNotifier {
	if strings.TrimSpace(url) == "" {
		return NoopInviteNotifier{}
	}
	return &HTTPInviteNotifier{
		client: &http.Client{Timeout: timeout},
		url:    strings.TrimSpace(url),
	}
}

func (n *HTTPInviteNotifier) NotifyInvite(ctx context.Context, toEmail string, teamID uint64, code string) error {
	payload := map[string]any{
		"email":   toEmail,
		"team_id": teamID,
		"code":    code,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("notifier status %d", resp.StatusCode)
	}
	return nil
}

type CircuitBreakerNotifier struct {
	next InviteNotifier

	threshold    int
	resetTimeout time.Duration

	mu        sync.Mutex
	failures  int
	openUntil time.Time
}

func NewCircuitBreakerNotifier(next InviteNotifier, threshold int, resetTimeout time.Duration) *CircuitBreakerNotifier {
	if next == nil {
		next = NoopInviteNotifier{}
	}
	if threshold <= 0 {
		threshold = 3
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	return &CircuitBreakerNotifier{
		next:         next,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (c *CircuitBreakerNotifier) NotifyInvite(ctx context.Context, toEmail string, teamID uint64, code string) error {
	now := time.Now()

	c.mu.Lock()
	if now.Before(c.openUntil) {
		c.mu.Unlock()
		return ErrCircuitOpen
	}
	c.mu.Unlock()

	err := c.next.NotifyInvite(ctx, toEmail, teamID, code)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err == nil {
		c.failures = 0
		c.openUntil = time.Time{}
		return nil
	}

	c.failures++
	if c.failures >= c.threshold {
		c.openUntil = now.Add(c.resetTimeout)
	}
	return err
}
