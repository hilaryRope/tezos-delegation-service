package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Delegation struct {
	Timestamp time.Time `json:"timestamp"`
	Amount    int64     `json:"amount"`
	Delegator string    `json:"delegator"`
	Level     int64     `json:"level"`
}

type DelegationStore interface {
	BulkInsert(ctx context.Context, rows []InsertDelegation) error
	GetPage(ctx context.Context, year *int, limit, offset int) ([]Delegation, error)
	GetLastSeen(ctx context.Context) (time.Time, int64, error)
}

type delegationStore struct {
	db *sql.DB
}

func NewDelegationStore(db *sql.DB) DelegationStore {
	return &delegationStore{db: db}
}

type InsertDelegation struct {
	TzktID    int64
	Timestamp time.Time
	Amount    int64
	Delegator string
	Level     int64
}

func (s *delegationStore) BulkInsert(ctx context.Context, rows []InsertDelegation) error {
	if len(rows) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO delegations (tzkt_id, timestamp, amount, delegator, level, year)
VALUES ($1, $2, $3, $4, $5, EXTRACT(YEAR FROM $2::TIMESTAMPTZ)::INT)
ON CONFLICT (tzkt_id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx,
			r.TzktID,
			r.Timestamp,
			r.Amount,
			r.Delegator,
			r.Level,
		); err != nil {
			return fmt.Errorf("insert delegation tzkt_id=%d: %w", r.TzktID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *delegationStore) GetPage(ctx context.Context, year *int, limit, offset int) ([]Delegation, error) {
	var rows *sql.Rows
	var err error

	if year != nil {
		rows, err = s.db.QueryContext(ctx, `
SELECT timestamp, amount, delegator, level
FROM delegations
WHERE year = $1
ORDER BY timestamp DESC, id DESC
LIMIT $2 OFFSET $3
`, *year, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("query delegations for year %d: %w", *year, err)
		}
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT timestamp, amount, delegator, level
FROM delegations
ORDER BY timestamp DESC, id DESC
LIMIT $1 OFFSET $2
`, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("query delegations: %w", err)
		}
	}
	defer rows.Close()

	out := make([]Delegation, 0, limit)
	for rows.Next() {
		var d Delegation
		if err := rows.Scan(&d.Timestamp, &d.Amount, &d.Delegator, &d.Level); err != nil {
			return nil, fmt.Errorf("scan delegation row: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return out, nil
}

func (s *delegationStore) GetLastSeen(ctx context.Context) (time.Time, int64, error) {
	var ts time.Time
	var lvl sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(timestamp), '0001-01-01'), COALESCE(MAX(level), 0)
FROM delegations
`).Scan(&ts, &lvl)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("query last seen: %w", err)
	}
	return ts, lvl.Int64, nil
}
