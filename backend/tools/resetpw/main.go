// Command resetpw sets a user's password. It exists because password hashes are
// bcrypt and therefore one-way: a forgotten password can only be replaced.
//
//	go run ./tools/resetpw -match col               # list candidates, change nothing
//	go run ./tools/resetpw -email col@example.com -password 'new-password'
//
// DATABASE_URL must be set. Nothing is written unless -email and -password are
// both given, and the email has to match exactly one row.
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func main() {
	match := flag.String("match", "", "list users whose email contains this substring")
	email := flag.String("email", "", "exact email of the user to update")
	password := flag.String("password", "", "new plaintext password")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is empty; refusing to guess which database to open")
	}
	driver := "pgx"
	if strings.HasPrefix(dsn, "sqlite") {
		driver, dsn = "sqlite", strings.TrimPrefix(dsn, "sqlite://")
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()

	ph := func(n int) string {
		if driver == "sqlite" {
			return "?"
		}
		return fmt.Sprintf("$%d", n)
	}

	if *match != "" {
		// Match the email or the linked profile name: people remember the name
		// they typed into the app, not the address they signed up with.
		rows, err := db.Query(
			`SELECT u.id, u.email, u.role, u.is_admin,
			        COALESCE(o.name, v.name, '')
			   FROM users u
			   LEFT JOIN owners o ON o.id = u.owner_id
			   LEFT JOIN vets   v ON v.id = u.vet_id
			  WHERE u.email LIKE `+ph(1)+`
			     OR o.name  LIKE `+ph(2)+`
			     OR v.name  LIKE `+ph(3)+`
			  ORDER BY u.id`,
			"%"+*match+"%", "%"+*match+"%", "%"+*match+"%")
		if err != nil {
			log.Fatalf("query: %v", err)
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var id int64
			var mail, role, name string
			var admin bool
			if err := rows.Scan(&id, &mail, &role, &admin, &name); err != nil {
				log.Fatalf("scan: %v", err)
			}
			fmt.Printf("id=%d  email=%s  role=%s  admin=%t  name=%q\n", id, mail, role, admin, name)
			n++
		}
		if err := rows.Err(); err != nil {
			log.Fatalf("rows: %v", err)
		}
		fmt.Printf("%d match(es)\n", n)
		return
	}

	if *email == "" || *password == "" {
		log.Fatal("need -match to list, or both -email and -password to update")
	}

	var id int64
	err = db.QueryRow(`SELECT id FROM users WHERE email = `+ph(1), *email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		log.Fatalf("no user with email %q", *email)
	} else if err != nil {
		log.Fatalf("lookup: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}
	res, err := db.Exec(
		`UPDATE users SET password_hash = `+ph(1)+` WHERE id = `+ph(2), string(hash), id)
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	n, _ := res.RowsAffected()
	fmt.Printf("updated %d row(s) for id=%d (%s)\n", n, id, *email)
}
