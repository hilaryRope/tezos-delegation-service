package tzkt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFetchDelegations_OK(t *testing.T) {
	now := time.Now().UTC()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/operations/delegations", r.URL.Path)
		require.Equal(t, "applied", r.URL.Query().Get("status"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 1,
				"level": 100,
				"timestamp": "` + now.Format(time.RFC3339) + `",
				"amount": 12345,
				"sender": { "address": "tz1abc" }
			}
		]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 2*time.Second)
	res, err := c.FetchDelegations(context.Background(), now.Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, int64(1), res[0].ID)
	require.Equal(t, int64(12345), res[0].Amount)
	require.Equal(t, "tz1abc", res[0].Sender.Address)
}
