package main

import (
	"database/sql"
	"net/http"
)

type stats struct {
	Owners            int `json:"owners"`
	Pets              int `json:"pets"`
	Vets              int `json:"vets"`
	UpcomingSurgeries int `json:"upcoming_surgeries"`
	VaccinationsDue   int `json:"vaccinations_due_soon"`
	PendingReminders  int `json:"pending_reminders"`
}

func (s *server) getStats(w http.ResponseWriter, r *http.Request) {
	var st stats
	var row *sql.Row
	now, dueCutoff := s.db.nowExpr(), s.db.datePlusExpr(14)
	if own, ok := isOwner(r); ok {
		row = s.db.QueryRow(`
		SELECT
		 1,
		 (SELECT COUNT(*) FROM pets WHERE owner_id = ?1),
		 (SELECT COUNT(*) FROM vets),
		 (SELECT COUNT(*) FROM surgeries s JOIN pets p ON p.id = s.pet_id
		  WHERE p.owner_id = ?1 AND s.status = 'scheduled' AND s.scheduled_at >= `+now+`),
		 (SELECT COUNT(*) FROM vaccinations x JOIN pets p ON p.id = x.pet_id
		  WHERE p.owner_id = ?1 AND x.next_due != '' AND x.next_due <= `+dueCutoff+`),
		 (SELECT COUNT(*) FROM reminders r JOIN vaccinations x ON x.id = r.vaccination_id
		  JOIN pets p ON p.id = x.pet_id WHERE p.owner_id = ?1 AND r.status = 'pending')`, own)
	} else if vetID, ok := vetScope(r); ok {
		// Non-admin staff see their own caseload: owners of their patients,
		// their patients, their appointments, their patients' vaccinations.
		row = s.db.QueryRow(`
		SELECT
		 (SELECT COUNT(DISTINCT owner_id) FROM pets WHERE vet_id = ?1),
		 (SELECT COUNT(*) FROM pets WHERE vet_id = ?1),
		 (SELECT COUNT(*) FROM vets),
		 (SELECT COUNT(*) FROM surgeries
		  WHERE vet_id = ?1 AND status = 'scheduled' AND scheduled_at >= `+now+`),
		 (SELECT COUNT(*) FROM vaccinations x JOIN pets p ON p.id = x.pet_id
		  WHERE p.vet_id = ?1 AND x.next_due != '' AND x.next_due <= `+dueCutoff+`),
		 (SELECT COUNT(*) FROM reminders r JOIN vaccinations x ON x.id = r.vaccination_id
		  JOIN pets p ON p.id = x.pet_id WHERE p.vet_id = ?1 AND r.status = 'pending')`, vetID)
	} else {
		row = s.db.QueryRow(`
		SELECT
		 (SELECT COUNT(*) FROM owners),
		 (SELECT COUNT(*) FROM pets),
		 (SELECT COUNT(*) FROM vets),
		 (SELECT COUNT(*) FROM surgeries WHERE status = 'scheduled' AND scheduled_at >= ` + now + `),
		 (SELECT COUNT(*) FROM vaccinations WHERE next_due != '' AND next_due <= ` + dueCutoff + `),
		 (SELECT COUNT(*) FROM reminders WHERE status = 'pending')`)
	}
	if err := row.Scan(&st.Owners, &st.Pets, &st.Vets, &st.UpcomingSurgeries,
		&st.VaccinationsDue, &st.PendingReminders); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, st)
}
