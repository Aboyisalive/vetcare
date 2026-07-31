package main

import (
	"net/http"
	"strings"
)

type vaccinationInput struct {
	PetID          int64  `json:"pet_id"`
	VetID          *int64 `json:"vet_id"`
	Vaccine        string `json:"vaccine"`
	AdministeredAt string `json:"administered_at"`
	NextDue        string `json:"next_due"`
	Notes          string `json:"notes"`
}

func (in *vaccinationInput) validate() string {
	in.Vaccine = strings.TrimSpace(in.Vaccine)
	if in.PetID <= 0 {
		return "pet_id is required"
	}
	if in.Vaccine == "" {
		return "vaccine is required"
	}
	if !validDate(in.AdministeredAt) {
		return "administered_at must be YYYY-MM-DD"
	}
	if in.NextDue != "" && !validDate(in.NextDue) {
		return "next_due must be YYYY-MM-DD or empty"
	}
	return ""
}

const vaccinationSelect = `
	SELECT x.id, x.pet_id, p.name, x.vet_id, COALESCE(v.name, ''), x.vaccine,
	       x.administered_at, x.next_due, x.notes, x.created_at
	FROM vaccinations x
	JOIN pets p ON p.id = x.pet_id
	LEFT JOIN vets v ON v.id = x.vet_id`

func scanVaccination(row interface{ Scan(...any) error }) (Vaccination, error) {
	var vx Vaccination
	err := row.Scan(&vx.ID, &vx.PetID, &vx.PetName, &vx.VetID, &vx.VetName, &vx.Vaccine,
		&vx.AdministeredAt, &vx.NextDue, &vx.Notes, &vx.CreatedAt)
	return vx, err
}

func (s *server) listVaccinations(w http.ResponseWriter, r *http.Request) {
	q := vaccinationSelect
	where := []string{}
	args := []any{}
	if own, ok := isOwner(r); ok {
		where = append(where, "p.owner_id = ?")
		args = append(args, own)
	}
	if vetID, ok := vetScope(r); ok {
		// Non-admin staff only see vaccinations for their own patients.
		where = append(where, "p.vet_id = ?")
		args = append(args, vetID)
	}
	if petID := r.URL.Query().Get("pet_id"); petID != "" {
		where = append(where, "x.pet_id = ?")
		args = append(args, petID)
	}
	// due_within=N filters to vaccinations due in the next N days (or overdue).
	if days := r.URL.Query().Get("due_within"); days != "" {
		where = append(where, "x.next_due != '' AND x.next_due <= "+s.db.datePlusParamExpr())
		args = append(args, days)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY CASE WHEN x.next_due = '' THEN 1 ELSE 0 END, x.next_due, x.id"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	vaccinations := []Vaccination{}
	for rows.Next() {
		vx, err := scanVaccination(rows)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		vaccinations = append(vaccinations, vx)
	}
	writeJSON(w, 200, vaccinations)
}

func (s *server) createVaccination(w http.ResponseWriter, r *http.Request) {
	var in vaccinationInput
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
		INSERT INTO vaccinations (pet_id, vet_id, vaccine, administered_at, next_due, notes)
		VALUES (?,?,?,?,?,?)`,
		in.PetID, in.VetID, in.Vaccine, in.AdministeredAt, in.NextDue, in.Notes)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	vx, err := scanVaccination(s.db.QueryRow(vaccinationSelect+" WHERE x.id = ?", id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, vx)
}

func (s *server) updateVaccination(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var in vaccinationInput
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
		UPDATE vaccinations SET pet_id=?, vet_id=?, vaccine=?, administered_at=?,
		       next_due=?, notes=? WHERE id=?`,
		in.PetID, in.VetID, in.Vaccine, in.AdministeredAt, in.NextDue, in.Notes, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "vaccination not found")
		return
	}
	vx, err := scanVaccination(s.db.QueryRow(vaccinationSelect+" WHERE x.id = ?", id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, vx)
}

func (s *server) deleteVaccination(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	res, err := s.db.Exec(`DELETE FROM vaccinations WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "vaccination not found")
		return
	}
	w.WriteHeader(204)
}
