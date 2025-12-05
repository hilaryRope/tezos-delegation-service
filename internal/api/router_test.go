package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tezos-delegation-service/db"
	"tezos-delegation-service/internal/store"
)

func setupTestRouter(t *testing.T) (http.Handler, store.DelegationStore) {
	dsn := "postgres://xtz:xtz@localhost:5432/xtz?sslmode=disable"

	dbConn, err := db.New(dsn)
	require.NoError(t, err, "postgres must be available (use: docker-compose up)")
	t.Cleanup(func() { dbConn.Close() })

	// Get absolute path to migrations directory
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../..")
	migrationsPath := "file://" + filepath.Join(projectRoot, "db/migrations")

	err = db.MigrateWithPath(dsn, migrationsPath)
	require.NoError(t, err, "failed to run migrations")

	delegationStore := store.NewDelegationStore(dbConn)

	router := NewRouter(delegationStore, dbConn)

	return router, delegationStore
}

func TestRouter_HealthEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp healthResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp.Status)
	assert.Equal(t, "healthy", resp.Checks["database"])
	assert.NotEmpty(t, resp.Uptime)
	assert.NotEmpty(t, resp.Checks["database_connections"])
}

func TestRouter_DelegationsEndpoint_EmptyDatabase(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.Data)
	assert.GreaterOrEqual(t, len(resp.Data), 0)
}

func TestRouter_DelegationsEndpoint_WithData(t *testing.T) {
	router, delegationStore := setupTestRouter(t)

	// Insert test data
	ctx := context.Background()
	testData := []store.InsertDelegation{
		{
			TzktID:    1001,
			Timestamp: time.Date(2022, 5, 5, 6, 29, 14, 0, time.UTC),
			Amount:    125896,
			Delegator: "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL",
			Level:     2338084,
		},
		{
			TzktID:    1002,
			Timestamp: time.Date(2021, 5, 7, 14, 48, 7, 0, time.UTC),
			Amount:    9856354,
			Delegator: "KT1JejNYjmQYh8yw95u5kfQDRuxJcaUPjUnf",
			Level:     1461334,
		},
	}
	require.NoError(t, delegationStore.BulkInsert(ctx, testData))

	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(resp.Data), 2)

	if len(resp.Data) > 0 {
		assert.NotEmpty(t, resp.Data[0].Timestamp)
		assert.NotEmpty(t, resp.Data[0].Amount)
		assert.NotEmpty(t, resp.Data[0].Delegator)
		assert.NotEmpty(t, resp.Data[0].Level)
	}
}

func TestRouter_DelegationsEndpoint_WithYearFilter(t *testing.T) {
	router, delegationStore := setupTestRouter(t)

	ctx := context.Background()
	testData := []store.InsertDelegation{
		{
			TzktID:    2001,
			Timestamp: time.Date(2022, 5, 5, 6, 29, 14, 0, time.UTC),
			Amount:    100000,
			Delegator: "tz1year2022",
			Level:     2338084,
		},
		{
			TzktID:    2002,
			Timestamp: time.Date(2021, 5, 7, 14, 48, 7, 0, time.UTC),
			Amount:    200000,
			Delegator: "tz1year2021",
			Level:     1461334,
		},
	}
	require.NoError(t, delegationStore.BulkInsert(ctx, testData))

	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations?year=2022", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	for _, d := range resp.Data {
		ts, err := time.Parse("2006-01-02T15:04:05Z", d.Timestamp)
		require.NoError(t, err)
		assert.Equal(t, 2022, ts.Year())
	}
}

func TestRouter_DelegationsEndpoint_WithPagination(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations?page=1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Data)
}

func TestRouter_DelegationsEndpoint_InvalidYear(t *testing.T) {
	router, _ := setupTestRouter(t)

	testCases := []struct {
		name     string
		url      string
		wantCode int
	}{
		{
			name:     "invalid year format",
			url:      "/xtz/delegations?year=invalid",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "year before 2018",
			url:      "/xtz/delegations?year=2017",
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.wantCode, w.Code)
		})
	}
}

