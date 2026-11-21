package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	contactadapters "veyrogram/api/internal/contact/adapters"
	contactapp "veyrogram/api/internal/contact/app"
	contacthttp "veyrogram/api/internal/contact/http"
	identityadapters "veyrogram/api/internal/identity/adapters"
)

// blockDirectory answers the message module's Blocks port from the contact module's store.
type blockDirectory struct {
	store  *contactadapters.MySQLStore
	tenant uint64
}

func (b blockDirectory) BlockedBy(ctx context.Context, blocker, sender string) (bool, error) {
	return b.store.BlockedBy(ctx, b.tenant, blocker, sender)
}

func contactMux(pool *sql.DB, log *slog.Logger, auth func(http.Handler) http.Handler) *http.ServeMux {
	svc := &contactapp.Service{
		Store:    &contactadapters.MySQLStore{DB: pool},
		Accounts: accountDirectory{ids: &identityadapters.MySQLStore{DB: pool}, tenant: 1},
		Tenant:   1,
	}
	mux := http.NewServeMux()
	(&contacthttp.Handler{Svc: svc, Log: log, Auth: auth}).Mount(mux)
	return mux
}
