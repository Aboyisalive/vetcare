package main

import (
	"net/http"
	"strings"
)

type vetInput struct {
	Name      string `json:"name"`
	Specialty string `json:"specialty"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

func (in *vetInput) validate() string {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return "name is required"
	}
	return ""
}

func (s *server) listVets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT v.id, v.name, v.specialty, v.email, v.phone,
		       CASE WHEN EXISTS (SELECT 1 FROM users u
		            WHERE u.vet_id = v.id AND u.is_admin = ` + s.db.boolTrue() + `)
		            THEN 1 ELSE 0 END,
		       (SELECT COUNT(*) FROM pets p WHERE p.vet_id = v.id),
		       (SELECT COUNT(*) FROM surgeries sg
		        WHERE sg.vet_id = v.id AND sg.status = 'scheduled'
		          AND sg.scheduled_at >= ` + s.db.nowExpr() + `)
		FROM vets v ORDER BY v.name`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	vets := []Vet{}
	for rows.Next() {
		var v Vet
		if err := rows.Scan(&v.ID, &v.Name, &v.Specialty, &v.Email, &v.Phone,
			&v.IsAdmin, &v.PatientCount, &v.UpcomingCount); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		vets = append(vets, v)
	}
	writeJSON(w, 200, vets)
}

type assignInput struct {
	VetID *int64 `json:"vet_id"`
}

// assignPet moves a patient onto a staff member's caseload (or clears the
// assignment with a null vet_id). Administrators only.
func (s *server) assignPet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var in assignInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if !s.vetExists(w, in.VetID) {
		return
	}
	res, err := s.db.Exec(`UPDATE pets SET vet_id = ? WHERE id = ?`, in.VetID, id)
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

func (s *server) createVet(w http.ResponseWriter, r *http.Request) {
	var in vetInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	id, err := s.db.insertID(`INSERT INTO vets (name, specialty, email, phone) VALUES (?,?,?,?)`,
		in.Name, in.Specialty, in.Email, in.Phone)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, Vet{ID: id, Name: in.Name, Specialty: in.Specialty, Email: in.Email, Phone: in.Phone})
}

func (s *server) updateVet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var in vetInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, 422, msg)
		return
	}
	res, err := s.db.Exec(`UPDATE vets SET name=?, specialty=?, email=?, phone=? WHERE id=?`,
		in.Name, in.Specialty, in.Email, in.Phone, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "vet not found")
		return
	}
	writeJSON(w, 200, Vet{ID: id, Name: in.Name, Specialty: in.Specialty, Email: in.Email, Phone: in.Phone})
}

func (s *server) deleteVet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	res, err := s.db.Exec(`DELETE FROM vets WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "vet not found")
		return
	}
	w.WriteHeader(204)
}
