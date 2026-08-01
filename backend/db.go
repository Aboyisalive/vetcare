package main

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// resolveDSN decides which engine to use from DATABASE_URL, falling back to a
// local SQLite file. Recognised forms:
//
//	postgres://user:pass@host:5432/dbname?sslmode=require   -> PostgreSQL
//	postgresql://...                                        -> PostgreSQL
//	sqlite:///abs/path.db  |  sqlite://./rel.db  |  file:x   -> SQLite
//	/any/other/path.db                                       -> SQLite
func resolveDSN(databaseURL, sqlitePath string) (dialect, dsn string) {
	url := strings.TrimSpace(databaseURL)
	switch {
	case url == "":
		return dialectSQLite, sqliteDSN(sqlitePath)
	case strings.HasPrefix(url, "postgres://"), strings.HasPrefix(url, "postgresql://"):
		return dialectPostgres, url
	case strings.HasPrefix(url, "sqlite://"):
		return dialectSQLite, sqliteDSN(strings.TrimPrefix(url, "sqlite://"))
	case strings.HasPrefix(url, "file:"):
		return dialectSQLite, url
	default:
		return dialectSQLite, sqliteDSN(url)
	}
}

func sqliteDSN(path string) string {
	return path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}

func openDB(databaseURL, sqlitePath string) (*DB, error) {
	dialect, dsn := resolveDSN(databaseURL, sqlitePath)

	driver := "sqlite"
	if dialect == dialectPostgres {
		driver = "pgx"
	}
	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if dialect == dialectSQLite {
		// modernc.org/sqlite serializes writes; a single connection avoids
		// SQLITE_BUSY under concurrent handlers. PostgreSQL wants a real pool.
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	db := &DB{DB: sqlDB, dialect: dialect}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// describe returns a log-safe description of the connection (no credentials).
func describeDSN(dialect, dsn string) string {
	if dialect != dialectPostgres {
		return "sqlite: " + strings.SplitN(dsn, "?", 2)[0]
	}
	// Strip user:pass@ from postgres://user:pass@host/db for logging.
	rest := dsn
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		rest = strings.TrimPrefix(rest, prefix)
	}
	if at := strings.LastIndex(rest, "@"); at != -1 {
		rest = rest[at+1:]
	}
	return "postgres: " + strings.SplitN(rest, "?", 2)[0]
}

func (db *DB) migrate() error {
	schema := sqliteSchema
	if db.postgres() {
		schema = postgresSchema
	}
	if _, err := db.DB.Exec(schema); err != nil {
		return err
	}
	if err := db.addColumns(); err != nil {
		return err
	}
	if err := db.widenSurgeryStatus(); err != nil {
		return err
	}
	// Indexes on added columns must come after the ALTER TABLE above: on a
	// database that already has the table, CREATE TABLE IF NOT EXISTS is a
	// no-op, so the column only exists once addColumns has run.
	_, err := db.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_pets_vet ON pets(vet_id)`)
	return err
}

// addedColumn is a column introduced after the initial schema. CREATE TABLE IF
// NOT EXISTS is a no-op on a database that already has the table, so these are
// applied separately with ALTER TABLE.
type addedColumn struct {
	table  string
	column string
	sqlite string
	pg     string
}

var addedColumns = []addedColumn{
	// The clinic staff member a patient is assigned to.
	{
		table:  "pets",
		column: "vet_id",
		sqlite: `ALTER TABLE pets ADD COLUMN vet_id INTEGER REFERENCES vets(id) ON DELETE SET NULL`,
		pg:     `ALTER TABLE pets ADD COLUMN IF NOT EXISTS vet_id BIGINT REFERENCES vets(id) ON DELETE SET NULL`,
	},
	// Staff accounts with is_admin see the whole clinic, not just their own
	// patients and appointments.
	{
		table:  "users",
		column: "is_admin",
		sqlite: `ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`,
		pg:     `ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE`,
	},
}

// surgeryStatusCheck is the current allowed set for surgeries.status. Owner
// bookings introduced 'requested' and 'declined', so databases created before
// that carry a narrower CHECK constraint and must be widened.
const surgeryStatusCheck = `status IN ('requested','scheduled','completed','cancelled','declined')`

// widenSurgeryStatus brings an existing database's surgeries.status constraint
// up to date. Idempotent on both engines.
func (db *DB) widenSurgeryStatus() error {
	if db.postgres() {
		// Postgres names the inline CHECK surgeries_status_check; drop and
		// recreate it so the statement can be re-run safely.
		_, err := db.DB.Exec(`
			ALTER TABLE surgeries DROP CONSTRAINT IF EXISTS surgeries_status_check;
			ALTER TABLE surgeries ADD CONSTRAINT surgeries_status_check CHECK (` + surgeryStatusCheck + `)`)
		return err
	}
	// SQLite cannot alter a CHECK, so the table is rebuilt — but only when the
	// stored DDL is actually the old one.
	var ddl string
	err := db.DB.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='surgeries'`).Scan(&ddl)
	if err != nil || strings.Contains(ddl, "'requested'") {
		return nil // fresh database (already correct) or no such table yet
	}
	_, err = db.DB.Exec(`
		PRAGMA foreign_keys=off;
		BEGIN;
		CREATE TABLE surgeries_new (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			pet_id       INTEGER NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
			vet_id       INTEGER REFERENCES vets(id) ON DELETE SET NULL,
			procedure    TEXT NOT NULL,
			scheduled_at TEXT NOT NULL,
			duration_min INTEGER NOT NULL DEFAULT 60,
			status       TEXT NOT NULL DEFAULT 'scheduled' CHECK (` + surgeryStatusCheck + `),
			notes        TEXT NOT NULL DEFAULT '',
			created_at   TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO surgeries_new (id, pet_id, vet_id, procedure, scheduled_at, duration_min, status, notes, created_at)
			SELECT id, pet_id, vet_id, procedure, scheduled_at, duration_min, status, notes, created_at FROM surgeries;
		DROP TABLE surgeries;
		ALTER TABLE surgeries_new RENAME TO surgeries;
		CREATE INDEX IF NOT EXISTS idx_surgeries_pet ON surgeries(pet_id);
		CREATE INDEX IF NOT EXISTS idx_surgeries_time ON surgeries(scheduled_at);
		COMMIT;
		PRAGMA foreign_keys=on;`)
	return err
}

