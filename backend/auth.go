package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie = "vetcare_session"
	sessionTTL    = 7 * 24 * time.Hour
)

type authUser struct {
	ID      int64
	Email   string
	Role    string
	OwnerID *int64
	VetID   *int64
	IsAdmin bool
}

type authPayload struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	IsAdmin bool   `json:"is_admin"`
	Owner   *Owner `json:"owner,omitempty"`
	Vet     *Vet   `json:"vet,omitempty"`
}

type ctxKey int

const userCtxKey ctxKey = 0

func userFrom(r *http.Request) *authUser {
	u, _ := r.Context().Value(userCtxKey).(*authUser)
	return u
}

// isOwner reports whether the request is from an owner-role user; ownerID is
// only valid when true.
func isOwner(r *http.Request) (ownerID int64, ok bool) {
	u := userFrom(r)
	if u == nil || u.Role != "owner" || u.OwnerID == nil {
		return 0, false
	}
	return *u.OwnerID, true
}

// isAdmin reports whether the request is from a clinic administrator, who sees
// every owner, patient and appointment in the clinic.
func isAdmin(r *http.Request) bool {
	u := userFrom(r)
	return u != nil && u.IsAdmin
}

// vetScope returns the vet a non-admin staff request is limited to: they only
// see patients and appointments assigned to them. ok is false for admins (no
// limit) and for owner accounts (scoped by isOwner instead).
//
// A staff account with no vet profile scopes to 0, which matches no rows —
// failing closed rather than exposing the whole clinic.
func vetScope(r *http.Request) (vetID int64, ok bool) {
	u := userFrom(r)
	if u == nil || u.Role != "vet" || u.IsAdmin {
		return 0, false
	}
	if u.VetID == nil {
		return 0, true
	}
	return *u.VetID, true
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *server) createSession(w http.ResponseWriter, userID int64) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(sessionTTL)
	_, err = s.db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?,?,?)`,
		token, userID, expires.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *server) userForRequest(r *http.Request) *authUser {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	var u authUser
	err = s.db.QueryRow(`
		SELECT u.id, u.email, u.role, u.owner_id, u.vet_id, u.is_admin
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > `+s.db.nowExpr(), c.Value).
		Scan(&u.ID, &u.Email, &u.Role, &u.OwnerID, &u.VetID, &u.IsAdmin)
	if err != nil {
		return nil
	}
	return &u
}

// requireAuth rejects unauthenticated requests and stores the user in context.
func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.userForRequest(r)
		if u == nil {
			writeErr(w, 401, "authentication required")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, u)))
	}
}

// requireVet additionally rejects non-vet users.
func (s *server) requireVet(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if userFrom(r).Role != "vet" {
			writeErr(w, 403, "veterinarian access required")
			return
		}
		next(w, r)
	})
}

// requireAdmin restricts a route to clinic administrators.
func (s *server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireVet(func(w http.ResponseWriter, r *http.Request) {
		if !isAdmin(r) {
			writeErr(w, 403, "clinic administrator access required")
			return
		}
		next(w, r)
	})
}

type signupInput struct {
	Role      string `json:"role"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
	Specialty string `json:"specialty"`
}

func (in *signupInput) validate() string {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Role != "owner" && in.Role != "vet" {
		return "role must be owner or vet"
	}
	if in.Name == "" {
		return "name is required"
	}
	if !strings.Contains(in.Email, "@") || len(in.Email) < 5 {
		return "a valid email is required"
	}
	if len(in.Password) < 8 {
		return "password must be at least 8 characters"
	}
	return ""
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	var in signupInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}

	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ?`, in.Email).Scan(&exists); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if exists > 0 {
		writeErr(w, 409, "an account with this email already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()

	var ownerID, vetID *int64
	if in.Role == "owner" {
		id, err := tx.insertID(`INSERT INTO owners (name, email, phone, address) VALUES (?,?,?,?)`,
			in.Name, in.Email, in.Phone, in.Address)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		ownerID = &id
	} else {
		id, err := tx.insertID(`INSERT INTO vets (name, specialty, email, phone) VALUES (?,?,?,?)`,
			in.Name, in.Specialty, in.Email, in.Phone)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		vetID = &id
	}
	userID, err := tx.insertID(`INSERT INTO users (email, password_hash, role, owner_id, vet_id) VALUES (?,?,?,?,?)`,
		in.Email, string(hash), in.Role, ownerID, vetID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	if err := s.createSession(w, userID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.writeAuthPayload(w, 201, &authUser{ID: userID, Email: in.Email, Role: in.Role, OwnerID: ownerID, VetID: vetID})
}

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in loginInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	var u authUser
	var hash string
	err := s.db.QueryRow(`SELECT id, email, password_hash, role, owner_id, vet_id, is_admin FROM users WHERE email = ?`, in.Email).
		Scan(&u.ID, &u.Email, &hash, &u.Role, &u.OwnerID, &u.VetID, &u.IsAdmin)
	if err == sql.ErrNoRows || (err == nil && bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil) {
		writeErr(w, 401, "invalid email or password")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := s.createSession(w, u.ID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.writeAuthPayload(w, 200, &u)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token = ?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(204)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	s.writeAuthPayload(w, 200, userFrom(r))
}

func (s *server) writeAuthPayload(w http.ResponseWriter, status int, u *authUser) {
	p := authPayload{ID: u.ID, Email: u.Email, Role: u.Role, IsAdmin: u.IsAdmin}
	if u.OwnerID != nil {
		var o Owner
		err := s.db.QueryRow(`
			SELECT o.id, o.name, o.email, o.phone, o.address, o.created_at,
			       (SELECT COUNT(*) FROM pets p WHERE p.owner_id = o.id)
			FROM owners o WHERE o.id = ?`, *u.OwnerID).
			Scan(&o.ID, &o.Name, &o.Email, &o.Phone, &o.Address, &o.CreatedAt, &o.PetCount)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		p.Owner = &o
	}
	if u.VetID != nil {
		var v Vet
		err := s.db.QueryRow(`SELECT id, name, specialty, email, phone FROM vets WHERE id = ?`, *u.VetID).
			Scan(&v.ID, &v.Name, &v.Specialty, &v.Email, &v.Phone)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		v.IsAdmin = u.IsAdmin
		p.Vet = &v
	}
	writeJSON(w, status, p)
}

// petOwnedBy reports whether the pet exists and belongs to the owner.
// Writes a 404/403 response and returns false otherwise.
func (s *server) petOwnedBy(w http.ResponseWriter, petID, ownerID int64) bool {
	var got int64
	err := s.db.QueryRow(`SELECT owner_id FROM pets WHERE id = ?`, petID).Scan(&got)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "pet not found")
		return false
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return false
	}
	if got != ownerID {
		writeErr(w, 403, "this pet is not registered to your account")
		return false
	}
	return true
}
