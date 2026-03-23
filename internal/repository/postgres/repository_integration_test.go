package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	postgresmodule "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/example/em_jgo/internal/domain/subscription"
)

func TestRepositoryCRUDAndTotalCost(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is not available in the environment")
	}
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := postgresmodule.Run(ctx,
		"postgres:18",
		postgresmodule.WithDatabase("subscriptions"),
		postgresmodule.WithUsername("postgres"),
		postgresmodule.WithPassword("postgres"),
	)
	require.NoError(t, err)
	defer func() {
		_ = container.Terminate(ctx)
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, waitForDatabase(ctx, dsn))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	require.NoError(t, applyMigrations(ctx, dsn))

	repo := New(pool)
	userID := uuid.New()
	endDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	created, err := repo.Create(ctx, subscription.Subscription{
		ID:          uuid.New(),
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      userID,
		StartDate:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:     &endDate,
	})
	require.NoError(t, err)
	require.Equal(t, "Yandex Plus", created.ServiceName)

	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, fetched.ID)

	updated, err := repo.Update(ctx, subscription.Subscription{
		ID:          created.ID,
		ServiceName: "Netflix",
		Price:       500,
		UserID:      userID,
		StartDate:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:     &endDate,
	})
	require.NoError(t, err)
	require.Equal(t, "Netflix", updated.ServiceName)

	items, total, err := repo.List(ctx, subscription.ListFilter{UserID: &userID, ServiceName: stringPtr("Netflix"), Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.EqualValues(t, 1, total)

	totalCost, err := repo.CalculateTotalCost(ctx, subscription.TotalCostFilter{
		StartPeriod: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndPeriod:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		UserID:      &userID,
		ServiceName: stringPtr("Netflix"),
	})
	require.NoError(t, err)
	require.Equal(t, 1000, totalCost)

	require.NoError(t, repo.Delete(ctx, created.ID))
}

func applyMigrations(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, migrationsDir()); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func waitForDatabase(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		pingCtx, cancel := context.WithTimeout(ctx, time.Second)
		lastErr = db.PingContext(pingCtx)
		cancel()
		if lastErr == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for db: %w (last ping error: %v)", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func migrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "migrations"))
}

func stringPtr(value string) *string {
	return &value
}
