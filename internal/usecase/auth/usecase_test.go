package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
	"github.com/rs/zerolog"
)

// ─── mocks ────────────────────────────────────────────────────────────────────

type mockWhitelistRepo struct {
	allowed       bool
	allowedErr    error
	listResult    []entity.WhitelistEntry
	listErr       error
	createResult  *entity.WhitelistEntry
	createErr     error
	deleteErr     error
	lastIsAllowed string
	lastCreate    struct {
		email, addedBy, notes string
	}
	lastDeleteID string
}

func (m *mockWhitelistRepo) IsAllowed(_ context.Context, email string) (bool, error) {
	m.lastIsAllowed = email
	return m.allowed, m.allowedErr
}
func (m *mockWhitelistRepo) List(context.Context) ([]entity.WhitelistEntry, error) {
	return m.listResult, m.listErr
}
func (m *mockWhitelistRepo) GetByEmail(context.Context, string) (*entity.WhitelistEntry, error) {
	return nil, repository.ErrWhitelistNotFound
}
func (m *mockWhitelistRepo) GetByID(context.Context, string) (*entity.WhitelistEntry, error) {
	return nil, repository.ErrWhitelistNotFound
}
func (m *mockWhitelistRepo) Create(_ context.Context, email, addedBy, notes string) (*entity.WhitelistEntry, error) {
	m.lastCreate.email = email
	m.lastCreate.addedBy = addedBy
	m.lastCreate.notes = notes
	return m.createResult, m.createErr
}
func (m *mockWhitelistRepo) SetActive(context.Context, string, bool) error {
	return nil
}
func (m *mockWhitelistRepo) Delete(_ context.Context, id string) error {
	m.lastDeleteID = id
	return m.deleteErr
}

type mockProxy struct {
	resp *auth.ProxyLoginResponse
	err  error
}

func (m *mockProxy) Login(context.Context, string, string) (*auth.ProxyLoginResponse, error) {
	return m.resp, m.err
}

type mockGoogle struct {
	info *auth.GoogleTokenInfo
	err  error
}

func (m *mockGoogle) Verify(context.Context, string) (*auth.GoogleTokenInfo, error) {
	return m.info, m.err
}

// ─── LoginEmailPassword ───────────────────────────────────────────────────────

func TestLoginEmailPassword(t *testing.T) {
	t.Parallel()

	type setup struct {
		repo  *mockWhitelistRepo
		proxy *mockProxy
	}

	cases := []struct {
		name    string
		email   string
		pass    string
		setup   setup
		wantErr error
	}{
		{
			name:  "happy path",
			email: "user@dealls.com",
			pass:  "secret",
			setup: setup{
				repo:  &mockWhitelistRepo{allowed: true},
				proxy: &mockProxy{resp: &auth.ProxyLoginResponse{AccessToken: "tok-123", Expire: "2030-01-01T00:00:00Z"}},
			},
			wantErr: nil,
		},
		{
			name:  "blank password",
			email: "user@dealls.com",
			pass:  "",
			setup: setup{repo: &mockWhitelistRepo{allowed: true}, proxy: &mockProxy{}},
			wantErr: auth.ErrInvalidCredentials,
		},
		{
			// Non-whitelisted accounts must be indistinguishable from wrong
			// passwords to prevent email enumeration via the whitelist gate.
			name:  "not whitelisted masks as invalid credentials",
			email: "stranger@gmail.com",
			pass:  "secret",
			setup: setup{
				repo:  &mockWhitelistRepo{allowed: false},
				proxy: &mockProxy{},
			},
			wantErr: auth.ErrInvalidCredentials,
		},
		{
			name:  "upstream rejects",
			email: "user@dealls.com",
			pass:  "wrong",
			setup: setup{
				repo:  &mockWhitelistRepo{allowed: true},
				proxy: &mockProxy{err: auth.ErrInvalidCredentials},
			},
			wantErr: auth.ErrInvalidCredentials,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := auth.NewAuthUsecase(tc.setup.repo, tc.setup.proxy, &mockGoogle{}, "secret", zerolog.Nop())

			res, err := uc.LoginEmailPassword(context.Background(), tc.email, tc.pass)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err: got %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.SessionToken == "" {
				t.Errorf("expected session token")
			}
			if res.Provider != "email" {
				t.Errorf("provider: got %q want email", res.Provider)
			}
		})
	}
}

