package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	queries := []string{
		`INSERT INTO accounts (id, display_name) VALUES ('acct_default', 'Gene / default-org') ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO users (id, email, display_name) VALUES ('user_gene', 'gene@example.com', 'Gene') ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO memberships (id, user_id, account_id, role) VALUES ('mem_gene_default', 'user_gene', 'acct_default', 'owner') ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO tunnels (id, account_id, hostname, target, access, status, region, owner, risk) VALUES ('api', 'acct_default', 'api.bloop.to', 'app-server:8080', 'token-protected', 'healthy', 'iad-1', 'gene', 'low') ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO tunnels (id, account_id, hostname, target, access, status, region, owner, risk) VALUES ('admin', 'acct_default', 'admin.bloop.to', 'backoffice:3000', 'basic-auth', 'guarded', 'iad-1', 'gene', 'medium') ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO tunnels (id, account_id, hostname, target, access, status, region, owner, risk) VALUES ('hooks', 'acct_default', 'hooks.bloop.to', 'webhook-gateway:8787', 'public', 'hot', 'ord-1', 'gene', 'watch') ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO review_flags (id, item, reason, severity) VALUES ('rf_1', 'public-demo.bloop.to', 'Public route points at a demo target with no auth', 'elevated') ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO onboarding_steps (id, account_id, step_key, title, detail, state) VALUES ('ob_1', 'acct_default', 'connect-target', 'Connect first target', 'Link a dev server, webhook receiver, or admin app to a named route.', 'done') ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO onboarding_steps (id, account_id, step_key, title, detail, state) VALUES ('ob_2', 'acct_default', 'protect-routes', 'Protect sensitive routes', 'Add token or basic-auth coverage where public exposure would be stupid.', 'active') ON CONFLICT (id) DO NOTHING`,
	}

	for _, query := range queries {
		if _, err := pool.Exec(ctx, query); err != nil {
			return err
		}
	}
	return nil
}
