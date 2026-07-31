package main

import "testing"

func TestRebind(t *testing.T) {
	cases := []struct {
		name    string
		dialect string
		in      string
		want    string
	}{
		{
			name:    "sqlite untouched",
			dialect: dialectSQLite,
			in:      "SELECT * FROM pets WHERE owner_id = ? AND name = ?",
			want:    "SELECT * FROM pets WHERE owner_id = ? AND name = ?",
		},
		{
			name:    "sequential placeholders",
			dialect: dialectPostgres,
			in:      "SELECT * FROM pets WHERE owner_id = ? AND name = ?",
			want:    "SELECT * FROM pets WHERE owner_id = $1 AND name = $2",
		},
		{
			name:    "numbered placeholders keep their index",
			dialect: dialectPostgres,
			in:      "SELECT ?1, ?1, ?2",
			want:    "SELECT $1, $1, $2",
		},
		{
			name:    "question mark inside a literal is not a placeholder",
			dialect: dialectPostgres,
			in:      "SELECT 'why? ok' WHERE a = ?",
			want:    "SELECT 'why? ok' WHERE a = $1",
		},
		{
			// The reminder message builder contains an escaped quote ('''s ).
			// Placeholders after it must still be rebound.
			name:    "escaped quote inside literal",
			dialect: dialectPostgres,
			in:      "SELECT p.name || '''s ' || x.vaccine WHERE id = ?",
			want:    "SELECT p.name || '''s ' || x.vaccine WHERE id = $1",
		},
		{
			name:    "no placeholders",
			dialect: dialectPostgres,
			in:      "SELECT COUNT(*) FROM owners",
			want:    "SELECT COUNT(*) FROM owners",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rebind(tc.dialect, tc.in); got != tc.want {
				t.Errorf("rebind(%q)\n got: %s\nwant: %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestDateExprs(t *testing.T) {
	lite := &DB{dialect: dialectSQLite}
	pg := &DB{dialect: dialectPostgres}

	if got, want := lite.datePlusExpr(14), `date('now', '+14 days')`; got != want {
		t.Errorf("sqlite datePlusExpr(14) = %s, want %s", got, want)
	}
	if got, want := lite.datePlusExpr(-7), `date('now', '-7 days')`; got != want {
		t.Errorf("sqlite datePlusExpr(-7) = %s, want %s", got, want)
	}
	if got, want := lite.insertIgnore("users (id) VALUES (?)"), "INSERT OR IGNORE INTO users (id) VALUES (?)"; got != want {
		t.Errorf("sqlite insertIgnore = %s, want %s", got, want)
	}
	// PostgreSQL needs the conflict clause at the very end of the statement.
	if got, want := pg.insertIgnore("users (id) VALUES (?)"), "INSERT INTO users (id) VALUES (?) ON CONFLICT DO NOTHING"; got != want {
		t.Errorf("postgres insertIgnore = %s, want %s", got, want)
	}
	if pg.datePlusExpr(14) == lite.datePlusExpr(14) {
		t.Error("postgres datePlusExpr should differ from sqlite")
	}
}

func TestResolveDSN(t *testing.T) {
	cases := []struct {
		url, sqlitePath string
		wantDialect     string
		wantDSNPrefix   string
	}{
		{"", "vetcare.db", dialectSQLite, "vetcare.db?"},
		{"postgres://u:p@host:5432/db", "vetcare.db", dialectPostgres, "postgres://u:p@host:5432/db"},
		{"postgresql://u:p@host/db?sslmode=require", "vetcare.db", dialectPostgres, "postgresql://"},
		{"sqlite://./local.db", "vetcare.db", dialectSQLite, "./local.db?"},
		{"/var/data/vet.db", "vetcare.db", dialectSQLite, "/var/data/vet.db?"},
		{"  postgres://u@host/db  ", "vetcare.db", dialectPostgres, "postgres://u@host/db"},
	}
	for _, tc := range cases {
		dialect, dsn := resolveDSN(tc.url, tc.sqlitePath)
		if dialect != tc.wantDialect {
			t.Errorf("resolveDSN(%q) dialect = %s, want %s", tc.url, dialect, tc.wantDialect)
		}
		if len(dsn) < len(tc.wantDSNPrefix) || dsn[:len(tc.wantDSNPrefix)] != tc.wantDSNPrefix {
			t.Errorf("resolveDSN(%q) dsn = %q, want prefix %q", tc.url, dsn, tc.wantDSNPrefix)
		}
	}
}

func TestDescribeDSNHidesCredentials(t *testing.T) {
	got := describeDSN(dialectPostgres, "postgres://admin:s3cret@db.example.com:5432/vetcare?sslmode=require")
	if want := "postgres: db.example.com:5432/vetcare"; got != want {
		t.Errorf("describeDSN = %q, want %q", got, want)
	}
	for _, leak := range []string{"s3cret", "admin"} {
		if contains(got, leak) {
			t.Errorf("describeDSN leaked %q in %q", leak, got)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
