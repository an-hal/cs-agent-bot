package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
)

func TestAuthProxyClient_Login(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		status  int
		body    string
		wantErr error
		wantTok string
	}{
		{
			name:    "ok",
			status:  http.StatusOK,
			body:    `{"access_token":"tok-1","expire":"2030-01-01T00:00:00Z","user":{"_id":"1","email":"u@d.com"}}`,
			wantTok: "tok-1",
		},
		{
			name:    "401 invalid",
			status:  http.StatusUnauthorized,
			body:    `{"error":"bad"}`,
			wantErr: auth.ErrInvalidCredentials,
		},
		{
			name:    "500 unavailable",
			status:  http.StatusInternalServerError,
			body:    `oops`,
			wantErr: auth.ErrProxyUnavailable,
		},
		{
			name:    "ok but empty token",
			status:  http.StatusOK,
			body:    `{"access_token":""}`,
			wantErr: nil, // surfaces as a wrapped error, not the sentinels
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := auth.NewAuthProxyClientWithHTTP(srv.URL, srv.Client())
			res, err := c.Login(context.Background(), "u@d.com", "pw")

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err: got %v want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantTok != "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				if res.AccessToken != tc.wantTok {
					t.Errorf("token: got %q want %q", res.AccessToken, tc.wantTok)
				}
			} else if err == nil {
				t.Fatalf("expected error for empty token case")
			}
		})
	}
}

func TestGoogleTokenVerifier_Verify(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string
		status  int
		wantErr bool
	}{
		{
			name:    "ok",
			status:  http.StatusOK,
			body:    `{"sub":"g-1","email":"u@d.com","aud":"client-1","email_verified":"true"}`,
			wantErr: false,
		},
		{
			name:    "wrong aud",
			status:  http.StatusOK,
			body:    `{"sub":"g-1","email":"u@d.com","aud":"someone-else","email_verified":"true"}`,
			wantErr: true,
		},
		{
			name:    "email not verified",
			status:  http.StatusOK,
			body:    `{"sub":"g-1","email":"u@d.com","aud":"client-1","email_verified":"false"}`,
			wantErr: true,
		},
		{
			name:    "google 400",
			status:  http.StatusBadRequest,
			body:    `{"error":"invalid"}`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			v := auth.NewGoogleTokenVerifierWithHTTP("client-1", srv.URL, srv.Client())
			info, err := v.Verify(context.Background(), "fake")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if info.Sub != "g-1" {
				t.Errorf("sub: got %q", info.Sub)
			}
		})
	}
}

func TestGoogleVerifier_RejectsEmptyCredential(t *testing.T) {
	t.Parallel()
	v := auth.NewGoogleTokenVerifierWithHTTP("client-1", "http://unused", nil)
	if _, err := v.Verify(context.Background(), ""); !errors.Is(err, auth.ErrGoogleInvalidToken) {
		t.Fatalf("got %v, want ErrGoogleInvalidToken", err)
	}
}
