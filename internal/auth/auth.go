package auth

import (
	"encoding/hex"
	"errors"
	"log/slog"

	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidToken       = errors.New("inv_token")
	ErrInvalidClaims      = errors.New("inv_claims")
	ErrInvalidIssuer      = errors.New("inv_issuer")
	ErrExtUserNoEmail     = errors.New("external_user_no_email")
	ErrSocialAccountTaken = errors.New("social account taken")
	ErrPasswordShort      = errors.New("password_too_short")
	ErrPasswordLong       = errors.New("password_too_long")
	ErrPasswordSimple     = errors.New("password_too_simple")
)

type AuthHandler struct {
	db           *database.Database
	redis        *redis.Client
	secret       *secretWrap
	EmailManager *email.EmailManager
	i18nManager  *i18n.I18nManager
	providers    map[string]OAuthProvider
}

func NewAuthHandler(db *database.Database, redisC *redis.Client, emailManager *email.EmailManager, i18nManager *i18n.I18nManager, accessSecret, twoFactorSecret, providerSecret, challengeSecret, mfaPepper string) (*AuthHandler, error) {
	h := &AuthHandler{
		db:           db,
		redis:        redisC,
		EmailManager: emailManager,
		i18nManager:  i18nManager,
		secret: &secretWrap{
			access:    []byte(accessSecret),
			mfaPepper: mfaPepper,
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

	decodedSecret, err = hex.DecodeString(challengeSecret)
	if err != nil {
		return nil, err
	}
	h.secret.challenge = decodedSecret

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
