package email

import (
	"bytes"
	"net/smtp"
	"sync"

	"github.com/7apri/SimpleGOWebserver/internal/templates"
)

type EmailManager struct {
	auth       smtp.Auth
	tmplMgr    *templates.TemplateManager
	bufferPool *sync.Pool
}

func NewEmailManager(pass, user string, tmplMgr *templates.TemplateManager) *EmailManager {
	return &EmailManager{
		auth:    nil, // smtp.PlainAuth("", user, pass, consts.SmtpHost)
		tmplMgr: tmplMgr,
		bufferPool: &sync.Pool{
			New: func() any { return bytes.NewBuffer(make([]byte, 0, 4096)) },
		},
	}
}
