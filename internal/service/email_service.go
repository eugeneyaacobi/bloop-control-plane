package service

import (
	"context"
	"fmt"
	"net/smtp"

	"bloop-control-plane/internal/config"
)

type VerificationEmailSender interface {
	SendVerificationEmail(ctx context.Context, toEmail, token string) error
}

type EmailService struct {
	cfg *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

func (s *EmailService) SendVerificationEmail(ctx context.Context, toEmail, token string) error {
	_ = ctx
	if s.cfg.SMTPHost == "" || s.cfg.SMTPFrom == "" {
		return fmt.Errorf("smtp is not configured")
	}
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	body := []byte("Subject: Verify your bloop signup\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n" +
		"Use this verification token to complete signup:\r\n\r\n" + token + "\r\n")

	var auth smtp.Auth
	if s.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, s.cfg.SMTPFrom, []string{toEmail}, body)
}
