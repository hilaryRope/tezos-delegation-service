package poller

import (
	"context"
	"testing"
	"time"

	"tezos-delegation-service/internal/store"
	"tezos-delegation-service/internal/tzkt"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	lastTs time.Time
	insert []store.InsertDelegation
}

func (m *mockStore) BulkInsert(_ context.Context, rows []store.InsertDelegation) error {
	m.insert = append(m.insert, rows...)
	return nil
}
func (m *mockStore) GetPage(context.Context, *int, int, int) ([]store.Delegation, error) {
	return nil, nil
}
func (m *mockStore) GetLastSeen(context.Context) (time.Time, int64, error) {
	return m.lastTs, 0, nil
}

type mockClient struct {
	delegations []tzkt.Delegation
}

func (m *mockClient) FetchDelegations(context.Context, time.Time, int) ([]tzkt.Delegation, error) {
	return m.delegations, nil
}

func TestSyncOnce_Inserts(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockStore{lastTs: now.Add(-time.Hour)}
	mc := &mockClient{
		delegations: []tzkt.Delegation{
			{
				ID:        1,
				Level:     10,
				Timestamp: now,
				Amount:    1000,
				Sender: struct {
					Address string `json:"address"`
				}{Address: "tz1abc"},
			},
		},
	}

	p := NewPoller(Config{
		Store:        ms,
		Client:       mc,
		BatchSize:    100,
		PollInterval: time.Second,
		GenesisStart: now.Add(-24 * time.Hour),
	})

	n, err := p.syncOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Len(t, ms.insert, 1)
	require.Equal(t, int64(1), ms.insert[0].TzktID)
}
