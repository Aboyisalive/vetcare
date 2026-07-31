package main

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type surgeryInput struct {
	PetID       int64  `json:"pet_id"`
	VetID       *int64 `json:"vet_id"`
	Procedure   string `json:"procedure"`
	ScheduledAt string `json:"scheduled_at"`
	DurationMin int    `json:"duration_min"`
	Status      string `json:"status"`
	Notes       string `json:"notes"`
}

func (in *surgeryInput) validate() string {
	in.Procedure = strings.TrimSpace(in.Procedure)
	if in.PetID <= 0 {
		return "pet_id is required"
	}
	if in.Procedure == "" {
		return "procedure is required"
	}
	if !validDateTime(in.ScheduledAt) {
		return "scheduled_at must be YYYY-MM-DDTHH:MM"
	}
	if in.DurationMin <= 0 {
		in.DurationMin = 60
	}
	if in.Status == "" {
		in.Status = "scheduled"
	}
	if in.Status != "scheduled" && in.Status != "completed" && in.Status != "cancelled" {
		return "status must be scheduled, completed or cancelled"
	}
	return ""
}

const surgerySelect = `
	SELECT s.id, s.pet_id, p.name, s.vet_id, COALESCE(v.name, ''), s.procedure,
	       s.scheduled_at, s.duration_min, s.status, s.notes, s.created_at
	FROM surgeries s
	JOIN pets p ON p.id = s.pet_id
	LEFT JOIN vets v ON v.id = s.vet_id`

func scanSurgery(row interface{ Scan(...any) error }) (Surgery, error) {
	var sg Surgery
	err := row.Scan(&sg.ID, &sg.PetID, &sg.PetName, &sg.VetID, &sg.VetName, &sg.Procedure,
		&sg.ScheduledAt, &sg.DurationMin, &sg.Status, &sg.Notes, &sg.CreatedAt)
	return sg, err
}

