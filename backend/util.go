package main

import (
	"net/http"
	"time"
)

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func validDateTime(s string) bool {
	_, err := time.Parse("2006-01-02T15:04", s)
	return err == nil
}

// rowExists writes a 422 response and returns false when the referenced row
// is missing. table must be a trusted literal, never user input.
func (s *server) rowExists(w http.ResponseWriter, table string, id int64, label string) bool {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM `+table+` WHERE id = ?`, id).Scan(&one)
	if err != nil {
		writeErr(w, 422, label+" not found")
		return false
	}
	return true
}

// vetExists allows a nil vet_id (optional reference).
func (s *server) vetExists(w http.ResponseWriter, vetID *int64) bool {
	if vetID == nil {
		return true
	}
	return s.rowExists(w, "vets", *vetID, "vet")
}
