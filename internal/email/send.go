package email

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/templates"
)

type EmailCtx struct {
	Reciever string
	EmailTemplateIdentifier
}

type EmailTemplateIdentifier struct {
	Lang string
	Name string
	Data any
}

func (mgr *EmailManager) SendEmail(ctx *EmailCtx) error {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("email_panic", "err", r)
		}
	}()

	buf := mgr.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		if buf.Cap() <= 65536 {
			buf.Reset()
			mgr.bufferPool.Put(buf)
		}
	}()

	tmpl := mgr.tmplMgr.Get(ctx.Lang, templates.TemplateKey{Kind: "email", Name: ctx.Name})
	if err := tmpl.ExecuteTemplate(buf, "email_subject", ctx); err != nil {
		return err
	}
	subject := buf.String()
	buf.Reset()

	err := tmpl.Execute(buf, ctx.Data)
	if err != nil {
		return err
	}
	htmlBody := buf.String()

	var message strings.Builder
	fmt.Fprintf(&message, "From: %s\r\n", mgr.from)
	fmt.Fprintf(&message, "To: %s\r\n", ctx.Reciever)
	fmt.Fprintf(&message, "Subject: %s\r\n", subject)
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	message.WriteString("\r\n")
	message.WriteString(htmlBody)

	return smtp.SendMail(
		mgr.host,
		mgr.auth,
		mgr.from,
		[]string{ctx.Reciever},
		[]byte(message.String()),
	)
}

const (
	baseUrl = "https://local.7apri.cfd"

	secureUrlBase = baseUrl + "/secure"
	verifyUrlBase = baseUrl + "/verify"
)

type UserDetail struct {
	UserContact
	Lang string `json:"lang"`
}
type UserContact struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}
type ChallengeType string

const (
	ChallengeVerify ChallengeType = "verify"
	ChallengeReset  ChallengeType = "reset"
	ChallengeLock   ChallengeType = "lock"
)

type GeneratedChallenge struct {
	ChallengeRaw
	ChallengeHash
}

type ChallengeHash struct {
	TokenHash string
	CodeHash  string
}

type ChallengeRaw struct {
	RawToken string
	RawCode  string
}
