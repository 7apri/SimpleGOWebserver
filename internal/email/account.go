package email

import (
	"log/slog"
)

type EmailDataSecurity struct {
	Username   string
	Email      string
	Code       []string
	Token      string
	SecureLink string
}

func (mgr *EmailManager) SendVerificationEmail(challange ChallengeRaw, user UserDetail) {
	err := mgr.SendEmail(&EmailCtx{
		Reciever: user.Email,
		EmailTemplateIdentifier: EmailTemplateIdentifier{
			Lang: user.Lang,
			Name: "verify",
			Data: EmailDataSecurity{
				Username:   user.Username,
				Email:      user.Email,
				Code:       []string{challange.RawCode[:3], challange.RawCode[3:]},
				Token:      challange.RawToken,
				SecureLink: "not implemented yet",
			},
		},
	})
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.Email, "kind", "verify")
	}
}

func (mgr *EmailManager) SendPasswordResetEmail(challange ChallengeRaw, user UserDetail) {
	err := mgr.SendEmail(&EmailCtx{
		Reciever: user.Email,
		EmailTemplateIdentifier: EmailTemplateIdentifier{
			Lang: user.Lang,
			Name: "reset",
			Data: EmailDataSecurity{
				Username:   user.Username,
				Code:       []string{challange.RawCode[:3], challange.RawCode[3:]},
				Token:      challange.RawToken,
				SecureLink: "not implemented yet",
			},
		},
	})
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.Email, "kind", "reset")
	}
}

type NewLoginInfo struct {
	Device     string
	IP         string
	Time       string
	SecureLink string
}

func (mgr *EmailManager) SendNewLoginEmail(info *NewLoginInfo, user UserDetail) {
	err := mgr.SendEmail(&EmailCtx{
		Reciever: user.Email,
		EmailTemplateIdentifier: EmailTemplateIdentifier{
			Lang: user.Lang,
			Name: "new-login",
			Data: info,
		},
	})
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.Email, "kind", "new-login")
	}
}

func (mgr *EmailManager) SendAccountExistsEmail(user UserDetail) {
	err := mgr.SendEmail(&EmailCtx{
		Reciever: user.Email,
		EmailTemplateIdentifier: EmailTemplateIdentifier{
			Lang: user.Lang,
			Name: "account-exists",
			Data: user,
		},
	})
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.Email, "kind", "account-exists")
	}
}

func (mgr *EmailManager) SendSecurityPasswordReset(user UserDetail) {
	err := mgr.SendEmail(&EmailCtx{
		Reciever: user.Email,
		EmailTemplateIdentifier: EmailTemplateIdentifier{
			Lang: user.Lang,
			Name: "account-exists",
			Data: user,
		},
	})
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.Email, "kind", "account-exists")
	}
}

func (mgr *EmailManager) SendWelcomeEmail(user UserDetail) {
	err := mgr.SendEmail(&EmailCtx{
		Reciever: user.Email,
		EmailTemplateIdentifier: EmailTemplateIdentifier{
			Lang: user.Lang,
			Name: "welcome",
			Data: user,
		},
	})
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.Email, "kind", "welcome")
	}
}
