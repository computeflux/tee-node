package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/computeflux/tee-provider/pkg/api/auth"
	teechain "github.com/computeflux/tee-provider/tee-chain"
)

// OAuthService 社交登录验证
type OAuthService struct {
	tc     *teechain.TeeChain
	client *http.Client
}

func NewOAuthService(tc *teechain.TeeChain) *OAuthService {
	return &OAuthService{tc: tc}
}

func (s *OAuthService) httpClient() *http.Client {
	if s.client != nil {
		return s.client
	}
	return http.DefaultClient
}

func (s *OAuthService) cfg() *auth.AuthConfig {
	return auth.LoadConfig(s.tc)
}

// ── Auth URLs ─────────────────────────────────────────────

func (s *OAuthService) GoogleAuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", s.cfg().GoogleClientID)
	q.Set("redirect_uri", s.cfg().GoogleRedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
}

func (s *OAuthService) GitHubAuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", s.cfg().GitHubClientID)
	q.Set("redirect_uri", s.cfg().GitHubRedirectURL)
	q.Set("scope", "user:email")
	q.Set("state", state)
	return "https://github.com/login/oauth/authorize?" + q.Encode()
}

// ── Google ────────────────────────────────────────────────

func (s *OAuthService) VerifyGoogleIDToken(idToken string) (*auth.OAuthUserInfo, error) {
	verifyURL := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s",
		url.QueryEscape(idToken))

	resp, err := s.httpClient().Get(verifyURL)
	if err != nil {
		return nil, fmt.Errorf("google tokeninfo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenInfo struct {
		Email     string `json:"email"`
		Name      string `json:"name"`
		Picture   string `json:"picture"`
		Sub       string `json:"sub"`
		Aud       string `json:"aud"`
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tokenInfo); err != nil {
		return nil, fmt.Errorf("parse google response: %w", err)
	}
	if tokenInfo.Error != "" {
		return nil, fmt.Errorf("google token error: %s — %s", tokenInfo.Error, tokenInfo.ErrorDesc)
	}
	if tokenInfo.Aud != s.cfg().GoogleClientID {
		return nil, fmt.Errorf("token audience mismatch")
	}
	if tokenInfo.Email == "" {
		return nil, fmt.Errorf("google did not return email")
	}
	return &auth.OAuthUserInfo{
		Email: tokenInfo.Email, Name: tokenInfo.Name,
		AvatarURL: tokenInfo.Picture, ProviderID: tokenInfo.Sub,
	}, nil
}

func (s *OAuthService) ExchangeGoogleCode(code string) (*auth.OAuthUserInfo, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", s.cfg().GoogleClientID)
	data.Set("client_secret", s.cfg().GoogleClientSecret)
	data.Set("redirect_uri", s.cfg().GoogleRedirectURL)
	data.Set("grant_type", "authorization_code")

	resp, err := s.httpClient().Post("https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("google token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		Error       string `json:"error"`
	}
	if json.Unmarshal(body, &tokenResp) != nil || tokenResp.Error != "" {
		return nil, fmt.Errorf("google token error: %s", string(body))
	}

	if tokenResp.IDToken != "" {
		return s.VerifyGoogleIDToken(tokenResp.IDToken)
	}

	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	userResp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer userResp.Body.Close()

	userBody, _ := io.ReadAll(userResp.Body)
	var ui struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	json.Unmarshal(userBody, &ui)
	if ui.Email == "" {
		return nil, fmt.Errorf("google did not return email")
	}
	return &auth.OAuthUserInfo{
		Email: ui.Email, Name: ui.Name, AvatarURL: ui.Picture, ProviderID: ui.Sub,
	}, nil
}

// ── GitHub ────────────────────────────────────────────────

func (s *OAuthService) ExchangeGitHubCode(code string) (*auth.OAuthUserInfo, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", s.cfg().GitHubClientID)
	data.Set("client_secret", s.cfg().GitHubClientSecret)
	data.Set("redirect_uri", s.cfg().GitHubRedirectURL)

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token",
		strings.NewReader(data.Encode()))
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tr struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	json.Unmarshal(body, &tr)
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("github returned no access_token: %s", string(body))
	}

	userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+tr.AccessToken)
	userReq.Header.Set("Accept", "application/json")
	userResp, err := s.httpClient().Do(userReq)
	if err != nil {
		return nil, err
	}
	defer userResp.Body.Close()

	userBody, _ := io.ReadAll(userResp.Body)
	var gh struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	json.Unmarshal(userBody, &gh)

	if gh.Email == "" {
		emails := s.getGitHubEmails(tr.AccessToken)
		if len(emails) > 0 {
			gh.Email = emails[0]
		}
	}
	if gh.Email == "" {
		return nil, fmt.Errorf("github did not return email")
	}

	name := gh.Name
	if name == "" {
		name = gh.Login
	}
	return &auth.OAuthUserInfo{
		Email: gh.Email, Name: name, AvatarURL: gh.AvatarURL,
		ProviderID: fmt.Sprintf("github_%d", gh.ID),
	}, nil
}

func (s *OAuthService) getGitHubEmails(accessToken string) []string {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	json.Unmarshal(body, &emails)
	var result []string
	for _, e := range emails {
		if e.Verified {
			result = append(result, e.Email)
		}
	}
	return result
}

// ── X/Twitter ─────────────────────────────────────────────

func (s *OAuthService) ExchangeXCode(code, codeVerifier string) (*auth.OAuthUserInfo, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", s.cfg().TwitterClientID)
	data.Set("redirect_uri", s.cfg().TwitterRedirectURL)
	data.Set("code_verifier", codeVerifier)

	req, _ := http.NewRequest("POST", "https://api.twitter.com/2/oauth2/token",
		strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.cfg().TwitterClientID, s.cfg().TwitterClientSecret)

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tr struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	json.Unmarshal(body, &tr)
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("x returned no access_token: %s", string(body))
	}

	userReq, _ := http.NewRequest("GET",
		"https://api.twitter.com/2/users/me?user.fields=profile_image_url,description,name", nil)
	userReq.Header.Set("Authorization", "Bearer "+tr.AccessToken)
	userResp, err := s.httpClient().Do(userReq)
	if err != nil {
		return nil, err
	}
	defer userResp.Body.Close()

	userBody, _ := io.ReadAll(userResp.Body)
	var xu struct {
		Data struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			Username        string `json:"username"`
			ProfileImageURL string `json:"profile_image_url"`
		} `json:"data"`
	}
	json.Unmarshal(userBody, &xu)
	if xu.Data.ID == "" {
		return nil, fmt.Errorf("x did not return user data")
	}

	email := fmt.Sprintf("%s@twitter.local", xu.Data.Username)
	name := xu.Data.Name
	if name == "" {
		name = xu.Data.Username
	}
	return &auth.OAuthUserInfo{
		Email: email, Name: name, AvatarURL: xu.Data.ProfileImageURL,
		ProviderID: "x_" + xu.Data.ID,
	}, nil
}
