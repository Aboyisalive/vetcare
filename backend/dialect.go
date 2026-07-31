package main

import (
	"database/sql"
	"strconv"
	"strings"
)

const (
	dialectSQLite   = "sqlite"
	dialectPostgres = "postgres"
)

// DB wraps *sql.DB with the small amount of dialect awareness the app needs:
// placeholder style, date/time expressions, upsert syntax and insert-returning-id.
// Handlers keep using Query/QueryRow/Exec exactly as before.
type DB struct {
	*sql.DB
	dialect string
}

func (db *DB) postgres() bool { return db.dialect == dialectPostgres }

// rebind converts SQLite's `?` / `?N` placeholders to PostgreSQL's `$N`.
// Text inside single-quoted literals is left untouched.
func rebind(dialect, q string) string {
	if dialect != dialectPostgres || !strings.ContainsRune(q, '?') {
		return q
	}
	var b strings.Builder
	b.Grow(len(q) + 8)
	next := 1
	inLiteral := false
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c == '\'' {
			inLiteral = !inLiteral
			b.WriteByte(c)
			continue
		}
		if c != '?' || inLiteral {
			b.WriteByte(c)
			continue
		}
		// `?12` keeps its explicit index; a bare `?` takes the next one.
		j := i + 1
		for j < len(q) && q[j] >= '0' && q[j] <= '9' {
			j++
		}
		if j > i+1 {
			b.WriteByte('$')
			b.WriteString(q[i+1 : j])
			i = j - 1
			continue
		}
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(next))
		next++
	}
	return b.String()
}

func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.DB.Query(rebind(db.dialect, query), args...)
}

func (db *DB) QueryRow(query string, args ...any) *sql.Row {
	return db.DB.QueryRow(rebind(db.dialect, query), args...)
}

func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	return db.DB.Exec(rebind(db.dialect, query), args...)
}

// Tx mirrors DB so transactional statements get the same rebinding.
type Tx struct {
	*sql.Tx
	dialect string
}

func (db *DB) Begin() (*Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, dialect: db.dialect}, nil
}

func (tx *Tx) Exec(query string, args ...any) (sql.Result, error) {
	return tx.Tx.Exec(rebind(tx.dialect, query), args...)
}

func (tx *Tx) QueryRow(query string, args ...any) *sql.Row {
	return tx.Tx.QueryRow(rebind(tx.dialect, query), args...)
}

// insertID runs an INSERT and returns the new row's id. PostgreSQL has no
// LastInsertId, so the statement gets a RETURNING clause instead.
func (db *DB) insertID(query string, args ...any) (int64, error) {
	if db.postgres() {
		var id int64
		err := db.QueryRow(query+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	res, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (tx *Tx) insertID(query string, args ...any) (int64, error) {
	if tx.dialect == dialectPostgres {
		var id int64
		err := tx.QueryRow(query+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	res, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ---------- Dialect-specific SQL fragments ----------
//
// Every date/time column in this schema is TEXT in 'YYYY-MM-DD' or
// 'YYYY-MM-DD HH:MM:SS' form, so comparisons stay plain string comparisons on
// both engines. These helpers produce that same text shape from each engine's
// clock, in UTC.

// nowExpr is the current UTC timestamp as 'YYYY-MM-DD HH:MM:SS'.
func (db *DB) nowExpr() string {
	if db.postgres() {
		return `to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD HH24:MI:SS')`
	}
	return `datetime('now')`
}

// todayExpr is the current UTC date as 'YYYY-MM-DD'.
func (db *DB) todayExpr() string {
	if db.postgres() {
		return `to_char(now() AT TIME ZONE 'utc', 'YYYY-MM-DD')`
	}
	return `date('now')`
}

// datePlusExpr is the UTC date `days` days from now as 'YYYY-MM-DD'.
func (db *DB) datePlusExpr(days int) string {
	d := strconv.Itoa(days)
	if db.postgres() {
		return `to_char((now() AT TIME ZONE 'utc') + (` + d + ` * interval '1 day'), 'YYYY-MM-DD')`
	}
	sign := "+"
	if days < 0 {
		sign = ""
	}
	return `date('now', '` + sign + d + ` days')`
}

// datePlusParamExpr is like datePlusExpr but takes the day count as a bound
// parameter, consuming one placeholder.
func (db *DB) datePlusParamExpr() string {
	if db.postgres() {
		return `to_char((now() AT TIME ZONE 'utc') + (CAST(? AS INTEGER) * interval '1 day'), 'YYYY-MM-DD')`
	}
	return `date('now', '+' || CAST(? AS INTEGER) || ' days')`
}

// boolTrue is the literal for a true boolean value. is_admin is INTEGER on
// SQLite and BOOLEAN on PostgreSQL, so `= 1` is not portable.
func (db *DB) boolTrue() string {
	if db.postgres() {
		return "TRUE"
	}
	return "1"
}

// insertIgnore builds an INSERT that silently skips rows violating a unique
// constraint.
func (db *DB) insertIgnore(body string) string {
	if db.postgres() {
		return "INSERT INTO " + body + " ON CONFLICT DO NOTHING"
	}
	return "INSERT OR IGNORE INTO " + body
}