func (s *server) listSurgeries(w http.ResponseWriter, r *http.Request) {
	q := surgerySelect
	where := []string{}
	args := []any{}
	if own, ok := isOwner(r); ok {
		where = append(where, "p.owner_id = ?")
		args = append(args, own)
	}
	if vetID, ok := vetScope(r); ok {
		// Non-admin staff only see appointments assigned to them.
		where = append(where, "s.vet_id = ?")
		args = append(args, vetID)
	}
	// Admins can narrow the clinic-wide schedule to one staff member.
	if vetID := r.URL.Query().Get("vet_id"); vetID != "" && isAdmin(r) {
		if vetID == "unassigned" {
			where = append(where, "s.vet_id IS NULL")
		} else {
			where = append(where, "s.vet_id = ?")
			args = append(args, vetID)
		}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		where = append(where, "s.status = ?")
		args = append(args, status)
	}
	if petID := r.URL.Query().Get("pet_id"); petID != "" {
		where = append(where, "s.pet_id = ?")
		args = append(args, petID)
	}
	if from := r.URL.Query().Get("from"); from != "" {
		where = append(where, "s.scheduled_at >= ?")
		args = append(args, from)
	}
	if to := r.URL.Query().Get("to"); to != "" {
		where = append(where, "s.scheduled_at <= ?")
		args = append(args, to)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY s.scheduled_at"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	surgeries := []Surgery{}
	for rows.Next() {
		sg, err := scanSurgery(rows)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		surgeries = append(surgeries, sg)
	}
	writeJSON(w, 200, surgeries)
}

// vetConflict reports whether the vet already has a scheduled surgery
// overlapping [start, start+duration). excludeID skips the surgery being updated.
func (s *server) vetConflict(vetID *int64, scheduledAt string, durationMin int, excludeID int64) (bool, error) {
	if vetID == nil {
		return false, nil
	}
	start, err := time.Parse("2006-01-02T15:04", scheduledAt)
	if err != nil {
		return false, err
	}
	end := start.Add(time.Duration(durationMin) * time.Minute)
	rows, err := s.db.Query(`
		SELECT scheduled_at, duration_min FROM surgeries
		WHERE vet_id = ? AND status = 'scheduled' AND id != ?`, *vetID, excludeID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var at string
		var dur int
		if err := rows.Scan(&at, &dur); err != nil {
			return false, err
		}
		otherStart, err := time.Parse("2006-01-02T15:04", at)
		if err != nil {
			continue
		}
		otherEnd := otherStart.Add(time.Duration(dur) * time.Minute)
		if start.Before(otherEnd) && otherStart.Before(end) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *server) createSurgery(w http.ResponseWriter, r *http.Request) {
	var in surgeryInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	if vetID, ok := vetScope(r); ok {
		// Non-admin staff book onto their own calendar, for their own patients.
		in.VetID = &vetID
		if !s.petInCaseload(w, in.PetID, vetID) {
			return
		}
	}
	if !s.rowExists(w, "pets", in.PetID, "pet") || !s.vetExists(w, in.VetID) {
		return
	}
	if in.Status == "scheduled" {
		conflict, err := s.vetConflict(in.VetID, in.ScheduledAt, in.DurationMin, 0)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		if conflict {
			writeErr(w, 409, "vet already has a surgery scheduled in that time slot")
			return
		}
	}
	id, err := s.db.insertID(`
		INSERT INTO surgeries (pet_id, vet_id, procedure, scheduled_at, duration_min, status, notes)
		VALUES (?,?,?,?,?,?,?)`,
		in.PetID, in.VetID, in.Procedure, in.ScheduledAt, in.DurationMin, in.Status, in.Notes)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	sg, err := scanSurgery(s.db.QueryRow(surgerySelect+" WHERE s.id = ?", id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, sg)
}

func (s *server) updateSurgery(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var in surgeryInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	// Owners may only touch surgeries for their own pets (e.g. cancelling an
	// appointment); the pet on the surgery must stay theirs too.
	if own, ok := isOwner(r); ok {
		var petID int64
		if err := s.db.QueryRow(`SELECT pet_id FROM surgeries WHERE id = ?`, id).Scan(&petID); err != nil {
			writeErr(w, 404, "surgery not found")
			return
		}
		if !s.petOwnedBy(w, petID, own) || !s.petOwnedBy(w, in.PetID, own) {
			return
		}
	}
	// Non-admin staff may only touch their own appointments, and may not hand
	// one to another vet or move it onto a patient outside their caseload.
	if vetID, ok := vetScope(r); ok {
		if !s.surgeryAssignedTo(w, id, vetID) || !s.petInCaseload(w, in.PetID, vetID) {
			return
		}
		in.VetID = &vetID
	}
	if !s.rowExists(w, "pets", in.PetID, "pet") || !s.vetExists(w, in.VetID) {
		return
	}
	if in.Status == "scheduled" {
		conflict, err := s.vetConflict(in.VetID, in.ScheduledAt, in.DurationMin, id)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		if conflict {
			writeErr(w, 409, "vet already has a surgery scheduled in that time slot")
			return
		}
	}
	res, err := s.db.Exec(`
		UPDATE surgeries SET pet_id=?, vet_id=?, procedure=?, scheduled_at=?,
		       duration_min=?, status=?, notes=? WHERE id=?`,
		in.PetID, in.VetID, in.Procedure, in.ScheduledAt, in.DurationMin, in.Status, in.Notes, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "surgery not found")
		return
	}
	sg, err := scanSurgery(s.db.QueryRow(surgerySelect+" WHERE s.id = ?", id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, sg)
}

// surgeryAssignedTo reports whether the surgery exists and is assigned to the
// given vet. Writes a 404/403 response and returns false otherwise.
func (s *server) surgeryAssignedTo(w http.ResponseWriter, surgeryID, vetID int64) bool {
	var got *int64
	err := s.db.QueryRow(`SELECT vet_id FROM surgeries WHERE id = ?`, surgeryID).Scan(&got)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "surgery not found")
		return false
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return false
	}
	if got == nil || *got != vetID {
		writeErr(w, 403, "this appointment is not assigned to you")
		return false
	}
	return true
}

func (s *server) deleteSurgery(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if vetID, ok := vetScope(r); ok && !s.surgeryAssignedTo(w, id, vetID) {
		return
	}
	res, err := s.db.Exec(`DELETE FROM surgeries WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "surgery not found")
		return
	}
	w.WriteHeader(204)
}
