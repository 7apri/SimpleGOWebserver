package auth

import "strings"

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

func validateUsername(username string) bool {
	username = strings.TrimSpace(username)
	if username == "" || strings.Contains(username, "@") {
		return false
	}
	if _, isInvalid := usernameBlackList[username]; isInvalid {
		return false
	}
}
