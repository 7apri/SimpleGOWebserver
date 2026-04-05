package auth

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
)

type GithubOAuth struct {
	clientId     string
	clientSecret string
	baseValues   url.Values
	handlr       *AuthHandler
}

const (
	githubLoginUrl     = "https://github.com/login/oauth/authorize" + "?"
	githubTokenUrl     = "https://github.com/login/oauth/access_token"
	githubUserUrl      = "https://api.github.com/user"
	githubUserEmailUrl = githubUserUrl + "/emails"
	githubScope        = "read:user user:email"
	githubName         = "github"
)

func (g *GithubOAuth) Name() string { return githubName }

func NewGithubOAuth(clientId string, clientSecret string, baseRedirectUrl string) *GithubOAuth {
	v := url.Values{}
	v.Set("client_id", clientId)
	v.Set("redirect_uri", baseRedirectUrl+"?provider=github")
	v.Set("scope", githubScope)
	v.Set("code_challenge_method", "S256")

	return &GithubOAuth{
		clientId:     clientId,
		clientSecret: clientSecret,
		baseValues:   v,
	}
}

func (g *GithubOAuth) getAuthURL(state, challenge string) string {
	v := url.Values{}
	maps.Copy(v, g.baseValues)

	v.Set("state", state)
	v.Set("code_challenge", challenge)

	return githubLoginUrl + v.Encode()
}

type githubTokenResp struct {
	AccessToken string `json:"access_token"`
}
type githubUserResp struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}
type githubEmailResp struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (g *GithubOAuth) exchangeCode(ctx context.Context, code, verifier string) (string, error) {
	reqBody := strings.NewReader(fmt.Sprintf(
		`{"client_id":"%s", "client_secret":"%s", "code":"%s", "code_verifier":"%s"}`,
		g.clientId, g.clientSecret, code, verifier,
	))

	req, _ := http.NewRequestWithContext(ctx, "POST", githubTokenUrl, reqBody)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", err
	}
	defer resp.Body.Close()

	var tokenData githubTokenResp
	err = sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&tokenData)
	if err != nil {
		return "", err
	}

	return tokenData.AccessToken, err
}
func (g *GithubOAuth) fetchUser(ctx context.Context, token string) (*ExternalUser, error) {
	headerVal := "Bearer " + token

	userReq, _ := http.NewRequestWithContext(ctx, "GET", githubUserUrl, nil)
	userReq.Header.Set("Authorization", headerVal)

	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil || userResp.StatusCode != http.StatusOK {
		return nil, err
	}
	defer userResp.Body.Close()

	var user githubUserResp
	if err := sonic.ConfigDefault.NewDecoder(userResp.Body).Decode(&user); err != nil {
		return nil, err
	}

	if user.Email != "" {
		return g.mapToExternalUser(&user), nil
	}

	emailReq, _ := http.NewRequestWithContext(ctx, "GET", githubUserEmailUrl, nil)
	emailReq.Header.Set("Authorization", headerVal)

	emailResp, err := http.DefaultClient.Do(emailReq)
	if err != nil || emailResp.StatusCode != http.StatusOK {
		return nil, err
	}
	defer emailResp.Body.Close()

	var emails []githubEmailResp
	if err := sonic.ConfigDefault.NewDecoder(emailResp.Body).Decode(&emails); err != nil {
		return nil, err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			user.Email = e.Email
			break
		}
	}

	if user.Email == "" {
		return nil, ErrExtUserNoEmail
	}

	return g.mapToExternalUser(&user), nil
}

func (g *GithubOAuth) mapToExternalUser(u *githubUserResp) *ExternalUser {
	return &ExternalUser{
		ID:        strconv.Itoa(u.ID),
		Username:  u.Login,
		Email:     strings.ToLower(u.Email),
		AvatarURL: u.AvatarURL,
	}
}
