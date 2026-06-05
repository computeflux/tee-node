package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	apiAuth "github.com/computeflux/tee-provider/pkg/api/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOAuthService(client *http.Client) *OAuthService {
	apiAuth.ResetConfigCache()
	apiAuth.SetCachedConfigForTest(&apiAuth.AuthConfig{
		GoogleClientID:      "google-client-id",
		GoogleClientSecret:  "google-secret",
		GoogleRedirectURL:   "http://localhost/callback",
		GitHubClientID:      "github-client-id",
		GitHubClientSecret:  "github-secret",
		GitHubRedirectURL:   "http://localhost/github/callback",
		TwitterClientID:     "twitter-client-id",
		TwitterClientSecret: "twitter-secret",
		TwitterRedirectURL:  "http://localhost/x/callback",
	})
	return &OAuthService{tc: nil, client: client}
}

// mockRoundTripper 将特定 host 的请求重定向到 mock server
func newMockRoundTripper(mockServerURL string) http.RoundTripper {
	return &mockRoundTripper{
		base:   http.DefaultTransport,
		target: strings.TrimPrefix(mockServerURL, "http://"),
	}
}

type mockRoundTripper struct {
	base   http.RoundTripper
	target string
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = m.target
	req.Host = m.target
	return m.base.RoundTrip(req)
}

// ── Auth URL tests ─────────────────────────────────────────

func TestOAuthService_GoogleAuthURL(t *testing.T) {
	svc := newTestOAuthService(nil)
	u := svc.GoogleAuthURL("state-123", "")
	require.NotEmpty(t, u)
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	assert.Equal(t, "accounts.google.com", parsed.Host)
	assert.Equal(t, "/o/oauth2/v2/auth", parsed.Path)
	q := parsed.Query()
	assert.Equal(t, "google-client-id", q.Get("client_id"))
	assert.Equal(t, "http://localhost/callback", q.Get("redirect_uri"))
	assert.Equal(t, "state-123", q.Get("state"))
	assert.Equal(t, "openid email profile", q.Get("scope"))
}

func TestOAuthService_GitHubAuthURL(t *testing.T) {
	svc := newTestOAuthService(nil)
	u := svc.GitHubAuthURL("state-456", "")
	require.NotEmpty(t, u)
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	assert.Equal(t, "github.com", parsed.Host)
	assert.Equal(t, "/login/oauth/authorize", parsed.Path)
	q := parsed.Query()
	assert.Equal(t, "github-client-id", q.Get("client_id"))
	assert.Equal(t, "http://localhost/github/callback", q.Get("redirect_uri"))
	assert.Equal(t, "state-456", q.Get("state"))
}

// ── Google ID Token verification ───────────────────────────

func TestOAuthService_VerifyGoogleIDToken_Success(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tokeninfo", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]any{
			"email":   "alice@gmail.com",
			"name":    "Alice",
			"picture": "https://example.com/pic.jpg",
			"sub":     "12345",
			"aud":     "google-client-id",
		})
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	info, err := svc.VerifyGoogleIDToken("test-id-token")
	require.NoError(t, err)
	assert.Equal(t, "alice@gmail.com", info.Email)
	assert.Equal(t, "Alice", info.Name)
	assert.Equal(t, "https://example.com/pic.jpg", info.AvatarURL)
	assert.Equal(t, "12345", info.ProviderID)
}

func TestOAuthService_VerifyGoogleIDToken_AudienceMismatch(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"email": "alice@gmail.com",
			"aud":   "wrong-client-id",
		})
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	_, err := svc.VerifyGoogleIDToken("test-id-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audience mismatch")
}

func TestOAuthService_VerifyGoogleIDToken_NoEmail(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"sub": "12345",
			"aud": "google-client-id",
		})
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	_, err := svc.VerifyGoogleIDToken("test-id-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not return email")
}

func TestOAuthService_VerifyGoogleIDToken_ErrorResponse(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_token",
			"error_description": "Token expired",
		})
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	_, err := svc.VerifyGoogleIDToken("test-id-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token error")
}

// ── Google Code Exchange ───────────────────────────────────

