package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	adminapi "bloop-control-plane/internal/api/admin"
	customerapi "bloop-control-plane/internal/api/customer"
	onboardingapi "bloop-control-plane/internal/api/onboarding"
	sessionapi "bloop-control-plane/internal/api/session"
	"bloop-control-plane/internal/config"
	"bloop-control-plane/internal/repository"
	"bloop-control-plane/internal/runtime"
	"bloop-control-plane/internal/security"
	"bloop-control-plane/internal/service"
	"bloop-control-plane/internal/session"
	"github.com/go-chi/chi/v5"
)

type RouterDeps struct {
	CustomerRepo   repository.CustomerRepository
	AdminRepo      repository.AdminRepository
	OnboardingRepo repository.OnboardingRepository
	SessionRepo    repository.SessionRepository
	RuntimeRepo    runtime.Repository
	SignupService  *service.SignupService
	CustomerAudit  service.AuditRecorder
	Config         *config.Config
	IsReady        func() bool
}

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()
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

	customerService := service.NewCustomerService(deps.CustomerRepo, deps.RuntimeRepo, deps.CustomerAudit)
	customerHandler := &customerapi.Handler{Service: customerService}
	adminService := service.NewAdminService(deps.AdminRepo, deps.RuntimeRepo)
	adminHandler := &adminapi.Handler{Service: adminService}
	onboardingService := service.NewOnboardingService(deps.OnboardingRepo)
	onboardingHandler := &onboardingapi.Handler{
		Service:              onboardingService,
		SignupService:        deps.SignupService,
		SignupRequestLimiter: security.NewRateLimiter(5, time.Minute),
		SignupVerifyLimiter:  security.NewRateLimiter(10, time.Minute),
	}
	sessionHandler := &sessionapi.Handler{Service: service.NewSessionService(deps.SessionRepo)}

	prototypeCustomer := session.Resolver{
		PrototypeAccountID: "acct_default",
		PrototypeUserID:    "user_gene",
		PrototypeRole:      "customer",
		AllowPrototype:     cfg.AllowDevAuthFallback,
		CookieName:         cfg.SessionCookieName,
		Tokens:             tokenManager,
	}
	prototypeAdmin := session.Resolver{
		PrototypeUserID: "user_gene",
		PrototypeRole:   "admin",
		AllowPrototype:  cfg.AllowDevAuthFallback,
		CookieName:      cfg.SessionCookieName,
		Tokens:          tokenManager,
	}

	r.Route("/api/session", func(sr chi.Router) {
		sr.Use(prototypeCustomer.Middleware)
		sessionapi.Mount(sr, sessionHandler)
	})

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

	return r
}
