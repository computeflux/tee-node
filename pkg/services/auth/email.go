package auth

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	teechain "github.com/computeflux/tee-provider/tee-chain"
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

func NewSMTPEmailService(cfg *AuthConfig) *SMTPEmailService {
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
	// fmt.Println("code", otp)
	subject := "Your verification code"

	body := fmt.Sprintf(
		"Your verification code is: %s\n\nIt expires in 5 minutes.\n\n— %s",
		otp, s.fromName,
	)
	return s.send(to, subject, body)
}

func (s *SMTPEmailService) send(to, subject, body string) error {
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

	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	// 端口 465 使用 TLS 直连（SMTPS）
	if s.port == 465 {
		return s.sendViaTLS(addr, to, msg)
	}

	// 端口 587 使用 STARTTLS
	return s.sendViaSTARTTLS(addr, to, msg)
}

// sendViaTLS 用于端口 465 的 SSL/TLS 直连（如飞书、QQ 邮箱）
func (s *SMTPEmailService) sendViaTLS(addr, to, msg string) error {
	tlsConfig := &tls.Config{
		ServerName: s.host,
	}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if s.user != "" && s.password != "" {
		auth := smtp.PlainAuth("", s.user, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}

// sendViaSTARTTLS 用于端口 587 的标准 STARTTLS（如 Gmail）
func (s *SMTPEmailService) sendViaSTARTTLS(addr, to, msg string) error {
	var auth smtp.Auth
	if s.user != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.user, s.password, s.host)
	}
	return smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg))
}

// ── Factory ───────────────────────────────────────────────

func NewEmailService(tc *teechain.TeeChain) EmailService {
	cfg := LoadConfig(tc)
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
