package bridge

import (
	"context"
	"sync"
	"time"
)

// Service owns the live Bridge Client and lets an administrator
// connect/rotate/disconnect Bridge at runtime (plan §15 "Connect,
// disconnect, and rotate flow"), persisting the result via
// SettingsRepository so it survives restarts. Falls back to the
// config-file/env bridge.* section (fallback Config) when no settings row
// exists yet, matching plan §15's config shape being the documented
// initial-setup path.
type Service struct {
	mu       sync.RWMutex
	client   *Client
	fallback CoreConfig
	repo     *SettingsRepository
}

// NewService builds a Service, preferring a persisted Settings row over
// the static fallback config if one exists. DB errors reading settings at
// startup are treated as "use fallback config" rather than failing boot --
// Bridge connectivity is never allowed to block Core startup (plan §15).
func NewService(fallback CoreConfig, repo *SettingsRepository) *Service {
	s := &Service{fallback: fallback, repo: repo}
	s.client = NewClientFromCoreConfig(fallback)

	if saved, err := repo.Get(context.Background()); err == nil && saved != nil {
		s.client = NewClient(Config{
			Enabled:       saved.Enabled,
			BaseURL:       saved.BaseURL,
			InstanceID:    saved.InstanceID,
			InstanceToken: saved.InstanceToken,
			Timeout:       time.Duration(saved.TimeoutSeconds) * time.Second,
		})
	}
	return s
}

// Client returns the currently active Bridge client. Safe for concurrent
// use; the returned pointer may be swapped out by a concurrent
// Connect/Disconnect, so callers should call Service.Client() again rather
// than caching the pointer across a connect/disconnect boundary.
func (s *Service) Client() *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

// ConnectInput is the admin-supplied connection request (plan §16 "Account
// signup/login/connect form"). baseURL/instanceID/instanceToken are
// obtained by the administrator from Bridge's own owner UI/API (create or
// select an instance, copy its one-time-shown instanceToken) -- Core does
// not itself proxy Bridge account signup/login in this phase; see work-log
// gap notes.
type ConnectInput struct {
	BaseURL       string
	InstanceID    string
	InstanceToken string
}

// Connect validates the supplied credentials against Bridge's own
// GET /v1/instance/status before persisting them, so a typo'd token never
// gets silently saved as "connected". Returns the fetched status on
// success.
func (s *Service) Connect(ctx context.Context, in ConnectInput) (*InstanceStatus, error) {
	timeout := time.Duration(s.fallback.TimeoutSeconds) * time.Second
	trial := NewClient(Config{
		Enabled: true, BaseURL: in.BaseURL, InstanceID: in.InstanceID,
		InstanceToken: in.InstanceToken, Timeout: timeout,
	})
	status, err := trial.InstanceStatus(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if err := s.repo.Upsert(ctx, &Settings{
		Enabled: true, BaseURL: in.BaseURL, InstanceID: in.InstanceID,
		InstanceToken: in.InstanceToken, TimeoutSeconds: s.fallback.TimeoutSeconds,
		ConnectedAt: &now,
	}); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.client = trial
	s.mu.Unlock()
	return status, nil
}

// Disconnect clears the persisted settings and reverts to the (likely
// disabled) fallback config client. It does not deactivate devices on
// Bridge -- Bridge-side instance revocation is an owner-authenticated
// action taken from Bridge's own UI, out of scope for the instance-token
// credential Core holds.
func (s *Service) Disconnect(ctx context.Context) error {
	if err := s.repo.Clear(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	s.client = NewClientFromCoreConfig(s.fallback)
	s.mu.Unlock()
	return nil
}

// Status returns the current sanitized Bridge status, or ErrDisabled if
// Bridge is not configured.
func (s *Service) Status(ctx context.Context) (*InstanceStatus, error) {
	return s.Client().InstanceStatus(ctx)
}
