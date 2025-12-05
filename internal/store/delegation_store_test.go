package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"tezos-delegation-service/db"
)

func TestBulkInsert_Idempotent(t *testing.T) {
	dsn := "postgres://xtz:xtz@localhost:5432/xtz?sslmode=disable"

	dbConn, err := db.New(dsn)
	require.NoError(t, err, "postgres must be available (use: docker-compose up)")
	defer func(dbConn *sql.DB) {
		_ = dbConn.Close()
	}(dbConn)

	// Get absolute path to migrations directory
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../..")
	migrationsPath := "file://" + filepath.Join(projectRoot, "db/migrations")

	err = db.MigrateWithPath(dsn, migrationsPath)
	require.NoError(t, err, "failed to run migrations")

	s := NewDelegationStore(dbConn)

	ctx := context.Background()
	ts := time.Now().UTC()

	rows := []InsertDelegation{
		{TzktID: 1, Timestamp: ts, Amount: 100, Delegator: "tz1abc", Level: 10},
	}
	require.NoError(t, s.BulkInsert(ctx, rows))
	require.NoError(t, s.BulkInsert(ctx, rows))

	page, err := s.GetPage(ctx, nil, 100, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(page), 1)
}
