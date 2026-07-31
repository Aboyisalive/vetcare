package main

import (
	"database/sql"
	"net/http"
	"strings"
)

type ownerInput struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}

func (in *ownerInput) validate() string {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return "name is required"
	}
	return ""
}

func (s *server) listOwners(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT o.id, o.name, o.email, o.phone, o.address, o.created_at,
		       (SELECT COUNT(*) FROM pets p WHERE p.owner_id = o.id)
		FROM owners o ORDER BY o.name`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	owners := []Owner{}
	for rows.Next() {
		var o Owner
		if err := rows.Scan(&o.ID, &o.Name, &o.Email, &o.Phone, &o.Address, &o.CreatedAt, &o.PetCount); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		owners = append(owners, o)
	}
	writeJSON(w, 200, owners)
}

func (s *server) getOwner(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if own, ok := isOwner(r); ok && id != own {
		writeErr(w, 403, "you can only view your own account")
		return
	}
	var o Owner
	err = s.db.QueryRow(`
		SELECT o.id, o.name, o.email, o.phone, o.address, o.created_at,
		       (SELECT COUNT(*) FROM pets p WHERE p.owner_id = o.id)
		FROM owners o WHERE o.id = ?`, id).
		Scan(&o.ID, &o.Name, &o.Email, &o.Phone, &o.Address, &o.CreatedAt, &o.PetCount)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "owner not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, o)
}

func (s *server) createOwner(w http.ResponseWriter, r *http.Request) {
	var in ownerInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	id, err := s.db.insertID(`INSERT INTO owners (name, email, phone, address) VALUES (?,?,?,?)`,
		in.Name, in.Email, in.Phone, in.Address)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.getOwnerByID(w, id, 201)
}

func (s *server) updateOwner(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if own, ok := isOwner(r); ok && id != own {
		writeErr(w, 403, "you can only edit your own account")
		return
	}
	var in ownerInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	res, err := s.db.Exec(`UPDATE owners SET name=?, email=?, phone=?, address=? WHERE id=?`,
		in.Name, in.Email, in.Phone, in.Address, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "owner not found")
		return
	}
	s.getOwnerByID(w, id, 200)
}

func (s *server) deleteOwner(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	res, err := s.db.Exec(`DELETE FROM owners WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "owner not found")
		return
	}
	w.WriteHeader(204)
}

func (s *server) getOwnerByID(w http.ResponseWriter, id int64, status int) {
	var o Owner
	err := s.db.QueryRow(`
		SELECT o.id, o.name, o.email, o.phone, o.address, o.created_at,
		       (SELECT COUNT(*) FROM pets p WHERE p.owner_id = o.id)
		FROM owners o WHERE o.id = ?`, id).
		Scan(&o.ID, &o.Name, &o.Email, &o.Phone, &o.Address, &o.CreatedAt, &o.PetCount)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, status, o)
}
