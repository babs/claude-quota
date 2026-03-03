package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStatsStore(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, err := NewStatsStore()
	if err != nil {
		t.Fatalf("NewStatsStore() error: %v", err)
	}
	defer store.Close()

	var tableName string
	err = store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='fetch_stats'").Scan(&tableName)
	if err != nil {
		t.Fatalf("fetch_stats table not found: %v", err)
	}
	if tableName != "fetch_stats" {
		t.Errorf("expected table name fetch_stats, got %q", tableName)
	}
}

func TestRecordFetch_Full(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, err := NewStatsStore()
	if err != nil {
		t.Fatalf("NewStatsStore() error: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	resets5h := now.Add(3 * time.Hour)
	resets7d := now.Add(5 * 24 * time.Hour)
	sat5h := now.Add(2 * time.Hour)
	sat7d := now.Add(4 * 24 * time.Hour)
	fiveH := 42.5
	fiveHProj := 85.0
	sevenD := 73.0
	sevenDProj := 90.0
	sonnet := 10.0

	state := QuotaState{
		FiveHour:             &fiveH,
		FiveHourResets:       &resets5h,
		FiveHourProjected:    &fiveHProj,
		FiveHourSaturation:   &sat5h,
		SevenDay:             &sevenD,
		SevenDayResets:       &resets7d,
		SevenDayProjected:    &sevenDProj,
		SevenDaySaturation:   &sat7d,
		SevenDaySonnet:       &sonnet,
		SevenDaySonnetResets: &resets7d,
		LastUpdate:           &now,
	}

	store.RecordFetch(state, "account-123")

	var (
		fetchedAt    int64
		accountID    sql.NullString
		fiveHourUtil sql.NullFloat64
		resetsAt     sql.NullInt64
	)
	err = store.db.QueryRow("SELECT fetched_at, account_id, five_hour_util, five_hour_resets_at FROM fetch_stats WHERE id=1").
		Scan(&fetchedAt, &accountID, &fiveHourUtil, &resetsAt)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if fetchedAt != now.Unix() {
		t.Errorf("fetched_at = %d, want %d", fetchedAt, now.Unix())
	}
	if !accountID.Valid || accountID.String != "account-123" {
		t.Errorf("account_id = %v, want account-123", accountID)
	}
	if !fiveHourUtil.Valid || fiveHourUtil.Float64 != 42.5 {
		t.Errorf("five_hour_util = %v, want 42.5", fiveHourUtil)
	}
	if !resetsAt.Valid || resetsAt.Int64 != resets5h.Unix() {
		t.Errorf("five_hour_resets_at = %v, want %d", resetsAt, resets5h.Unix())
	}
}

func TestRecordFetch_NilFields(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, err := NewStatsStore()
	if err != nil {
		t.Fatalf("NewStatsStore() error: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	state := QuotaState{
		LastUpdate: &now,
	}

	store.RecordFetch(state, "")

	var (
		fiveHourUtil sql.NullFloat64
		accountID    sql.NullString
	)
	err = store.db.QueryRow("SELECT five_hour_util, account_id FROM fetch_stats WHERE id=1").
		Scan(&fiveHourUtil, &accountID)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if fiveHourUtil.Valid {
		t.Errorf("five_hour_util should be NULL, got %v", fiveHourUtil.Float64)
	}
	if accountID.Valid {
		t.Errorf("account_id should be NULL, got %v", accountID.String)
	}
}

func TestRecordFetch_MultipleRows(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, err := NewStatsStore()
	if err != nil {
		t.Fatalf("NewStatsStore() error: %v", err)
	}
	defer store.Close()

	for i := range 3 {
		now := time.Date(2026, 3, 1, 12, i, 0, 0, time.UTC)
		state := QuotaState{LastUpdate: &now}
		store.RecordFetch(state, "acct")
	}

	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM fetch_stats").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 3 {
		t.Errorf("row count = %d, want 3", count)
	}
}

func TestTimeToUnix(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got := timeToUnix(nil)
		if got != nil {
			t.Errorf("timeToUnix(nil) = %v, want nil", got)
		}
	})

	t.Run("value", func(t *testing.T) {
		ts := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		got := timeToUnix(&ts)
		want := ts.Unix()
		if got != want {
			t.Errorf("timeToUnix() = %v, want %v", got, want)
		}
	})
}

