package service

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"bloop-control-plane/internal/config"

	"github.com/matcornic/hermes"
)

type VerificationEmailSender interface {
	SendVerificationEmail(ctx context.Context, toEmail, token string) error
}

type EmailService struct {
	cfg    *config.Config
	hermes hermes.Hermes
}

func NewEmailService(cfg *config.Config) *EmailService {
	h := hermes.Hermes{
		Product: hermes.Product{
			Name:        "bloop",
			Link:        "https://bloop.to",
			Copyright:   "© bloop",
			TroubleText: "If the button above doesn't work, copy and paste this link into your browser:",
		},
	}
	return &EmailService{cfg: cfg, hermes: h}
}

// buildMIME constructs a multipart/alternative MIME message with HTML and plain text bodies.
func buildMIME(subject, htmlBody, textBody string) []byte {
	var sb strings.Builder
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: multipart/alternative; boundary=\"bloop_mime_boundary\"\r\n\r\n")
	sb.WriteString("--bloop_mime_boundary\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
	sb.WriteString(textBody + "\r\n")
	sb.WriteString("--bloop_mime_boundary\r\n")
	sb.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n\r\n")
	sb.WriteString(htmlBody + "\r\n")
	sb.WriteString("--bloop_mime_boundary--\r\n")
	return []byte(sb.String())
}

func (s *EmailService) send(ctx context.Context, toEmail string, msg []byte) error {
	_ = ctx
	if s.cfg.SMTPHost == "" || s.cfg.SMTPFrom == "" {
		return fmt.Errorf("smtp is not configured")
	}
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	var auth smtp.Auth
	if s.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}
	err := smtp.SendMail(addr, auth, s.cfg.SMTPFrom, []string{toEmail}, msg)
	if err != nil {
		fmt.Printf("ERROR email send: to=%s err=%v\n", toEmail, err)
	}
	return err
}

func (s *EmailService) SendPasswordResetEmail(ctx context.Context, toEmail, rawToken string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.PasswordResetBaseURL, rawToken)

	email := hermes.Email{
		Body: hermes.Body{
			Name: "",
			Intros: []string{
				"We received a request to reset your bloop password.",
			},
			Actions: []hermes.Action{
				{
					Instructions: "Click the button below to set a new password:",
					Button: hermes.Button{
						Color: "#6366f1",
						Text:  "Reset Password",
						Link:  resetURL,
					},
				},
			},
			Outros: []string{
				"This link expires in 15 minutes.",
				"If you did not request a password reset, you can safely ignore this email.",
			},
		},
	}

	htmlBody, err := s.hermes.GenerateHTML(email)
	if err != nil {
		return fmt.Errorf("generate reset email HTML: %w", err)
	}
	textBody, err := s.hermes.GeneratePlainText(email)
	if err != nil {
		return fmt.Errorf("generate reset email text: %w", err)
	}

	msg := buildMIME("Reset your bloop password", htmlBody, textBody)
	return s.send(ctx, toEmail, msg)
}

func (s *EmailService) SendVerificationEmail(ctx context.Context, toEmail, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.cfg.PasswordResetBaseURL, token)

	email := hermes.Email{
		Body: hermes.Body{
			Name: "",
			Intros: []string{
				"Welcome to bloop! Please verify your email address to get started.",
			},
			Actions: []hermes.Action{
				{
					Instructions: "Click the button below to confirm your email:",
					Button: hermes.Button{
						Color: "#6366f1",
						Text:  "Verify Email",
						Link:  verifyURL,
					},
				},
			},
			Outros: []string{
				"If you did not create a bloop account, you can safely ignore this email.",
			},
		},
	}

	htmlBody, err := s.hermes.GenerateHTML(email)
	if err != nil {
		return fmt.Errorf("generate verification email HTML: %w", err)
	}
	textBody, err := s.hermes.GeneratePlainText(email)
	if err != nil {
		return fmt.Errorf("generate verification email text: %w", err)
	}

	msg := buildMIME("Verify your bloop account", htmlBody, textBody)
	return s.send(ctx, toEmail, msg)
}
