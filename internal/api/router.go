package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	authapi "bloop-control-plane/internal/api/auth"
	adminapi "bloop-control-plane/internal/api/admin"
	runtimeapi "bloop-control-plane/internal/api/runtime"
	customerapi "bloop-control-plane/internal/api/customer"
	onboardingapi "bloop-control-plane/internal/api/onboarding"
	sessionapi "bloop-control-plane/internal/api/session"
	tokensapi "bloop-control-plane/internal/api/tokens"
	webauthnapi "bloop-control-plane/internal/api/webauthn"
	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/runtime"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"
)

type RouterDeps struct {
	CustomerRepo               repository.CustomerRepository
	AdminRepo                  repository.AdminRepository
	OnboardingRepo             repository.OnboardingRepository
	SessionRepo                repository.SessionRepository
	RuntimeRepo                runtime.Repository
	SignupService              *service.SignupService
	RuntimeInstallationService *service.RuntimeInstallationService
	Config                     *config.Config
	IsReady                    func() bool
	DBPool                     *pgxpool.Pool
	// Auth deps (optional — wire when available)
	AuthRepo    repository.AuthRepository
	AuditRepo   repository.AuditRepository
	LockoutRepo repository.LockoutRepository
	TokenRepo          repository.TokenRepository
	WebAuthnRepo       repository.WebAuthnRepository
	PasswordResetRepo  repository.PasswordResetRepository
}

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(securityHeaders)

	// CORS
	if deps.Config != nil && len(deps.Config.CORSAllowedOrigins) > 0 {
		corsHandler := cors.New(cors.Options{
			AllowedOrigins:   deps.Config.CORSAllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			AllowCredentials: true,
			MaxAge:           300,
		})
		r.Use(corsHandler.Handler)
	} else {
		// Reject cross-origin requests when CORS is not explicitly configured
		r.Use(rejectCrossOrigin)
	}

	// Request metrics
	var totalRequests atomic.Int64
	var activeRequests atomic.Int64

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			totalRequests.Add(1)
			activeRequests.Add(1)
			defer activeRequests.Add(-1)
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "service": "bloop-control-plane"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if deps.IsReady != nil && !deps.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "ready": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "ready": true})
	})
	r.Get("/metricsz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_requests":  totalRequests.Load(),
			"active_requests": activeRequests.Load(),
			"service":         "bloop-control-plane",
		})
	})

	cfg := deps.Config
	if cfg == nil {
		cfg = &config.Config{}
	}
	var tokenManager *session.TokenManager
	if strings.TrimSpace(cfg.SessionSecret) != "" {
		tokens, err := session.NewTokenManager(cfg.SessionSecret)
		if err == nil {
			tokenManager = tokens
		}
	}

	customerService := service.NewCustomerService(deps.CustomerRepo, deps.RuntimeRepo)
	customerHandler := &customerapi.Handler{Service: customerService}
	adminService := service.NewAdminService(deps.AdminRepo, deps.RuntimeRepo)
	adminHandler := &adminapi.Handler{Service: adminService}
	onboardingService := service.NewOnboardingService(deps.OnboardingRepo)
	onboardingHandler := &onboardingapi.Handler{
		Service:              onboardingService,
		SignupService:        deps.SignupService,
		SignupRequestLimiter: security.NewRateLimiter(5, time.Minute),
		SignupVerifyLimiter:  security.NewRateLimiter(10, time.Minute),
		SessionCookieName:    cfg.SessionCookieName,
		SessionCookieSecure:  cfg.SessionCookieSecure,
		SessionCookieDomain:  cfg.SessionCookieDomain,
	}
	sessionHandler := &sessionapi.Handler{Service: service.NewSessionService(deps.SessionRepo), CookieName: cfg.SessionCookieName, CookieSecure: cfg.SessionCookieSecure, CookieDomain: cfg.SessionCookieDomain}

	prototypeAccountID := "acct_default"
	if cfg.AllowDevAuthFallback {
		log.Println("[WARN] AllowDevAuthFallback is enabled — prototype auth bypass is active. This should be false in production.")
	}
	if cfg.PrototypeMode {
		log.Println("[WARN] PrototypeMode is enabled. This should be false in production.")
	}
	prototypeCustomer := session.Resolver{
		PrototypeAccountID: prototypeAccountID,
		PrototypeUserID:    "user_gene",
		PrototypeRole:      "customer",
		AllowPrototype:     cfg.AllowDevAuthFallback,
		CookieName:         cfg.SessionCookieName,
		Tokens:             tokenManager,
		SessionVersions:    deps.SessionRepo,
	}
	prototypeAdmin := session.Resolver{
		PrototypeUserID: "user_gene",
		PrototypeRole:   "admin",
		AllowPrototype:  cfg.AllowDevAuthFallback,
		CookieName:      cfg.SessionCookieName,
		Tokens:          tokenManager,
		SessionVersions: deps.SessionRepo,
	}

	r.Route("/api/session", func(sr chi.Router) {
		sr.Use(prototypeCustomer.Middleware)
		sessionapi.Mount(sr, sessionHandler)
	})

	if deps.DBPool != nil {
		r.Route("/api/runtime", func(sr chi.Router) {
			sr.Use(prototypeCustomer.Middleware)
			runtimeapi.Mount(sr, &runtimeapi.Handler{Pool: deps.DBPool, IngestSecret: cfg.RuntimeIngestSecret, PrototypeMode: cfg.PrototypeMode || cfg.AllowDevAuthFallback, PrototypeAccountID: prototypeAccountID, RuntimeInstallations: deps.RuntimeInstallationService})
		})
	}

	r.Route("/api/customer", func(sr chi.Router) {
		sr.Use(prototypeCustomer.Middleware)
		customerapi.Mount(sr, customerHandler)
	})

	r.Route("/api/admin", func(sr chi.Router) {
		sr.Use(prototypeAdmin.Middleware)
		adminapi.Mount(sr, adminHandler)
	})

	r.Route("/api/onboarding", func(sr chi.Router) {
		sr.Use(prototypeCustomer.Middleware)
		onboardingapi.Mount(sr, onboardingHandler)
	})

	// Auth routes (register, login, refresh)
	if deps.AuthRepo != nil && deps.AuditRepo != nil && deps.LockoutRepo != nil && tokenManager != nil {
		emailSvc := service.NewEmailService(cfg)
		authService := service.NewAuthService(deps.AuthRepo, deps.AuditRepo, deps.LockoutRepo, cfg, tokenManager, emailSvc)
		var passwordResetService *service.PasswordResetService
		if deps.PasswordResetRepo != nil {
			passwordResetService = service.NewPasswordResetService(deps.PasswordResetRepo, deps.AuthRepo, deps.AuditRepo, emailSvc, cfg)
		}
		authHandler := authapi.NewHandler(authService, passwordResetService, tokenManager, cfg.SessionCookieName, cfg.SessionCookieSecure, cfg.SessionCookieDomain)
		r.Route("/api/auth", func(sr chi.Router) {
			authapi.Mount(sr, authHandler)
		})
	}

	// Token management routes (requires session auth)
	if deps.TokenRepo != nil && deps.AuditRepo != nil && tokenManager != nil {
		tokenService := service.NewTokenService(deps.TokenRepo, deps.AuditRepo, cfg)
		tokenHandler := tokensapi.NewHandler(tokenService)
		r.Route("/api/tokens", func(sr chi.Router) {
			sr.Use(prototypeCustomer.Middleware)
			tokensapi.Mount(sr, tokenHandler)
		})
	}

	// WebAuthn routes (requires session auth for registration, public for login begin)
	if deps.AuthRepo != nil && deps.WebAuthnRepo != nil && deps.AuditRepo != nil && tokenManager != nil {
		webauthnService, err := service.NewWebAuthnService(deps.AuthRepo, deps.WebAuthnRepo, deps.AuditRepo, cfg)
		if err == nil {
			webauthnHandler := webauthnapi.NewHandler(webauthnService, tokenManager, cfg.SessionCookieName, cfg.SessionCookieSecure, cfg.SessionCookieDomain)
			r.Route("/api/webauthn", func(sr chi.Router) {
				sr.Use(prototypeCustomer.Middleware)
				webauthnapi.Mount(sr, webauthnHandler)
			})
		}
	}

	return r
}

// securityHeaders adds standard security headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// rejectCrossOrigin rejects requests with an Origin header when CORS is not configured.
func rejectCrossOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			http.Error(w, "cross-origin requests are not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