func newTestStore(t *testing.T) *StatsStore {
	t.Helper()
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	t.Cleanup(func() { statsDBPath = origPath })
	store, err := NewStatsStore()
	if err != nil {
		t.Fatalf("NewStatsStore() error: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestFetchErrorsTableExists(t *testing.T) {
	store := newTestStore(t)
	var name string
	err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='fetch_errors'").Scan(&name)
	if err != nil {
		t.Fatalf("fetch_errors table not found: %v", err)
	}
}

func TestAccountsTableExists(t *testing.T) {
	store := newTestStore(t)
	var name string
	err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='accounts'").Scan(&name)
	if err != nil {
		t.Fatalf("accounts table not found: %v", err)
	}
}

func TestRecordError_Full(t *testing.T) {
	store := newTestStore(t)
	store.RecordError("acct-uuid", "http", 401, "Token invalid")

	var (
		accountID  sql.NullString
		errType    string
		httpStatus sql.NullInt64
		message    sql.NullString
	)
	err := store.db.QueryRow("SELECT account_id, error_type, http_status, message FROM fetch_errors WHERE id=1").
		Scan(&accountID, &errType, &httpStatus, &message)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if !accountID.Valid || accountID.String != "acct-uuid" {
		t.Errorf("account_id = %v, want acct-uuid", accountID)
	}
	if errType != "http" {
		t.Errorf("error_type = %q, want http", errType)
	}
	if !httpStatus.Valid || httpStatus.Int64 != 401 {
		t.Errorf("http_status = %v, want 401", httpStatus)
	}
	if !message.Valid || message.String != "Token invalid" {
		t.Errorf("message = %v, want 'Token invalid'", message)
	}
}

func TestRecordError_NilOptionalFields(t *testing.T) {
	store := newTestStore(t)
	store.RecordError("", "network", 0, "connection refused")

	var (
		accountID  sql.NullString
		httpStatus sql.NullInt64
	)
	err := store.db.QueryRow("SELECT account_id, http_status FROM fetch_errors WHERE id=1").
		Scan(&accountID, &httpStatus)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if accountID.Valid {
		t.Errorf("account_id should be NULL, got %v", accountID.String)
	}
	if httpStatus.Valid {
		t.Errorf("http_status should be NULL, got %v", httpStatus.Int64)
	}
}

func TestLookupAccount_Miss(t *testing.T) {
	store := newTestStore(t)
	_, ok := store.LookupAccount("nonexistent-hash")
	if ok {
		t.Error("LookupAccount should return false for cache miss")
	}
}

func TestUpsertAccount(t *testing.T) {
	store := newTestStore(t)
	info := AccountInfo{
		AccountUUID:      "uuid-123",
		EmailAddress:     "test@example.com",
		OrganizationUUID: "org-456",
		OrganizationName: "Test Org",
	}
	store.UpsertAccount("hash-abc", info, "pro", "tier4")

	var (
		accountUUID   string
		email         sql.NullString
		subType       sql.NullString
		rateLimitTier sql.NullString
	)
	err := store.db.QueryRow(`
		SELECT account_uuid, email_address, subscription_type, rate_limit_tier
		FROM accounts WHERE refresh_token_hash = 'hash-abc'`,
	).Scan(&accountUUID, &email, &subType, &rateLimitTier)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if accountUUID != "uuid-123" {
		t.Errorf("account_uuid = %q, want uuid-123", accountUUID)
	}
	if !email.Valid || email.String != "test@example.com" {
		t.Errorf("email_address = %v, want test@example.com", email)
	}
	if !subType.Valid || subType.String != "pro" {
		t.Errorf("subscription_type = %v, want pro", subType)
	}
}

func TestLookupAccount_Hit(t *testing.T) {
	store := newTestStore(t)
	info := AccountInfo{
		AccountUUID:      "uuid-123",
		EmailAddress:     "test@example.com",
		OrganizationUUID: "org-456",
		OrganizationName: "Test Org",
	}
	store.UpsertAccount("hash-abc", info, "pro", "tier4")

	got, ok := store.LookupAccount("hash-abc")
	if !ok {
		t.Fatal("LookupAccount should return true for cache hit")
	}
	if got.AccountUUID != "uuid-123" {
		t.Errorf("AccountUUID = %q, want uuid-123", got.AccountUUID)
	}
	if got.EmailAddress != "test@example.com" {
		t.Errorf("EmailAddress = %q, want test@example.com", got.EmailAddress)
	}
	if got.OrganizationName != "Test Org" {
		t.Errorf("OrganizationName = %q, want 'Test Org'", got.OrganizationName)
	}
}

func TestLookupAccount_BumpsUpdatedAt(t *testing.T) {
	store := newTestStore(t)
	info := AccountInfo{AccountUUID: "uuid-123"}
	store.UpsertAccount("hash-abc", info, "", "")

	// Read initial updated_at.
	var updatedBefore int64
	store.db.QueryRow("SELECT updated_at FROM accounts WHERE refresh_token_hash = 'hash-abc'").Scan(&updatedBefore)

	// Wait a tiny bit to ensure timestamp changes.
	time.Sleep(1100 * time.Millisecond)

	store.LookupAccount("hash-abc")

	var updatedAfter int64
	store.db.QueryRow("SELECT updated_at FROM accounts WHERE refresh_token_hash = 'hash-abc'").Scan(&updatedAfter)
	if updatedAfter <= updatedBefore {
		t.Errorf("updated_at should have been bumped: before=%d, after=%d", updatedBefore, updatedAfter)
	}
}
