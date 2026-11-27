// Package syncer handles HTTP authentication, family management,
// LAN-first / public-fallback endpoint routing, and incremental bidirectional sync.
package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
)

// APIError represents an HTTP error returned by the Vita sync server.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Message)
}

// Client communicates with a self-hosted Vita sync server.
type Client struct {
	mu             sync.RWMutex
	lanURL         string
	publicURL      string
	activeEndpoint string // "lan" | "public" | "offline"
	transport      http.RoundTripper
}

// NewClient constructs a sync client with optional LAN and Public base URLs.
func NewClient(lanURL, publicURL string) *Client {
	return &Client{
		lanURL:         strings.TrimRight(strings.TrimSpace(lanURL), "/"),
		publicURL:      strings.TrimRight(strings.TrimSpace(publicURL), "/"),
		activeEndpoint: "offline",
	}
}

// SetTransport sets a custom HTTP RoundTripper (primarily for in-memory testing).
func (c *Client) SetTransport(rt http.RoundTripper) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transport = rt
}

// SetEndpoints updates the LAN and Public base URLs.
func (c *Client) SetEndpoints(lanURL, publicURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lanURL = strings.TrimRight(strings.TrimSpace(lanURL), "/")
	c.publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
}

// ActiveEndpoint returns which endpoint was most recently used ("lan", "public", or "offline").
func (c *Client) ActiveEndpoint() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeEndpoint
}

func (c *Client) doJSON(ctx context.Context, method, path, token, familyID string, reqBody, respBody any) error {
	c.mu.RLock()
	lan, pub := c.lanURL, c.publicURL
	c.mu.RUnlock()

	type target struct {
		name    string
		base    string
		timeout time.Duration
	}
	var targets []target
	if lan != "" {
		targets = append(targets, target{name: "lan", base: lan, timeout: 1500 * time.Millisecond})
	}
	if pub != "" {
		targets = append(targets, target{name: "public", base: pub, timeout: 10 * time.Second})
	}
	if len(targets) == 0 {
		return errors.New("未配置同步服务器地址")
	}

	var payload []byte
	if reqBody != nil {
		var err error
		payload, err = json.Marshal(reqBody)
		if err != nil {
			return err
		}
	}

	var lastErr error
	for _, tgt := range targets {
		reqCtx, cancel := context.WithTimeout(ctx, tgt.timeout)
		var bodyReader io.Reader
		if payload != nil {
			bodyReader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(reqCtx, method, tgt.base+path, bodyReader)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if familyID != "" {
			req.Header.Set("X-Family-ID", familyID)
		}

		c.mu.RLock()
		rt := c.transport
		c.mu.RUnlock()
		client := &http.Client{Timeout: tgt.timeout, Transport: rt}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue // network failure: fail over to public URL
		}
		raw, rerr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if rerr != nil {
			lastErr = rerr
			continue
		}

		c.mu.Lock()
		c.activeEndpoint = tgt.name
		c.mu.Unlock()

		if resp.StatusCode >= 400 {
			var errObj struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(raw, &errObj)
			msg := errObj.Error
			if msg == "" {
				msg = string(raw)
			}
			// 4xx/5xx from reachable server should NOT fail over blindly
			return &APIError{Status: resp.StatusCode, Message: msg}
		}
		if respBody != nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, respBody); err != nil {
				return err
			}
		}
		return nil
	}

	c.mu.Lock()
	c.activeEndpoint = "offline"
	c.mu.Unlock()
	if lastErr == nil {
		lastErr = errors.New("无法连接同步服务器")
	}
	return lastErr
}

// Register creates an account (and optionally creates or joins a family).
func (c *Client) Register(ctx context.Context, req model.AuthRequest) (model.AuthResponse, error) {
	var out model.AuthResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/auth/register", "", "", req, &out)
	return out, err
}

// Login authenticates an existing user.
func (c *Client) Login(ctx context.Context, username, password string) (model.AuthResponse, error) {
	var out model.AuthResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/auth/login", "", "", model.AuthRequest{
		Username: username,
		Password: password,
	}, &out)
	return out, err
}

// GetMe fetches current user & family list.
func (c *Client) GetMe(ctx context.Context, token string) (model.AuthResponse, error) {
	var out model.AuthResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/me", token, "", nil, &out)
	return out, err
}

// CreateFamily creates a new family space.
func (c *Client) CreateFamily(ctx context.Context, token, name string) (model.FamilyInfo, error) {
	var out model.FamilyInfo
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/families", token, "", model.CreateFamilyRequest{Name: name}, &out)
	return out, err
}

// JoinFamily joins an existing family space using an invite code.
func (c *Client) JoinFamily(ctx context.Context, token, inviteCode string) (model.FamilyInfo, error) {
	var out model.FamilyInfo
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/families/join", token, "", model.JoinFamilyRequest{InviteCode: inviteCode}, &out)
	return out, err
}

// Sync exchanges pending mutations and pulls new changes from the server into store.
func (c *Client) Sync(ctx context.Context, store *localstore.Store) error {
	cfg := store.Config()
	if cfg.AuthToken == "" || cfg.FamilyID == "" {
		return errors.New("尚未登录或加入家庭空间")
	}
	c.SetEndpoints(cfg.LANURL, cfg.PublicURL)
	sinceSeq, ops := store.SyncInput()
	req := model.SyncRequest{
		SinceSeq: sinceSeq,
		Changes:  ops,
	}
	var resp model.SyncResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/sync", cfg.AuthToken, cfg.FamilyID, req, &resp); err != nil {
		return err
	}
	return store.ApplyRemoteChanges(ops, resp.ServerSeq, resp.Changes)
}
