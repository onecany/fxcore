// Package mail 提供 SMTP 邮件发送（仅标准库 net/smtp，零第三方依赖）。
//
// 配置项来自环境变量（见 cmd/server/main.go 与 .env.example）：
//   SMTP_HOST / SMTP_PORT / SMTP_USER / SMTP_PASS / SMTP_FROM。
// 未配置 SMTP_HOST 时 Mailer.Enabled() 返回 false——调用方应降级为
// dev 模式（重置链接写日志并随响应返回），保证无邮件服务器也可联调。
package mail

import (
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

// Config SMTP 邮件配置。
type Config struct {
	Host     string // SMTP 服务器地址；空 = 邮件发送禁用
	Port     int    // SMTP 端口（默认 587）
	Username string // 认证用户名；空 = 无认证
	Password string // 认证密码
	From     string // 发件人地址；空 = 取 Username
}

// Mailer SMTP 邮件发送器。
type Mailer struct {
	cfg Config
}

// New 构造 Mailer（Port 为 0 时归一为 587）。
func New(cfg Config) *Mailer {
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	return &Mailer{cfg: cfg}
}

// Enabled 是否配置了 SMTP（Host 非空）。
func (m *Mailer) Enabled() bool {
	return m != nil && m.cfg.Host != ""
}

// Send 发送一封纯文本邮件（UTF-8，支持中文正文）。
// 收件人/发件人地址只做基本非空校验，地址合法性由 SMTP 服务器裁决。
func (m *Mailer) Send(to, subject, textBody string) error {
	if !m.Enabled() {
		return errors.New("smtp not configured")
	}
	from := m.cfg.From
	if from == "" {
		from = m.cfg.Username
	}
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return errors.New("mail: empty from or to address")
	}
	var auth smtp.Auth
	if m.cfg.Username != "" {
		// PlainAuth 明文认证（STARTTLS 隧道内传输，587 端口标准用法）
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	msg := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"Content-Transfer-Encoding: 8bit\r\n" +
			"\r\n" + textBody)
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