func (db *DB) addColumns() error {
	for _, c := range addedColumns {
		if db.postgres() {
			// ADD COLUMN IF NOT EXISTS makes the Postgres side idempotent.
			if _, err := db.DB.Exec(c.pg); err != nil {
				return fmt.Errorf("%s.%s: %w", c.table, c.column, err)
			}
			continue
		}
		has, err := db.sqliteHasColumn(c.table, c.column)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := db.DB.Exec(c.sqlite); err != nil {
			return fmt.Errorf("%s.%s: %w", c.table, c.column, err)
		}
	}
	return nil
}

// sqliteHasColumn reports whether table already has column. SQLite has no
// ADD COLUMN IF NOT EXISTS, so the check happens up front.
func (db *DB) sqliteHasColumn(table, column string) (bool, error) {
	rows, err := db.DB.Query(`SELECT 1 FROM pragma_table_info(?) WHERE name = ?`, table, column)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), rows.Err()
}

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS owners (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL,
	email      TEXT NOT NULL DEFAULT '',
	phone      TEXT NOT NULL DEFAULT '',
	address    TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS vets (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	name      TEXT NOT NULL,
	specialty TEXT NOT NULL DEFAULT '',
	email     TEXT NOT NULL DEFAULT '',
	phone     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS pets (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id     INTEGER NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
	vet_id       INTEGER REFERENCES vets(id) ON DELETE SET NULL,
	name         TEXT NOT NULL,
	species      TEXT NOT NULL,
	breed        TEXT NOT NULL DEFAULT '',
	sex          TEXT NOT NULL DEFAULT 'unknown' CHECK (sex IN ('male','female','unknown')),
	birth_date   TEXT NOT NULL DEFAULT '',
	weight_kg    REAL NOT NULL DEFAULT 0,
	microchip_id TEXT NOT NULL DEFAULT '',
	notes        TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_pets_owner ON pets(owner_id);

CREATE TABLE IF NOT EXISTS medical_records (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	pet_id     INTEGER NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
	vet_id     INTEGER REFERENCES vets(id) ON DELETE SET NULL,
	visit_date TEXT NOT NULL,
	category   TEXT NOT NULL DEFAULT 'exam' CHECK (category IN ('exam','injury','illness','dental','follow-up','other')),
	diagnosis  TEXT NOT NULL DEFAULT '',
	treatment  TEXT NOT NULL DEFAULT '',
	notes      TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_records_pet ON medical_records(pet_id);

CREATE TABLE IF NOT EXISTS surgeries (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	pet_id       INTEGER NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
	vet_id       INTEGER REFERENCES vets(id) ON DELETE SET NULL,
	procedure    TEXT NOT NULL,
	scheduled_at TEXT NOT NULL,
	duration_min INTEGER NOT NULL DEFAULT 60,
	status       TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('requested','scheduled','completed','cancelled','declined')),
	notes        TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_surgeries_pet ON surgeries(pet_id);
CREATE INDEX IF NOT EXISTS idx_surgeries_time ON surgeries(scheduled_at);

CREATE TABLE IF NOT EXISTS vaccinations (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	pet_id          INTEGER NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
	vet_id          INTEGER REFERENCES vets(id) ON DELETE SET NULL,
	vaccine         TEXT NOT NULL,
	administered_at TEXT NOT NULL,
	next_due        TEXT NOT NULL DEFAULT '',
	notes           TEXT NOT NULL DEFAULT '',
	created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_vaccinations_pet ON vaccinations(pet_id);
CREATE INDEX IF NOT EXISTS idx_vaccinations_due ON vaccinations(next_due);

CREATE TABLE IF NOT EXISTS reminders (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	vaccination_id INTEGER NOT NULL REFERENCES vaccinations(id) ON DELETE CASCADE,
	due_date       TEXT NOT NULL,
	status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed')),
	channel        TEXT NOT NULL DEFAULT 'log',
	message        TEXT NOT NULL DEFAULT '',
	sent_at        TEXT NOT NULL DEFAULT '',
	created_at     TEXT NOT NULL DEFAULT (datetime('now')),
	UNIQUE (vaccination_id, due_date)
);

CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	email         TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	role          TEXT NOT NULL CHECK (role IN ('owner','vet')),
	owner_id      INTEGER REFERENCES owners(id) ON DELETE CASCADE,
	vet_id        INTEGER REFERENCES vets(id) ON DELETE CASCADE,
	is_admin      INTEGER NOT NULL DEFAULT 0,
	created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
`

// postgresSchema mirrors sqliteSchema. Date columns stay TEXT in the same
// 'YYYY-MM-DD[ HH:MM:SS]' shape so ordering and comparisons behave identically
// on both engines.
const postgresSchema = `
CREATE TABLE IF NOT EXISTS owners (
	id         BIGSERIAL PRIMARY KEY,
	name       TEXT NOT NULL,
	email      TEXT NOT NULL DEFAULT '',
	phone      TEXT NOT NULL DEFAULT '',
	address    TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS')
);

CREATE TABLE IF NOT EXISTS vets (
	id        BIGSERIAL PRIMARY KEY,
	name      TEXT NOT NULL,
	specialty TEXT NOT NULL DEFAULT '',
	email     TEXT NOT NULL DEFAULT '',
	phone     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS pets (
	id           BIGSERIAL PRIMARY KEY,
	owner_id     BIGINT NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
	vet_id       BIGINT REFERENCES vets(id) ON DELETE SET NULL,
	name         TEXT NOT NULL,
	species      TEXT NOT NULL,
	breed        TEXT NOT NULL DEFAULT '',
	sex          TEXT NOT NULL DEFAULT 'unknown' CHECK (sex IN ('male','female','unknown')),
	birth_date   TEXT NOT NULL DEFAULT '',
	weight_kg    DOUBLE PRECISION NOT NULL DEFAULT 0,
	microchip_id TEXT NOT NULL DEFAULT '',
	notes        TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS')
);
CREATE INDEX IF NOT EXISTS idx_pets_owner ON pets(owner_id);

CREATE TABLE IF NOT EXISTS medical_records (
	id         BIGSERIAL PRIMARY KEY,
	pet_id     BIGINT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
	vet_id     BIGINT REFERENCES vets(id) ON DELETE SET NULL,
	visit_date TEXT NOT NULL,
	category   TEXT NOT NULL DEFAULT 'exam' CHECK (category IN ('exam','injury','illness','dental','follow-up','other')),
	diagnosis  TEXT NOT NULL DEFAULT '',
	treatment  TEXT NOT NULL DEFAULT '',
	notes      TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS')
);
CREATE INDEX IF NOT EXISTS idx_records_pet ON medical_records(pet_id);

CREATE TABLE IF NOT EXISTS surgeries (
	id           BIGSERIAL PRIMARY KEY,
	pet_id       BIGINT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
	vet_id       BIGINT REFERENCES vets(id) ON DELETE SET NULL,
	procedure    TEXT NOT NULL,
	scheduled_at TEXT NOT NULL,
	duration_min INTEGER NOT NULL DEFAULT 60,
	status       TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('requested','scheduled','completed','cancelled','declined')),
	notes        TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS')
);
CREATE INDEX IF NOT EXISTS idx_surgeries_pet ON surgeries(pet_id);
CREATE INDEX IF NOT EXISTS idx_surgeries_time ON surgeries(scheduled_at);

CREATE TABLE IF NOT EXISTS vaccinations (
	id              BIGSERIAL PRIMARY KEY,
	pet_id          BIGINT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
	vet_id          BIGINT REFERENCES vets(id) ON DELETE SET NULL,
	vaccine         TEXT NOT NULL,
	administered_at TEXT NOT NULL,
	next_due        TEXT NOT NULL DEFAULT '',
	notes           TEXT NOT NULL DEFAULT '',
	created_at      TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS')
);
CREATE INDEX IF NOT EXISTS idx_vaccinations_pet ON vaccinations(pet_id);
CREATE INDEX IF NOT EXISTS idx_vaccinations_due ON vaccinations(next_due);

CREATE TABLE IF NOT EXISTS reminders (
	id             BIGSERIAL PRIMARY KEY,
	vaccination_id BIGINT NOT NULL REFERENCES vaccinations(id) ON DELETE CASCADE,
	due_date       TEXT NOT NULL,
	status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed')),
	channel        TEXT NOT NULL DEFAULT 'log',
	message        TEXT NOT NULL DEFAULT '',
	sent_at        TEXT NOT NULL DEFAULT '',
	created_at     TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS'),
	UNIQUE (vaccination_id, due_date)
);

CREATE TABLE IF NOT EXISTS users (
	id            BIGSERIAL PRIMARY KEY,
	email         TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role          TEXT NOT NULL CHECK (role IN ('owner','vet')),
	owner_id      BIGINT REFERENCES owners(id) ON DELETE CASCADE,
	vet_id        BIGINT REFERENCES vets(id) ON DELETE CASCADE,
	is_admin      BOOLEAN NOT NULL DEFAULT FALSE,
	created_at    TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS')
);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS')
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
`
