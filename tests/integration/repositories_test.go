package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bloop-control-plane/internal/db"
	"bloop-control-plane/internal/db/migrations"
	"bloop-control-plane/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	migrationsDir := filepath.Join("..", "..", "internal", "db", "migrations")
	if err := migrations.Apply(context.Background(), pool, migrationsDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	for _, table := range []string{"audit_events", "email_verifications", "signup_requests", "onboarding_steps", "review_flags", "tunnels", "memberships", "users", "accounts"} {
		if _, err := pool.Exec(context.Background(), fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
	if err := db.Seed(context.Background(), pool); err != nil {
		t.Fatalf("seed db: %v", err)
	}
	return pool
}

func TestCustomerRepositoryReturnsWorkspaceAndTunnel(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	repo := repository.NewPostgresCustomerRepository(pool)

	acct, tunnels, err := repo.GetWorkspace(context.Background(), "acct_default")
	if err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	if acct.ID != "acct_default" {
		t.Fatalf("unexpected account: %+v", acct)
	}
	if len(tunnels) < 1 {
		t.Fatalf("expected seeded tunnels")
	}

	tunnel, err := repo.GetTunnelByID(context.Background(), "acct_default", "api")
	if err != nil {
		t.Fatalf("get tunnel: %v", err)
	}
	if tunnel == nil || tunnel.ID != "api" {
		t.Fatalf("unexpected tunnel: %+v", tunnel)
	}
}

func TestAdminRepositoryReturnsSeededViews(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	repo := repository.NewPostgresAdminRepository(pool)

	accounts, publicRoutes, flagged, err := repo.OverviewStats(context.Background())
	if err != nil {
		t.Fatalf("overview stats: %v", err)
	}
	if accounts < 1 || publicRoutes < 1 || flagged < 1 {
		t.Fatalf("unexpected overview counts: %d %d %d", accounts, publicRoutes, flagged)
	}

	users, err := repo.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) < 1 {
		t.Fatalf("expected seeded users")
	}

	tunnels, err := repo.ListTunnels(context.Background())
	if err != nil {
		t.Fatalf("list tunnels: %v", err)
	}
	if len(tunnels) < 1 {
		t.Fatalf("expected seeded tunnels")
	}

	flags, err := repo.ListReviewFlags(context.Background())
	if err != nil {
		t.Fatalf("list review flags: %v", err)
	}
	if len(flags) < 1 {
		t.Fatalf("expected seeded review flags")
	}
}

func TestOnboardingRepositoryReturnsSeededSteps(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	repo := repository.NewPostgresOnboardingRepository(pool)

	steps, err := repo.ListSteps(context.Background(), "acct_default")
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) < 1 {
		t.Fatalf("expected seeded steps")
	}
}

func TestSignupRepositoryLifecycle(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	repo := repository.NewPostgresSignupRepository(pool)

	expiresAt := time.Now().UTC().Add(time.Hour)
	if err := repo.CreateSignupRequest(context.Background(), "sr_test", "repo@example.com", "pending"); err != nil {
		t.Fatalf("create signup request: %v", err)
	}
	if err := repo.CreateEmailVerification(context.Background(), "sv_test", "sr_test", "hash_test", "pending", expiresAt); err != nil {
		t.Fatalf("create email verification: %v", err)
	}

	ev, signup, err := repo.FindVerificationByTokenHash(context.Background(), "hash_test")
	if err != nil {
		t.Fatalf("find verification: %v", err)
	}
	if ev == nil || signup == nil {
		t.Fatalf("expected signup verification lookup result")
	}
	if signup.Email != "repo@example.com" {
		t.Fatalf("unexpected signup: %+v", signup)
	}

	verifiedAt := time.Now().UTC()
	if err := repo.MarkVerificationCompleted(context.Background(), "sv_test", "sr_test", verifiedAt); err != nil {
		t.Fatalf("mark verification completed: %v", err)
	}

	var signupState, verificationState string
	if err := pool.QueryRow(context.Background(), "select state from signup_requests where id = 'sr_test'").Scan(&signupState); err != nil {
		t.Fatalf("read signup state: %v", err)
	}
	if err := pool.QueryRow(context.Background(), "select state from email_verifications where id = 'sv_test'").Scan(&verificationState); err != nil {
		t.Fatalf("read verification state: %v", err)
	}
	if signupState != "verified" || verificationState != "verified" {
		t.Fatalf("unexpected final states signup=%s verification=%s", signupState, verificationState)
	}
}
