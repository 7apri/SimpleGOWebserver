package email

import (
	"fmt"
	"log/slog"
)

type EmailDataSecurity struct {
	Username   string
	Code       string
	ActionLink string
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
				Code:       challange.RawCode,
				ActionLink: fmt.Sprintf("%s?token=%s", verifyUrlBase, challange.RawToken),
				SecureLink: "verify first",
			},
		},
	})
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.Email, "host", mgr.host, "kind", "verification")
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
				Code:       challange.RawCode,
				ActionLink: fmt.Sprintf("%s/reset?token=%s", baseUrl, challange.RawToken),
				SecureLink: "not implemented yet",
			},
		},
	})
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.Email, "host", mgr.host, "kind", "password reset")
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
		slog.Error("SMTP Error", "err", err, "to", user.Email, "host", mgr.host, "kind", "new login")
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
		slog.Error("SMTP Error", "err", err, "to", user.Email, "host", mgr.host, "kind", "welcome")
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
		slog.Error("SMTP Error", "err", err, "to", user.Email, "host", mgr.host, "kind", "welcome")
	}
}
