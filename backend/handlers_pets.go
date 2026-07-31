package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

type petInput struct {
	OwnerID     int64   `json:"owner_id"`
	VetID       *int64  `json:"vet_id"`
	Name        string  `json:"name"`
	Species     string  `json:"species"`
	Breed       string  `json:"breed"`
	Sex         string  `json:"sex"`
	BirthDate   string  `json:"birth_date"`
	WeightKg    float64 `json:"weight_kg"`
	MicrochipID string  `json:"microchip_id"`
	Notes       string  `json:"notes"`
}

func (in *petInput) validate() string {
	in.Name = strings.TrimSpace(in.Name)
	in.Species = strings.TrimSpace(in.Species)
	if in.OwnerID <= 0 {
		return "owner_id is required"
	}
	if in.Name == "" {
		return "name is required"
	}
	if in.Species == "" {
		return "species is required"
	}
	if in.Sex == "" {
		in.Sex = "unknown"
	}
	if in.Sex != "male" && in.Sex != "female" && in.Sex != "unknown" {
		return "sex must be male, female or unknown"
	}
	if in.BirthDate != "" && !validDate(in.BirthDate) {
		return "birth_date must be YYYY-MM-DD"
	}
	if in.WeightKg < 0 {
		return "weight_kg must be >= 0"
	}
	return ""
}

const petSelect = `
	SELECT p.id, p.owner_id, o.name, p.vet_id, COALESCE(v.name, ''), p.name,
	       p.species, p.breed, p.sex, p.birth_date, p.weight_kg, p.microchip_id,
	       p.notes, p.created_at
	FROM pets p
	JOIN owners o ON o.id = p.owner_id
	LEFT JOIN vets v ON v.id = p.vet_id`

func scanPet(row interface{ Scan(...any) error }) (Pet, error) {
	var p Pet
	err := row.Scan(&p.ID, &p.OwnerID, &p.OwnerName, &p.VetID, &p.VetName, &p.Name,
		&p.Species, &p.Breed, &p.Sex, &p.BirthDate, &p.WeightKg, &p.MicrochipID,
		&p.Notes, &p.CreatedAt)
	return p, err
}

// petAssignedTo reports whether the pet exists and is assigned to the given
// vet. Writes a 404/403 response and returns false otherwise.
func (s *server) petAssignedTo(w http.ResponseWriter, petID, vetID int64) bool {
	var got *int64
	err := s.db.QueryRow(`SELECT vet_id FROM pets WHERE id = ?`, petID).Scan(&got)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "pet not found")
		return false
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return false
	}
	if got == nil || *got != vetID {
		writeErr(w, 403, "this patient is not assigned to you")
		return false
	}
	return true
}

// petInCaseload reports whether the vet may work on this patient: either the
// patient is assigned to them, or an administrator booked them an appointment
// for it. Chart access follows the work, so a vet who is scheduled to operate
// can always read and write that patient's history.
// Writes a 404/403 response and returns false otherwise.
func (s *server) petInCaseload(w http.ResponseWriter, petID, vetID int64) bool {
	var ok int
	err := s.db.QueryRow(`
		SELECT CASE WHEN p.vet_id = ? OR EXISTS (
		            SELECT 1 FROM surgeries sg WHERE sg.pet_id = p.id AND sg.vet_id = ?)
		       THEN 1 ELSE 0 END
		FROM pets p WHERE p.id = ?`, vetID, vetID, petID).Scan(&ok)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "pet not found")
		return false
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return false
	}
	if ok == 0 {
		writeErr(w, 403, "this patient is not assigned to you")
		return false
	}
	return true
}