// ─── LoginGoogle ──────────────────────────────────────────────────────────────

func TestLoginGoogle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		repo    *mockWhitelistRepo
		google  *mockGoogle
		wantErr error
	}{
		{
			name:   "happy path",
			repo:   &mockWhitelistRepo{allowed: true},
			google: &mockGoogle{info: &auth.GoogleTokenInfo{Sub: "g-1", Email: "u@dealls.com", Name: "U"}},
		},
		{
			name:    "google rejects",
			repo:    &mockWhitelistRepo{allowed: true},
			google:  &mockGoogle{err: auth.ErrGoogleInvalidToken},
			wantErr: auth.ErrGoogleInvalidToken,
		},
		{
			name:    "not whitelisted",
			repo:    &mockWhitelistRepo{allowed: false},
			google:  &mockGoogle{info: &auth.GoogleTokenInfo{Sub: "g-1", Email: "stranger@gmail.com"}},
			wantErr: auth.ErrNotWhitelisted,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := auth.NewAuthUsecase(tc.repo, &mockProxy{}, tc.google, "secret", zerolog.Nop())

			res, err := uc.LoginGoogle(context.Background(), "fake-credential")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err: got %v want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.SessionToken == "" {
				t.Error("expected session token")
			}
			if res.Provider != "google" {
				t.Errorf("provider: got %q want google", res.Provider)
			}
			// HMAC token must verify
			if _, _, err := auth.VerifyGoogleSession("secret", res.SessionToken); err != nil {
				t.Errorf("session token failed verify: %v", err)
			}
		})
	}
}

// ─── Whitelist CRUD ───────────────────────────────────────────────────────────

func TestWhitelistCRUD(t *testing.T) {
	t.Parallel()

	t.Run("list returns slice", func(t *testing.T) {
		t.Parallel()
		repo := &mockWhitelistRepo{listResult: []entity.WhitelistEntry{{Email: "a@b.com"}}}
		uc := auth.NewAuthUsecase(repo, nil, nil, "x", zerolog.Nop())
		out, err := uc.ListWhitelist(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 {
			t.Errorf("got %d entries", len(out))
		}
	})

	t.Run("add normalizes email", func(t *testing.T) {
		t.Parallel()
		repo := &mockWhitelistRepo{createResult: &entity.WhitelistEntry{Email: "x@y.com"}}
		uc := auth.NewAuthUsecase(repo, nil, nil, "x", zerolog.Nop())
		_, err := uc.AddWhitelist(context.Background(), "  X@Y.COM  ", "admin", "note")
		if err != nil {
			t.Fatal(err)
		}
		if repo.lastCreate.email != "x@y.com" {
			t.Errorf("email not normalized: %q", repo.lastCreate.email)
		}
	})

	t.Run("add empty email errors", func(t *testing.T) {
		t.Parallel()
		uc := auth.NewAuthUsecase(&mockWhitelistRepo{}, nil, nil, "x", zerolog.Nop())
		if _, err := uc.AddWhitelist(context.Background(), "  ", "", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("remove forwards id", func(t *testing.T) {
		t.Parallel()
		repo := &mockWhitelistRepo{}
		uc := auth.NewAuthUsecase(repo, nil, nil, "x", zerolog.Nop())
		if err := uc.RemoveWhitelist(context.Background(), "abc"); err != nil {
			t.Fatal(err)
		}
		if repo.lastDeleteID != "abc" {
			t.Errorf("got %q", repo.lastDeleteID)
		}
	})
}
