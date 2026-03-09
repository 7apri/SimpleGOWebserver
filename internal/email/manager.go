package email

import (
	"bytes"
	"net/smtp"
	"sync"

	"github.com/7apri/SimpleGOWebserver/internal/templates"
)

type EmailManager struct {
	host       string
	from       string
	auth       smtp.Auth
	tmplMgr    *templates.TemplateManager
	bufferPool *sync.Pool
}

func NewEmailManager(host, from, pass, user string, tmplMgr *templates.TemplateManager) *EmailManager {
	return &EmailManager{
		host:    host,
		from:    from,
		auth:    nil, // smtp.PlainAuth("", user, pass, strings.Split(host, ":")[0])
		tmplMgr: tmplMgr,
		bufferPool: &sync.Pool{
			New: func() any { return bytes.NewBuffer(make([]byte, 0, 4096)) },
		},
	}
}