func TestOAuthService_ExchangeGoogleCode_WithIDToken(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "acc-123",
				"id_token":     "id-123",
			})
		case "/tokeninfo":
			q := r.URL.Query()
			assert.Equal(t, "id-123", q.Get("id_token"))
			json.NewEncoder(w).Encode(map[string]any{
				"email": "alice@gmail.com",
				"aud":   "google-client-id",
				"sub":   "12345",
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	info, err := svc.ExchangeGoogleCode("auth-code-123", "")
	require.NoError(t, err)
	assert.Equal(t, "alice@gmail.com", info.Email)
}

func TestOAuthService_ExchangeGoogleCode_WithUserInfo(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "acc-123",
			})
		case "/oauth2/v3/userinfo":
			authz := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer acc-123", authz)
			json.NewEncoder(w).Encode(map[string]any{
				"sub":   "12345",
				"email": "alice@gmail.com",
				"name":  "Alice",
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	info, err := svc.ExchangeGoogleCode("auth-code-123", "")
	require.NoError(t, err)
	assert.Equal(t, "Alice", info.Name)
}

// ── GitHub Code Exchange ───────────────────────────────────

func TestOAuthService_ExchangeGitHubCode_Success(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "gh-token-123",
			})
		case "/user":
			authz := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer gh-token-123", authz)
			json.NewEncoder(w).Encode(map[string]any{
				"id":         42,
				"login":      "alice",
				"name":       "Alice GitHub",
				"avatar_url": "https://github.com/avatar.png",
				"email":      "alice@github.com",
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	info, err := svc.ExchangeGitHubCode("gh-code-123")
	require.NoError(t, err)
	assert.Equal(t, "alice@github.com", info.Email)
	assert.Equal(t, "Alice GitHub", info.Name)
	assert.Equal(t, "github_42", info.ProviderID)
}

func TestOAuthService_ExchangeGitHubCode_FallbackEmail(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "gh-token-123",
			})
		case "/user":
			json.NewEncoder(w).Encode(map[string]any{
				"id":    42,
				"login": "alice",
				// email 为空，需要 fallback 到 /user/emails
			})
		case "/user/emails":
			json.NewEncoder(w).Encode([]map[string]any{
				{"email": "alice@github.com", "verified": true, "primary": true},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	info, err := svc.ExchangeGitHubCode("gh-code-123")
	require.NoError(t, err)
	assert.Equal(t, "alice@github.com", info.Email)
}

func TestOAuthService_ExchangeGitHubCode_NoAccessToken(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"error": "bad_verification_code",
		})
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	_, err := svc.ExchangeGitHubCode("bad-code")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no access_token")
}

// ── X/Twitter Code Exchange ────────────────────────────────

func TestOAuthService_ExchangeXCode_Success(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/oauth2/token":
			// 验证 Basic Auth
			user, pass, ok := r.BasicAuth()
			require.True(t, ok)
			assert.Equal(t, "twitter-client-id", user)
			assert.Equal(t, "twitter-secret", pass)
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "x-token-123",
			})
		case "/2/users/me":
			authz := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer x-token-123", authz)
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                "98765",
					"name":              "Alice X",
					"username":          "alice_x",
					"profile_image_url": "https://x.com/pic.jpg",
				},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	info, err := svc.ExchangeXCode("x-code-123", "verifier-123", "")
	require.NoError(t, err)
	assert.Equal(t, "alice_x@twitter.local", info.Email)
	assert.Equal(t, "Alice X", info.Name)
	assert.Equal(t, "x_98765", info.ProviderID)
}

func TestOAuthService_ExchangeXCode_NoUserData(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/oauth2/token":
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "x-token-123",
			})
		case "/2/users/me":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	_, err := svc.ExchangeXCode("x-code-123", "verifier-123", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not return user data")
}

func TestOAuthService_GetGitHubEmails(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/user/emails", r.URL.Path)
		json.NewEncoder(w).Encode([]map[string]any{
			{"email": "unverified@example.com", "verified": false},
			{"email": "primary@example.com", "verified": true, "primary": true},
			{"email": "secondary@example.com", "verified": true},
		})
	}))
	defer mock.Close()

	svc := newTestOAuthService(&http.Client{Transport: newMockRoundTripper(mock.URL)})
	emails := svc.getGitHubEmails("gh-token-123")
	assert.Equal(t, []string{"primary@example.com", "secondary@example.com"}, emails)
}
