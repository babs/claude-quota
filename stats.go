package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

var statsDBPath string

func init() {
	dir, err := userDataDir()
	if err != nil {
		dir = "."
	}
	statsDBPath = filepath.Join(dir, "claude-quota", "stats.db")
}

// userDataDir returns the platform-appropriate directory for application data.
//   - Linux: $XDG_DATA_HOME or ~/.local/share
//   - macOS: ~/Library/Application Support
//   - Windows: %LocalAppData%
func userDataDir() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return dir, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	case "windows":
		if dir := os.Getenv("LocalAppData"); dir != "" {
			return dir, nil
		}
		return os.UserConfigDir()
	default:
		return os.UserConfigDir()
	}
}

// StatsStore records quota fetch data to a local SQLite database.
type StatsStore struct {
	db *sql.DB
}

// NewStatsStore opens (or creates) the stats database and initializes the schema.
func NewStatsStore() (*StatsStore, error) {
	dir := filepath.Dir(statsDBPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create stats dir %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", statsDBPath)
	if err != nil {
		return nil, fmt.Errorf("open stats db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return &StatsStore{db: db}, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS fetch_stats (
			id                         INTEGER PRIMARY KEY AUTOINCREMENT,
			provider                   TEXT NOT NULL DEFAULT 'claude',
			fetched_at                 INTEGER NOT NULL, -- unix timestamp UTC
			account_id                 TEXT,
			five_hour_util             REAL,
			five_hour_resets_at        INTEGER, -- unix timestamp UTC
			five_hour_projected        REAL,
			five_hour_saturation       INTEGER, -- unix timestamp UTC
			seven_day_util             REAL,
			seven_day_resets_at        INTEGER, -- unix timestamp UTC
			seven_day_projected        REAL,
			seven_day_saturation       INTEGER, -- unix timestamp UTC
			seven_day_sonnet_util      REAL,
			seven_day_sonnet_resets_at INTEGER  -- unix timestamp UTC
		);
		CREATE INDEX IF NOT EXISTS idx_fetch_stats_fetched_at
			ON fetch_stats (fetched_at);

		CREATE TABLE IF NOT EXISTS accounts (
			refresh_token_hash TEXT PRIMARY KEY,
			provider           TEXT NOT NULL DEFAULT 'claude',
			account_uuid       TEXT NOT NULL,
			email_address      TEXT,
			organization_uuid  TEXT,
			organization_name  TEXT,
			subscription_type  TEXT,
			rate_limit_tier    TEXT,
			created_at         INTEGER NOT NULL, -- unix timestamp UTC
			updated_at         INTEGER NOT NULL  -- unix timestamp UTC
		);

		CREATE TABLE IF NOT EXISTS fetch_errors (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			provider    TEXT NOT NULL DEFAULT 'claude',
			occurred_at INTEGER NOT NULL, -- unix timestamp UTC
			account_id  TEXT,
			error_type  TEXT NOT NULL,    -- credential, http, network, parse
			http_status INTEGER,          -- NULL when not HTTP error
			message     TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_fetch_errors_occurred_at
			ON fetch_errors (occurred_at);
	`)
	if err != nil {
		return err
	}
	return migrateSchema(db)
}

func migrateSchema(db *sql.DB) error {
	migrations := []struct {
		table  string
		column string
		sql    string
	}{
		{"fetch_stats", "provider", `ALTER TABLE fetch_stats ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude'`},
		{"accounts", "provider", `ALTER TABLE accounts ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude'`},
		{"fetch_errors", "provider", `ALTER TABLE fetch_errors ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude'`},
	}

	for _, m := range migrations {
		ok, err := columnExists(db, m.table, m.column)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migrate %s.%s: %w", m.table, m.column, err)
		}
	}
	return nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	// Safety: table names come from hardcoded migration literals, not user input.
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, fmt.Errorf("schema info for %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("scan schema info for %s: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// RecordFetch inserts a row from a successful quota fetch.
// Errors are logged but never returned — stat recording is best-effort.
func (s *StatsStore) RecordFetch(state QuotaState, accountID string) {
	var accountIDVal any
	if accountID != "" {
		accountIDVal = accountID
	}

	_, err := s.db.Exec(`
		INSERT INTO fetch_stats (
			provider, fetched_at, account_id,
			five_hour_util, five_hour_resets_at, five_hour_projected, five_hour_saturation,
			seven_day_util, seven_day_resets_at, seven_day_projected, seven_day_saturation,
			seven_day_sonnet_util, seven_day_sonnet_resets_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		defaultStatsProvider(state.Provider),
		timeToUnix(state.LastUpdate),
		accountIDVal,
		state.FiveHour,
		timeToUnix(state.FiveHourResets),
		state.FiveHourProjected,
		timeToUnix(state.FiveHourSaturation),
		state.SevenDay,
		timeToUnix(state.SevenDayResets),
		state.SevenDayProjected,
		timeToUnix(state.SevenDaySaturation),
		state.SevenDaySonnet,
		timeToUnix(state.SevenDaySonnetResets),
	)
	if err != nil {
		log.Printf("Failed to record fetch stats: %v", err)
	}
}

// LookupAccount returns cached account info by refresh token hash.
// On cache hit, updated_at is bumped. Returns false on miss.
func (s *StatsStore) LookupAccount(refreshTokenHash string) (AccountInfo, bool) {
	var info AccountInfo
	var email, orgUUID, orgName sql.NullString
	err := s.db.QueryRow(`
		SELECT account_uuid, email_address, organization_uuid, organization_name
		FROM accounts WHERE refresh_token_hash = ?`, refreshTokenHash,
	).Scan(&info.AccountUUID, &email, &orgUUID, &orgName)
	if err != nil {
		return AccountInfo{}, false
	}
	info.EmailAddress = email.String
	info.OrganizationUUID = orgUUID.String
	info.OrganizationName = orgName.String

	// Best-effort: bump updated_at on cache hit.
	_, _ = s.db.Exec(`UPDATE accounts SET updated_at = ? WHERE refresh_token_hash = ?`,
		time.Now().Unix(), refreshTokenHash)
	return info, true
}

// UpsertAccount inserts or replaces an account cache entry.
func (s *StatsStore) UpsertAccount(provider Provider, refreshTokenHash string, info AccountInfo, subType, rateLimitTier string) {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO accounts
			(refresh_token_hash, provider, account_uuid, email_address, organization_uuid, organization_name,
			 subscription_type, rate_limit_tier, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE((SELECT created_at FROM accounts WHERE refresh_token_hash = ?), ?), ?)`,
		refreshTokenHash, defaultStatsProvider(provider), info.AccountUUID, info.EmailAddress, info.OrganizationUUID, info.OrganizationName,
		subType, rateLimitTier, refreshTokenHash, now, now,
	)
	if err != nil {
		log.Printf("Failed to upsert account: %v", err)
	}
}

// RecordError inserts a fetch error row. Best-effort: logs on failure.
func (s *StatsStore) RecordError(provider Provider, accountID, errType string, httpStatus int, message string) {
	var accountIDVal any
	if accountID != "" {
		accountIDVal = accountID
	}
	var httpStatusVal any
	if httpStatus != 0 {
		httpStatusVal = httpStatus
	}
	_, err := s.db.Exec(`
		INSERT INTO fetch_errors (provider, occurred_at, account_id, error_type, http_status, message)
		VALUES (?, ?, ?, ?, ?, ?)`,
		defaultStatsProvider(provider), time.Now().Unix(), accountIDVal, errType, httpStatusVal, message,
	)
	if err != nil {
		log.Printf("Failed to record fetch error: %v", err)
	}
}

// Close closes the database connection.
func (s *StatsStore) Close() error {
	return s.db.Close()
}

// timeToUnix converts *time.Time to Unix timestamp (seconds) for SQL insertion.
// Returns nil (SQL NULL) for nil input.
func timeToUnix(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func defaultStatsProvider(provider Provider) string {
	if provider == "" {
		return string(ProviderClaude)
	}
	return string(provider)
}
