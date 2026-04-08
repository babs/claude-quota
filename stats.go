package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// statsBootstrapMaxAttempts caps how many times NewStatsStore retries on
// SQLITE_BUSY during the open + WAL switch + schema migration sequence.
// busy_timeout handles steady-state lock contention, but the brief window
// where two processes both try to bootstrap a fresh WAL DB (creating the
// -shm and -wal sidecar files) can race in ways busy_timeout doesn't cover.
// 10 attempts × 100-500ms backoff covers the race comfortably.
const statsBootstrapMaxAttempts = 10

// isSQLiteBusy reports whether err is (or wraps) a SQLite "database is
// locked" / SQLITE_BUSY error. Used by NewStatsStore's bootstrap retry.
// String matching is unfortunate but the modernc.org/sqlite driver doesn't
// expose typed error codes in a stable Go interface.
func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLITE_BUSY") || strings.Contains(s, "database is locked")
}

// sqlExecutor is the minimal contract migrations need: a connection-bound
// executor that runs context-aware Exec/QueryRow. Both *sql.Tx and *sql.Conn
// satisfy it, which lets applyMigration drive a pinned-connection BEGIN
// IMMEDIATE flow while keeping the migration bodies portable to test setups
// that use a transaction.
type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var statsDBPath string

const currentSchemaVersion = 2

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

// NewStatsStore opens (or creates) the stats database and initializes the
// schema. On a fresh DB with multiple writers (the dual-tray install path
// is the canonical case), the open + WAL bootstrap + migration sequence
// retries on SQLITE_BUSY so the loser of any race lands at a clean state.
func NewStatsStore() (*StatsStore, error) {
	dir := filepath.Dir(statsDBPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create stats dir %s: %w", dir, err)
	}

	var lastErr error
	for attempt := 0; attempt < statsBootstrapMaxAttempts; attempt++ {
		store, err := openStatsStore()
		if err == nil {
			return store, nil
		}
		lastErr = err
		if !isSQLiteBusy(err) {
			return nil, err
		}
		// Linear backoff with a small base. The race we're guarding against
		// (concurrent WAL bootstrap on a brand-new DB) resolves in ~ms; this
		// gives us 10 chances over ~3s total without flooding retries.
		time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
	}
	return nil, fmt.Errorf("stats bootstrap retried %d times: %w", statsBootstrapMaxAttempts, lastErr)
}

// openStatsStore performs the actual open + bootstrap, factored out so the
// retry loop in NewStatsStore can call it cleanly.
func openStatsStore() (*StatsStore, error) {
	// modernc.org/sqlite honors `_pragma=` query params at connection open
	// time, applying them to EVERY connection the pool creates. We need this
	// (rather than db.Exec("PRAGMA …")) because Go's database/sql pool may
	// hand out a different underlying connection for each call, and PRAGMAs
	// are per-connection. Setting busy_timeout via DSN guarantees every
	// connection waits up to 5s for write locks instead of failing
	// immediately with SQLITE_BUSY. The driver sorts busy_timeout first so
	// it's effective by the time journal_mode runs.
	dsn := "file:" + statsDBPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open stats db: %w", err)
	}

	if err := initSchema(context.Background(), db); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return &StatsStore{db: db}, nil
}

func initSchema(ctx context.Context, db *sql.DB) error {
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("unsupported stats schema version %d (max %d)", version, currentSchemaVersion)
	}
	for _, m := range schemaMigrations {
		if m.version <= version {
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
		version = m.version
	}
	return nil
}

type schemaMigration struct {
	version int
	name    string
	up      func(ctx context.Context, exec sqlExecutor) error
}

var schemaMigrations = []schemaMigration{
	{
		version: 1,
		name:    "create initial stats schema",
		up: func(ctx context.Context, exec sqlExecutor) error {
			_, err := exec.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS fetch_stats (
					id                         INTEGER PRIMARY KEY AUTOINCREMENT,
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
				return fmt.Errorf("create v1 schema: %w", err)
			}
			return nil
		},
	},
	{
		version: 2,
		name:    "add provider columns",
		up: func(ctx context.Context, exec sqlExecutor) error {
			for _, stmt := range []string{
				`ALTER TABLE fetch_stats ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude'`,
				`ALTER TABLE accounts ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude'`,
				`ALTER TABLE fetch_errors ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude'`,
			} {
				if _, err := exec.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("apply v2 schema: %w", err)
				}
			}
			return nil
		},
	},
}

// schemaVersion reads PRAGMA user_version. Accepts either *sql.DB or any
// connection-bound executor — useful so the locked re-check inside
// applyMigration shares the connection that holds the write lock.
func schemaVersion(ctx context.Context, src any) (int, error) {
	var version int
	var row *sql.Row
	switch s := src.(type) {
	case *sql.DB:
		row = s.QueryRowContext(ctx, "PRAGMA user_version")
	case sqlExecutor:
		row = s.QueryRowContext(ctx, "PRAGMA user_version")
	default:
		return 0, fmt.Errorf("schemaVersion: unsupported source type %T", src)
	}
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func setSchemaVersion(ctx context.Context, exec sqlExecutor, version int) error {
	if _, err := exec.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return fmt.Errorf("set schema version %d: %w", version, err)
	}
	return nil
}

// applyMigration runs a single schema migration under a write-locked
// transaction (BEGIN IMMEDIATE on a pinned connection). The locked re-check
// of user_version inside the transaction makes the entire flow safe against
// concurrent migrators on a fresh DB: a second process arriving here will
// block at BEGIN IMMEDIATE until the first commits, then read user_version
// from the now-current state and skip migrations already applied.
//
// Without this, two trays starting simultaneously on a fresh stats.db both
// see user_version=0 outside any transaction, both try to apply v2's
// non-idempotent ALTER TABLE ADD COLUMN, and the loser fails with
// "duplicate column".
func applyMigration(ctx context.Context, db *sql.DB, m schemaMigration) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for migration v%d (%s): %w", m.version, m.name, err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin immediate v%d (%s): %w", m.version, m.name, err)
	}
	committed := false
	defer func() {
		if !committed {
			// Best-effort rollback. Use a fresh context so a cancelled ctx
			// doesn't leave the connection in a half-committed state.
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	// Locked re-check: if another writer applied this migration while we
	// waited for the BEGIN IMMEDIATE lock, the version will already be at
	// or above m.version and there's nothing to do.
	current, err := schemaVersion(ctx, conn)
	if err != nil {
		return err
	}
	if current >= m.version {
		// Roll back the empty transaction explicitly so the connection is
		// returned to the pool in a clean state, then mark committed=true
		// to skip the deferred rollback.
		if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
			return fmt.Errorf("rollback no-op migration v%d: %w", m.version, err)
		}
		committed = true
		return nil
	}

	if err := m.up(ctx, conn); err != nil {
		return fmt.Errorf("migration v%d (%s): %w", m.version, m.name, err)
	}
	if err := setSchemaVersion(ctx, conn, m.version); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit migration v%d (%s): %w", m.version, m.name, err)
	}
	committed = true
	return nil
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