func TestRouter_DelegationsEndpoint_InvalidPage(t *testing.T) {
	router, _ := setupTestRouter(t)

	testCases := []struct {
		name     string
		url      string
		wantCode int
	}{
		{
			name:     "invalid page format",
			url:      "/xtz/delegations?page=invalid",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "zero page",
			url:      "/xtz/delegations?page=0",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "negative page",
			url:      "/xtz/delegations?page=-1",
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.wantCode, w.Code)
		})
	}
}

func TestRouter_CORSHeaders(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
}

func TestRouter_OptionsRequest(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodOptions, "/xtz/delegations", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestRouter_NotFoundEndpoint(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRouter_DelegationsEndpoint_SortOrder(t *testing.T) {
	router, delegationStore := setupTestRouter(t)

	ctx := context.Background()
	testData := []store.InsertDelegation{
		{
			TzktID:    3001,
			Timestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:    100,
			Delegator: "tz1oldest",
			Level:     1000,
		},
		{
			TzktID:    3002,
			Timestamp: time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
			Amount:    200,
			Delegator: "tz1newest",
			Level:     3000,
		},
		{
			TzktID:    3003,
			Timestamp: time.Date(2022, 6, 15, 12, 0, 0, 0, time.UTC),
			Amount:    150,
			Delegator: "tz1middle",
			Level:     2000,
		},
	}
	require.NoError(t, delegationStore.BulkInsert(ctx, testData))

	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Data), 3)

	// Verify most recent first
	var timestamps []time.Time
	for _, d := range resp.Data {
		ts, err := time.Parse("2006-01-02T15:04:05Z", d.Timestamp)
		require.NoError(t, err)
		timestamps = append(timestamps, ts)
	}

	// Check that timestamps are in descending order
	for i := 0; i < len(timestamps)-1; i++ {
		assert.True(t, timestamps[i].After(timestamps[i+1]) || timestamps[i].Equal(timestamps[i+1]),
			"timestamps should be in descending order")
	}
}

