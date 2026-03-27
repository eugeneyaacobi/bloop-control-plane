package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	adminapi "bloop-control-plane/internal/api/admin"
	runtimeapi "bloop-control-plane/internal/api/runtime"
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
	"github.com/jackc/pgx/v5/pgxpool"
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

	return r
}
