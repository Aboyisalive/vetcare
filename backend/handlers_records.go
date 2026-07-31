package main

import (
	"net/http"
	"strings"
)

type recordInput struct {
	PetID     int64  `json:"pet_id"`
	VetID     *int64 `json:"vet_id"`
	VisitDate string `json:"visit_date"`
	Category  string `json:"category"`
	Diagnosis string `json:"diagnosis"`
	Treatment string `json:"treatment"`
	Notes     string `json:"notes"`
}

var recordCategories = map[string]bool{
	"exam": true, "injury": true, "illness": true,
	"dental": true, "follow-up": true, "other": true,
}

func (in *recordInput) validate() string {
	if in.PetID <= 0 {
		return "pet_id is required"
	}
	if !validDate(in.VisitDate) {
		return "visit_date must be YYYY-MM-DD"
	}
	if in.Category == "" {
		in.Category = "exam"
	}
	if !recordCategories[in.Category] {
		return "invalid category"
	}
	in.Diagnosis = strings.TrimSpace(in.Diagnosis)
	return ""
}

const recordSelect = `
	SELECT m.id, m.pet_id, p.name, m.vet_id, COALESCE(v.name, ''), m.visit_date,
	       m.category, m.diagnosis, m.treatment, m.notes, m.created_at
	FROM medical_records m
	JOIN pets p ON p.id = m.pet_id
	LEFT JOIN vets v ON v.id = m.vet_id`

func scanRecord(row interface{ Scan(...any) error }) (MedicalRecord, error) {
	var m MedicalRecord
	err := row.Scan(&m.ID, &m.PetID, &m.PetName, &m.VetID, &m.VetName, &m.VisitDate,
		&m.Category, &m.Diagnosis, &m.Treatment, &m.Notes, &m.CreatedAt)
	return m, err
}

func (s *server) listPetRecords(w http.ResponseWriter, r *http.Request) {
	petID, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if !s.rowExists(w, "pets", petID, "pet") {
		return
	}
	if own, ok := isOwner(r); ok && !s.petOwnedBy(w, petID, own) {
		return
	}
	if vetID, ok := vetScope(r); ok && !s.petInCaseload(w, petID, vetID) {
		return
	}
	rows, err := s.db.Query(recordSelect+" WHERE m.pet_id = ? ORDER BY m.visit_date DESC, m.id DESC", petID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	records := []MedicalRecord{}
	for rows.Next() {
		m, err := scanRecord(rows)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		records = append(records, m)
	}
	writeJSON(w, 200, records)
}

func (s *server) createRecord(w http.ResponseWriter, r *http.Request) {
	var in recordInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	if vetID, ok := vetScope(r); ok && !s.petInCaseload(w, in.PetID, vetID) {
		return
	}
	if !s.rowExists(w, "pets", in.PetID, "pet") || !s.vetExists(w, in.VetID) {
		return
	}
	id, err := s.db.insertID(`
		INSERT INTO medical_records (pet_id, vet_id, visit_date, category, diagnosis, treatment, notes)
		VALUES (?,?,?,?,?,?,?)`,
		in.PetID, in.VetID, in.VisitDate, in.Category, in.Diagnosis, in.Treatment, in.Notes)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	m, err := scanRecord(s.db.QueryRow(recordSelect+" WHERE m.id = ?", id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, m)
}

func (s *server) updateRecord(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var in recordInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	if !s.rowExists(w, "pets", in.PetID, "pet") || !s.vetExists(w, in.VetID) {
		return
	}
	res, err := s.db.Exec(`
		UPDATE medical_records SET pet_id=?, vet_id=?, visit_date=?, category=?,
		       diagnosis=?, treatment=?, notes=? WHERE id=?`,
		in.PetID, in.VetID, in.VisitDate, in.Category, in.Diagnosis, in.Treatment, in.Notes, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "record not found")
		return
	}
	m, err := scanRecord(s.db.QueryRow(recordSelect+" WHERE m.id = ?", id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, m)
}

func (s *server) deleteRecord(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	res, err := s.db.Exec(`DELETE FROM medical_records WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "record not found")
		return
	}
	w.WriteHeader(204)
}
