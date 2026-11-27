package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ilinmu/vita/internal/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLANFirstAndPublicFallback(t *testing.T) {
	var lanDown atomic.Bool
	var lanCalls, pubCalls atomic.Int32

	c := NewClient("http://lan.vita.local:8420", "https://vita.example.com")
	c.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "lan.vita.local:8420":
			if lanDown.Load() {
				return nil, errors.New("dial tcp: connection refused")
			}
			lanCalls.Add(1)
			rec := httptest.NewRecorder()
			_ = json.NewEncoder(rec).Encode(model.AuthResponse{
				Token: "lan-token",
				User:  model.UserInfo{ID: "u1", Username: "alice"},
			})
			res := rec.Result()
			res.Body = io.NopCloser(rec.Body)
			return res, nil
		case "vita.example.com":
			pubCalls.Add(1)
			rec := httptest.NewRecorder()
			_ = json.NewEncoder(rec).Encode(model.AuthResponse{
				Token: "pub-token",
				User:  model.UserInfo{ID: "u1", Username: "alice"},
			})
			res := rec.Result()
			res.Body = io.NopCloser(rec.Body)
			return res, nil
		default:
			return nil, errors.New("unexpected host: " + r.URL.Host)
		}
	}))

	// 1. Both reachable -> LAN wins
	resp, err := c.Login(context.Background(), "alice", "secret")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.Token != "lan-token" || c.ActiveEndpoint() != "lan" || lanCalls.Load() != 1 || pubCalls.Load() != 0 {
		t.Fatalf("expected LAN priority, got token=%s ep=%s lan=%d pub=%d", resp.Token, c.ActiveEndpoint(), lanCalls.Load(), pubCalls.Load())
	}

	// 2. LAN down -> falls back to Public
	lanDown.Store(true)
	resp, err = c.Login(context.Background(), "alice", "secret")
	if err != nil {
		t.Fatalf("Fallback Login failed: %v", err)
	}
	if resp.Token != "pub-token" || c.ActiveEndpoint() != "public" || pubCalls.Load() != 1 {
		t.Fatalf("expected Public fallback, got token=%s ep=%s pub=%d", resp.Token, c.ActiveEndpoint(), pubCalls.Load())
	}
}