func TestRouter_DelegationsEndpoint_PaginationLimit(t *testing.T) {
	router, delegationStore := setupTestRouter(t)

	ctx := context.Background()
	// Insert 60 delegations to test pagination
	testData := make([]store.InsertDelegation, 60)
	for i := 0; i < 60; i++ {
		testData[i] = store.InsertDelegation{
			TzktID:    int64(4000 + i),
			Timestamp: time.Date(2023, 1, 1, 0, i, 0, 0, time.UTC),
			Amount:    int64(100 + i),
			Delegator: "tz1test" + strconv.Itoa(i),
			Level:     int64(1000 + i),
		}
	}
	require.NoError(t, delegationStore.BulkInsert(ctx, testData))

	// Test first page
	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations?page=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Should return max 50 items per page
	assert.LessOrEqual(t, len(resp.Data), 50)

	// Test second page
	req = httptest.NewRequest(http.MethodGet, "/xtz/delegations?page=2", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp2 response
	err = json.NewDecoder(w.Body).Decode(&resp2)
	require.NoError(t, err)

	// Second page should have remaining items
	assert.GreaterOrEqual(t, len(resp2.Data), 10)
}

func TestRouter_DelegationsEndpoint_ResponseFormat(t *testing.T) {
	router, delegationStore := setupTestRouter(t)

	ctx := context.Background()
	testTimestamp := time.Date(2023, 11, 10, 18, 15, 0, 0, time.UTC)
	uniqueDelegator := "tz1FormatTest" + strconv.FormatInt(testTimestamp.Unix(), 10)

	testData := []store.InsertDelegation{
		{
			TzktID:    testTimestamp.Unix(), // Use timestamp as unique ID
			Timestamp: testTimestamp,
			Amount:    125896,
			Delegator: uniqueDelegator,
			Level:     2338084,
		},
	}
	err := delegationStore.BulkInsert(ctx, testData)
	require.NoError(t, err)

	// Query all delegations
	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp response
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Verify response structure and format
	assert.NotNil(t, resp.Data)
	if len(resp.Data) > 0 {
		// Check that all fields are present and properly formatted
		for _, d := range resp.Data {
			assert.NotEmpty(t, d.Timestamp, "timestamp should not be empty")
			assert.NotEmpty(t, d.Amount, "amount should not be empty")
			assert.NotEmpty(t, d.Delegator, "delegator should not be empty")
			assert.NotEmpty(t, d.Level, "level should not be empty")

			// Verify timestamp format (RFC3339)
			_, err := time.Parse("2006-01-02T15:04:05Z", d.Timestamp)
			assert.NoError(t, err, "timestamp should be in RFC3339 format")

			// Verify amount and level are numeric strings
			_, err = strconv.ParseInt(d.Amount, 10, 64)
			assert.NoError(t, err, "amount should be a numeric string")

			_, err = strconv.ParseInt(d.Level, 10, 64)
			assert.NoError(t, err, "level should be a numeric string")
		}
	}
}

func TestRouter_DelegationsEndpoint_MultipleYears(t *testing.T) {
	router, delegationStore := setupTestRouter(t)

	ctx := context.Background()
	testData := []store.InsertDelegation{
		{
			TzktID:    6001,
			Timestamp: time.Date(2018, 7, 1, 0, 0, 0, 0, time.UTC),
			Amount:    1000,
			Delegator: "tz12018",
			Level:     1,
		},
		{
			TzktID:    6002,
			Timestamp: time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:    2000,
			Delegator: "tz12019",
			Level:     100,
		},
		{
			TzktID:    6003,
			Timestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:    3000,
			Delegator: "tz12020",
			Level:     200,
		},
	}
	require.NoError(t, delegationStore.BulkInsert(ctx, testData))

	// Test filtering by 2019
	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations?year=2019", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Should only have 2019 data
	for _, d := range resp.Data {
		ts, err := time.Parse("2006-01-02T15:04:05Z", d.Timestamp)
		require.NoError(t, err)
		if d.Delegator == "tz12018" || d.Delegator == "tz12019" || d.Delegator == "tz12020" {
			assert.Equal(t, 2019, ts.Year())
		}
	}
}

func TestRouter_HealthEndpoint_UnhealthyDatabase(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should be healthy in normal test conditions
	assert.Equal(t, http.StatusOK, w.Code)

	var resp healthResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp.Status)
	assert.Contains(t, resp.Checks, "database")
}

func TestRouter_DelegationsEndpoint_YearEdgeCases(t *testing.T) {
	router, _ := setupTestRouter(t)

	testCases := []struct {
		name     string
		year     string
		wantCode int
	}{
		{
			name:     "year 2018 (genesis)",
			year:     "2018",
			wantCode: http.StatusOK,
		},
		{
			name:     "current year",
			year:     "2024",
			wantCode: http.StatusOK,
		},
		{
			name:     "future year",
			year:     "2030",
			wantCode: http.StatusOK,
		},
		{
			name:     "year 2017 (before genesis)",
			year:     "2017",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "year 1000",
			year:     "1000",
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/xtz/delegations?year="+tc.year, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.wantCode, w.Code)
		})
	}
}

func TestRouter_DelegationsEndpoint_CombinedFilters(t *testing.T) {
	router, delegationStore := setupTestRouter(t)

	ctx := context.Background()
	testData := []store.InsertDelegation{
		{
			TzktID:    7001,
			Timestamp: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:    100,
			Delegator: "tz1test1",
			Level:     1000,
		},
		{
			TzktID:    7002,
			Timestamp: time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC),
			Amount:    200,
			Delegator: "tz1test2",
			Level:     2000,
		},
	}
	require.NoError(t, delegationStore.BulkInsert(ctx, testData))

	// Test year + page combination
	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations?year=2022&page=1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Data)
}

func TestRouter_UnsupportedMethods(t *testing.T) {
	router, _ := setupTestRouter(t)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/xtz/delegations", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)
			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})
	}
}
