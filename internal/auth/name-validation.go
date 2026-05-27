package auth

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	MinLengthUsername = 3
	MaxLengthUsername = 15

	MinLengthDisplayName = 1
	MaxLengthDisplayName = 50
)

var (
	ErrUsernameInvalid = errors.New("username_invalid")
	ErrUsernameBlocked = errors.New("username_blacklisted")
	ErrUsernameShort   = errors.New("username_short")
	ErrUsernameLong    = errors.New("username_long")

	ErrDisplayNameInvalid = errors.New("display_name_invalid")
	ErrDisplayNameShort   = errors.New("display_name_short")
	ErrDisplayNameLong    = errors.New("display_name_long")
)

var usernameBlackList = map[string]struct{}{
	"sign-up":        {},
	"sign-in":        {},
	"password-reset": {},
	"2fa":            {},
	"api":            {},

	"home":          {},
	"explore":       {},
	"notifications": {},
	"chat":          {},
	"profile":       {},
}

func validateUsername(username string) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" || strings.Contains(username, "@") || !utf8.ValidString(username) {
		return false, ErrUsernameInvalid
	}
	if _, isInvalid := usernameBlackList[username]; isInvalid {
		return false, ErrUsernameBlocked
	}
	charCount := utf8.RuneCountInString(username)
	if charCount < MinLengthUsername {
		return false, ErrUsernameShort
	}
	if charCount > MaxLengthUsername {
		return false, ErrUsernameLong
	}
	return true, nil
}
func validateDisplayName(displayName string) (bool, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || strings.Contains(displayName, "@") || !utf8.ValidString(displayName) {
		return false, ErrDisplayNameInvalid
	}
	charCount := utf8.RuneCountInString(displayName)
	if charCount < MinLengthDisplayName {
		return false, ErrDisplayNameShort
	}
	if charCount > MaxLengthDisplayName {
		return false, ErrDisplayNameLong
	}
	return true, nil
}
