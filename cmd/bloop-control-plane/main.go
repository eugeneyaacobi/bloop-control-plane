package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"bloop-control-plane/internal/api"
	"bloop-control-plane/internal/audit"
	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/db"
	"bloop-control-plane/internal/db/migrations"
	"bloop-control-plane/internal/logging"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
	"bloop-control-plane/pkg/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("bloop-control-plane %s (%s) %s\n", version.Version, version.Commit, version.Date)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.LogLevel)
	var customerRepo repository.CustomerRepository
	var adminRepo repository.AdminRepository
	var onboardingRepo repository.OnboardingRepository
	var signupRepo repository.SignupRepository
	var sessionRepo repository.SessionRepository
	var signupService *service.SignupService
	ready := false
	if cfg.DatabaseURL != "" {
		pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "connect db: %v\n", err)
			os.Exit(1)
		}
		defer pool.Close()

		if err := migrations.Apply(context.Background(), pool); err != nil {
			fmt.Fprintf(os.Stderr, "apply migrations: %v\n", err)
			os.Exit(1)
		}
		if err := db.Seed(context.Background(), pool); err != nil {
			fmt.Fprintf(os.Stderr, "seed db: %v\n", err)
			os.Exit(1)
		}
		logger.Info("database connected and seeded")
		customerRepo = repository.NewPostgresCustomerRepository(pool)
		adminRepo = repository.NewPostgresAdminRepository(pool)
		onboardingRepo = repository.NewPostgresOnboardingRepository(pool)
		signupRepo = repository.NewPostgresSignupRepository(pool)
		sessionRepo = repository.NewPostgresSessionRepository(pool)
		provisioningRepo := repository.NewPostgresProvisioningRepository(pool)
		runtimeRepo := repository.NewPostgresRuntimeRepository(pool)
		runtimeInstallationRepo := repository.NewPostgresRuntimeInstallationRepository(pool)
		runtimeInstallationService := service.NewRuntimeInstallationService(runtimeInstallationRepo)
		sessionVersionRepo := repository.NewPostgresSessionVersionRepository(pool)
		auditRecorder := audit.New(pool)
		emailService := service.NewEmailService(cfg)
		var issuer *session.Issuer
		if cfg.SessionSecret != "" {
			if tokens, err := session.NewTokenManager(cfg.SessionSecret); err == nil {
				issuer = session.NewIssuer(tokens, cfg.SessionCookieName, cfg.SessionTTL, sessionVersionRepo)
			}
		}
		signupService = service.NewSignupService(signupRepo, emailService, auditRecorder, cfg, issuer, provisioningRepo)
		ready = true

		authRepo := repository.NewPostgresAuthRepository(pool)
		auditRepo := repository.NewPostgresAuditRepository(pool)
		lockoutRepo := repository.NewPostgresLockoutRepository(pool)
		tokenRepo := repository.NewPostgresTokenRepository(pool)
		webauthnRepo := repository.NewPostgresWebAuthnRepository(pool)
		passwordResetRepo := repository.NewPostgresPasswordResetRepository(pool)

		router := api.NewRouter(api.RouterDeps{
			CustomerRepo:               customerRepo,
			AdminRepo:                  adminRepo,
			OnboardingRepo:             onboardingRepo,
			SessionRepo:                sessionRepo,
			RuntimeRepo:                runtimeRepo,
			SignupService:              signupService,
			RuntimeInstallationService: runtimeInstallationService,
			Config:                     cfg,
			IsReady:                    func() bool { return ready },
			DBPool:                     pool,
			AuthRepo:                   authRepo,
			AuditRepo:                  auditRepo,
			LockoutRepo:                lockoutRepo,
			TokenRepo:                  tokenRepo,
			WebAuthnRepo:               webauthnRepo,
			PasswordResetRepo:          passwordResetRepo,
		})
		logger.Info("control plane starting", "listen_addr", cfg.ListenAddr, "smtp_host", logging.Redact(cfg.SMTPHost), "prototype_mode", cfg.PrototypeMode, "dev_auth_fallback", cfg.AllowDevAuthFallback)
		if err := http.ListenAndServe(cfg.ListenAddr, router); err != nil {
			fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	logger.Warn("starting without database connection is not supported for signup/onboarding plumbing")
	os.Exit(1)
}
