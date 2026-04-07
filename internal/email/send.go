package email

import (
	"bytes"
	"encoding/base64"
	"errors"
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

var ErrTemplateNotFound = errors.New("template was not found")

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
	if tmpl == nil {
		return ErrTemplateNotFound
	}
	if err := tmpl.ExecuteTemplate(buf, "email_subject", ctx); err != nil {
		return err
	}
	subject := strings.ReplaceAll(strings.TrimSpace(buf.String()), "\n", "")
	subject = strings.ReplaceAll(subject, "\r", "")
	buf.Reset()

	err := tmpl.Execute(buf, ctx.Data)
	if err != nil {
		return err
	}
	htmlBody := make([]byte, buf.Len())
	copy(htmlBody, buf.Bytes())
	buf.Reset()

	fmt.Fprintf(buf, "From: %s\r\n", mgr.from)
	fmt.Fprintf(buf, "To: %s\r\n", ctx.Reciever)
	fmt.Fprintf(buf, "Subject: %s\r\n", subject)
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString("\r\n")

	encoder := base64.NewEncoder(base64.StdEncoding, buf)
	encoder.Write(htmlBody)
	encoder.Close()

	return smtp.SendMail(
		mgr.host,
		mgr.auth,
		mgr.from,
		[]string{ctx.Reciever},
		buf.Bytes(),
	)
}

const (
	baseUrl = "https://local.7apri.cfd"

	secureUrlBase = baseUrl + "/secure"
	resetUrlBase  = baseUrl + "/password-reset"
	verifyUrlBase = baseUrl + "/account-verify"
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
