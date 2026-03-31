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

func (s *EmailService) SendPasswordResetEmail(ctx context.Context, toEmail, rawToken string) error {
	_ = ctx
	if s.cfg.SMTPHost == "" || s.cfg.SMTPFrom == "" {
		return fmt.Errorf("smtp is not configured")
	}
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.PasswordResetBaseURL, rawToken)
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	body := []byte("Subject: Reset your bloop password\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n" +
		"We received a request to reset your password.\r\n\r\n" +
		"Click the link below to set a new password:\r\n\r\n" + resetURL + "\r\n\r\n" +
		"This link expires in 15 minutes. If you did not request a reset, ignore this email.\r\n")

	var auth smtp.Auth
	if s.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}
	err := smtp.SendMail(addr, auth, s.cfg.SMTPFrom, []string{toEmail}, body)
	if err != nil {
		fmt.Printf("ERROR SendPasswordResetEmail: addr=%s user=%s to=%s err=%v\n", addr, s.cfg.SMTPUser, toEmail, err)
	}
	return err
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
