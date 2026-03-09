package auth

import (
	"log/slog"

	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/redis/go-redis/v9"
)

type AuthHandler struct {
	db           *database.Database
	redis        *redis.Client
	secret       *accessSecretWrap
	EmailManager *email.EmailManager
	providers    map[string]OAuthProvider
}

func NewAuthHandler(db *database.Database, redisC *redis.Client, emailManager *email.EmailManager, accessSecret string) *AuthHandler {
	h := &AuthHandler{
		db:           db,
		redis:        redisC,
		EmailManager: emailManager,
		secret: &accessSecretWrap{
			accessSecret: []byte(accessSecret),
		},
		providers: make(map[string]OAuthProvider),
	}
	return h
}
func (h *AuthHandler) RegisterProviders(providers ...OAuthProvider) {
	for _, provider := range providers {
		h.registerProvider(provider)
	}
}

func (h *AuthHandler) registerProvider(p OAuthProvider) {
	name := p.Name()
	slog.Info("registering OAuth provider", "name", name)
	h.providers[name] = p
}
