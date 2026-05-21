package auth

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/computeflux/tee-provider/pkg/api/auth"
)

// EmailService 邮件发送接口
type EmailService interface {
	SendOTP(to, otp string) error
}

// ── MockEmailService ──────────────────────────────────────

type MockEmailService struct {
	FromName string
}

func NewMockEmailService(fromName string) *MockEmailService {
	return &MockEmailService{FromName: fromName}
}

func (s *MockEmailService) SendOTP(to, otp string) error {
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("[MOCK EMAIL] OTP for %s: %s", to, otp)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	return nil
}

// ── SMTPEmailService ──────────────────────────────────────

type SMTPEmailService struct {
	host     string
	port     int
	user     string
	password string
	from     string
	fromName string
}

func NewSMTPEmailService(cfg *auth.AuthConfig) *SMTPEmailService {
	return &SMTPEmailService{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		user:     cfg.SMTPUser,
		password: cfg.SMTPPassword,
		from:     cfg.SMTPFrom,
		fromName: cfg.SMTPFromName,
	}
}

func (s *SMTPEmailService) SendOTP(to, otp string) error {
	subject := "Your verification code"
	body := fmt.Sprintf(
		"Your verification code is: %s\n\nIt expires in 5 minutes.\n\n— %s",
		otp, s.fromName,
	)
	return s.send(to, subject, body)
}

func (s *SMTPEmailService) send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	msg := fmt.Sprintf(
		"From: %s <%s>\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"\r\n"+
			"%s",
		s.fromName, s.from, to, subject, body,
	)

	var auth smtp.Auth
	if s.user != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.user, s.password, s.host)
	}
	return smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg))
}

// ── Factory ───────────────────────────────────────────────

func NewEmailService(cfg *auth.AuthConfig) EmailService {
	if cfg.MockEmail {
		return NewMockEmailService(cfg.SMTPFromName)
	}
	missing := []string{}
	if cfg.SMTPHost == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if cfg.SMTPFrom == "" {
		missing = append(missing, "SMTP_FROM")
	}
	if len(missing) > 0 {
		log.Printf("WARNING: MOCK_EMAIL=false but %s not set. Falling back to mock.", strings.Join(missing, ", "))
		return NewMockEmailService(cfg.SMTPFromName)
	}
	return NewSMTPEmailService(cfg)
}
