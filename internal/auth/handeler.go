package auth

import (
	"encoding/hex"
	"log/slog"

	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/redis/go-redis/v9"
)

type AuthHandler struct {
	db           *database.Database
	redis        *redis.Client
	secret       *secretWrap
	EmailManager *email.EmailManager
	i18nManager  *i18n.I18nManager
	providers    map[string]OAuthProvider
}

func NewAuthHandler(db *database.Database, redisC *redis.Client, emailManager *email.EmailManager, i18nManager *i18n.I18nManager, accessSecret, twoFactorSecret, providerSecret string) (*AuthHandler, error) {
	h := &AuthHandler{
		db:           db,
		redis:        redisC,
		EmailManager: emailManager,
		i18nManager:  i18nManager,
		secret: &secretWrap{
			access: []byte(accessSecret),
		},
		providers: make(map[string]OAuthProvider),
	}
	decodedSecret, err := hex.DecodeString(twoFactorSecret)
	if err != nil {
		return nil, err
	}
	h.secret.twoFactor = decodedSecret

	decodedSecret, err = hex.DecodeString(providerSecret)
	if err != nil {
		return nil, err
	}
	h.secret.provider = decodedSecret
	return h, nil
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
