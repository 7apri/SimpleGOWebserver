package auth

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"
)

type GoogleOAuth struct {
	clientId           string
	clientSecret       string
	baseValuesAuth     url.Values
	baseValuesExchange url.Values
}

func (g *GoogleOAuth) Name() string { return "google" }

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth" + "?"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserURL  = "https://www.googleapis.com/oauth2/v3/userinfo"
	googleScope    = "openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"
)

func NewGoogleOAuth(clientId, clientSecret, baseRedirectUrl string) *GoogleOAuth {
	redirectUri := baseRedirectUrl + "?provider=google"

	vA := url.Values{}
	vA.Set("client_id", clientId)
	vA.Set("redirect_uri", redirectUri)
	vA.Set("scope", googleScope)
	vA.Set("response_type", "code")
	vA.Set("code_challenge_method", "S256")

	vE := url.Values{}
	vE.Set("client_id", clientId)
	vE.Set("redirect_uri", redirectUri)
	vE.Set("client_secret", clientSecret)
	vE.Set("grant_type", "authorization_code")

	return &GoogleOAuth{
		clientId:           clientId,
		clientSecret:       clientSecret,
		baseValuesAuth:     vA,
		baseValuesExchange: vE,
	}
}

func (g *GoogleOAuth) getAuthURL(state, challenge string) string {
	v := url.Values{}
	maps.Copy(v, g.baseValuesAuth)

	v.Set("state", state)
	v.Set("code_challenge", challenge)

	return googleAuthURL + v.Encode()
}

func (g *GoogleOAuth) exchangeCode(ctx context.Context, code, verifier string) (string, error) {
	data := url.Values{}
	maps.Copy(data, g.baseValuesExchange)
	data.Set("code", code)
	data.Set("code_verifier", verifier)

	req, _ := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", err
	}

	defer resp.Body.Close()

	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.AccessToken, nil
}

func (g *GoogleOAuth) fetchUser(ctx context.Context, token string) (*ExternalUser, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", googleUserURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Sub   string `json:"sub"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &ExternalUser{
		ID:       res.Sub,
		Username: res.Name,
		Email:    strings.ToLower(res.Email),
	}, nil
}
