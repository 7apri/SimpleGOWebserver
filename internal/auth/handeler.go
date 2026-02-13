package auth

import (
	"log/slog"

	"github.com/7apri/SimpleGOWebserver/internal/database"
)

type AuthHandler struct {
	db        *database.Database
	secret    *accessSecretWrap
	providers map[string]OAuthProvider
}

func NewAuthHandler(db *database.Database, accessSecret string, providers ...OAuthProvider) *AuthHandler {
	h := &AuthHandler{
		db: db,
		secret: &accessSecretWrap{
			accessSecret: []byte(accessSecret),
		},
		providers: make(map[string]OAuthProvider, len(providers)),
	}
	for _, provider := range providers {
		h.registerProvider(provider)
	}

	return h
}
func (h *AuthHandler) registerProvider(p OAuthProvider) {
	name := p.Name()
	slog.Info("registering OAuth provider", "name", name)
	h.providers[name] = p
}