func (s *server) listPets(w http.ResponseWriter, r *http.Request) {
	q := petSelect
	where := []string{}
	args := []any{}
	if own, ok := isOwner(r); ok {
		// Owner accounts only ever see their own pets.
		where = append(where, "p.owner_id = ?")
		args = append(args, own)
	} else if vetID, ok := vetScope(r); ok {
		// Non-admin staff only see the patients assigned to them.
		where = append(where, "p.vet_id = ?")
		args = append(args, vetID)
	}
	if ownerID := r.URL.Query().Get("owner_id"); ownerID != "" {
		id, err := strconv.ParseInt(ownerID, 10, 64)
		if err != nil {
			writeErr(w, 400, "invalid owner_id")
			return
		}
		where = append(where, "p.owner_id = ?")
		args = append(args, id)
	}
	// Admins can narrow the clinic-wide list to one staff member's caseload.
	if vetID := r.URL.Query().Get("vet_id"); vetID != "" && isAdmin(r) {
		if vetID == "unassigned" {
			where = append(where, "p.vet_id IS NULL")
		} else {
			id, err := strconv.ParseInt(vetID, 10, 64)
			if err != nil {
				writeErr(w, 400, "invalid vet_id")
				return
			}
			where = append(where, "p.vet_id = ?")
			args = append(args, id)
		}
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY p.name"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	pets := []Pet{}
	for rows.Next() {
		p, err := scanPet(rows)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		pets = append(pets, p)
	}
	writeJSON(w, 200, pets)
}

func (s *server) getPet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	p, err := scanPet(s.db.QueryRow(petSelect+" WHERE p.id = ?", id))
	if err == sql.ErrNoRows {
		writeErr(w, 404, "pet not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if own, ok := isOwner(r); ok && p.OwnerID != own {
		writeErr(w, 403, "this pet is not registered to your account")
		return
	}
	if vetID, ok := vetScope(r); ok && !s.petInCaseload(w, id, vetID) {
		return
	}
	writeJSON(w, 200, p)
}

func (s *server) createPet(w http.ResponseWriter, r *http.Request) {
	var in petInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if own, ok := isOwner(r); ok {
		in.OwnerID = own
		// Owners never choose the treating vet; the clinic assigns one.
		in.VetID = nil
	}
	if vetID, ok := vetScope(r); ok {
		// Non-admin staff can only register patients onto their own caseload,
		// otherwise they would immediately lose sight of the new patient.
		in.VetID = &vetID
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	if !s.rowExists(w, "owners", in.OwnerID, "owner") || !s.vetExists(w, in.VetID) {
		return
	}
	id, err := s.db.insertID(`
		INSERT INTO pets (owner_id, vet_id, name, species, breed, sex, birth_date, weight_kg, microchip_id, notes)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		in.OwnerID, in.VetID, in.Name, in.Species, in.Breed, in.Sex, in.BirthDate, in.WeightKg, in.MicrochipID, in.Notes)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	p, err := scanPet(s.db.QueryRow(petSelect+" WHERE p.id = ?", id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, p)
}

func (s *server) updatePet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var in petInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if own, ok := isOwner(r); ok {
		if !s.petOwnedBy(w, id, own) {
			return
		}
		in.OwnerID = own
		// Owners must not be able to reassign (or unassign) the treating vet.
		if err := s.db.QueryRow(`SELECT vet_id FROM pets WHERE id = ?`, id).Scan(&in.VetID); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
	}
	if vetID, ok := vetScope(r); ok {
		if !s.petAssignedTo(w, id, vetID) {
			return
		}
		// Reassigning a patient to another staff member is an admin action.
		in.VetID = &vetID
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	if !s.rowExists(w, "owners", in.OwnerID, "owner") || !s.vetExists(w, in.VetID) {
		return
	}
	res, err := s.db.Exec(`
		UPDATE pets SET owner_id=?, vet_id=?, name=?, species=?, breed=?, sex=?, birth_date=?,
		       weight_kg=?, microchip_id=?, notes=? WHERE id=?`,
		in.OwnerID, in.VetID, in.Name, in.Species, in.Breed, in.Sex, in.BirthDate, in.WeightKg,
		in.MicrochipID, in.Notes, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "pet not found")
		return
	}
	p, err := scanPet(s.db.QueryRow(petSelect+" WHERE p.id = ?", id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, p)
}

func (s *server) deletePet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if own, ok := isOwner(r); ok && !s.petOwnedBy(w, id, own) {
		return
	}
	if vetID, ok := vetScope(r); ok && !s.petAssignedTo(w, id, vetID) {
		return
	}
	res, err := s.db.Exec(`DELETE FROM pets WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "pet not found")
		return
	}
	w.WriteHeader(204)
}
