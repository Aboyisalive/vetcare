package main

import (
	"net/http"
	"strconv"
	"time"
)

// Clinic booking rules. Owners may only request slots inside these hours, on a
// fixed grid, so the schedule stays tidy and easy to staff. Clinic staff are
// not restricted by them — emergencies do not respect opening hours.
const (
	clinicOpenHour  = 9  // first slot starts at 09:00
	clinicCloseHour = 17 // last slot must *end* by 17:00
	slotStepMin     = 30 // slots are offered every 30 minutes
	bookingHorizon  = 60 // days ahead an owner may request
)

// A vet's calendar is occupied by both 'scheduled' and 'requested' surgeries: a
// pending request holds its slot while it waits for the vet, so two owners
// cannot ask for the same time.

type slotVet struct {
	VetID     int64    `json:"vet_id"`
	VetName   string   `json:"vet_name"`
	Specialty string   `json:"specialty"`
	Slots     []string `json:"slots"` // "HH:MM", start times that fit the duration
}

type availabilityResponse struct {
	Date        string    `json:"date"`
	DurationMin int       `json:"duration_min"`
	Closed      bool      `json:"closed"` // clinic does not open that day
	Vets        []slotVet `json:"vets"`
}

// busyInterval is one occupied span on a vet's calendar.
type busyInterval struct {
	start time.Time
	end   time.Time
}

// eligibleVets returns the staff who may see this patient: its care team (the
// assigned vet plus anyone who has already treated it). A patient with no
// history yet can be booked with any member of staff.
func (s *server) eligibleVets(petID int64) ([]Vet, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.name, COALESCE(v.specialty, ''), COALESCE(v.email, ''), COALESCE(v.phone, '')
		FROM vets v
		WHERE EXISTS (SELECT 1 FROM pets p WHERE p.id = ? AND p.vet_id = v.id)
		   OR EXISTS (SELECT 1 FROM medical_records m WHERE m.pet_id = ? AND m.vet_id = v.id)
		   OR EXISTS (SELECT 1 FROM vaccinations x WHERE x.pet_id = ? AND x.vet_id = v.id)
		   OR EXISTS (SELECT 1 FROM surgeries sg WHERE sg.pet_id = ? AND sg.vet_id = v.id)
		ORDER BY v.name`, petID, petID, petID, petID)
	if err != nil {
		return nil, err
	}
	vets, err := scanVets(rows)
	if err != nil {
		return nil, err
	}
	if len(vets) > 0 {
		return vets, nil
	}
	rows, err = s.db.Query(`
		SELECT id, name, COALESCE(specialty, ''), COALESCE(email, ''), COALESCE(phone, '')
		FROM vets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	return scanVets(rows)
}

func scanVets(rows interface {
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
}) ([]Vet, error) {
	defer rows.Close()
	vets := []Vet{}
	for rows.Next() {
		var v Vet
		if err := rows.Scan(&v.ID, &v.Name, &v.Specialty, &v.Email, &v.Phone); err != nil {
			return nil, err
		}
		vets = append(vets, v)
	}
	return vets, rows.Err()
}

// busyFor loads the spans already taken on a vet's calendar for one day.
func (s *server) busyFor(vetID int64, date string) ([]busyInterval, error) {
	rows, err := s.db.Query(`
		SELECT scheduled_at, duration_min FROM surgeries
		WHERE vet_id = ? AND scheduled_at LIKE ? AND status IN ('scheduled','requested')`,
		vetID, date+"T%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	busy := []busyInterval{}
	for rows.Next() {
		var at string
		var dur int
		if err := rows.Scan(&at, &dur); err != nil {
			return nil, err
		}
		start, err := time.Parse("2006-01-02T15:04", at)
		if err != nil {
			continue
		}
		busy = append(busy, busyInterval{start, start.Add(time.Duration(dur) * time.Minute)})
	}
	return busy, rows.Err()
}

// clinicOpenOn reports whether the clinic takes bookings that day (Mon–Fri).
func clinicOpenOn(day time.Time) bool {
	wd := day.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

// freeSlots returns the start times on `date` where `duration` minutes fit
// inside opening hours without overlapping anything already booked. Slots in
// the past are dropped so today stays bookable but not retroactively.
func freeSlots(date string, duration int, busy []busyInterval, now time.Time) []string {
	day, err := time.Parse("2006-01-02", date)
	if err != nil || !clinicOpenOn(day) {
		return []string{}
	}
	slots := []string{}
	opens := day.Add(clinicOpenHour * time.Hour)
	closes := day.Add(clinicCloseHour * time.Hour)
	for t := opens; !t.Add(time.Duration(duration) * time.Minute).After(closes); t = t.Add(slotStepMin * time.Minute) {
		if t.Before(now) {
			continue
		}
		end := t.Add(time.Duration(duration) * time.Minute)
		free := true
		for _, b := range busy {
			if t.Before(b.end) && b.start.Before(end) {
				free = false
				break
			}
		}
		if free {
			slots = append(slots, t.Format("15:04"))
		}
	}
	return slots
}

// getAvailability answers "who can see my pet on this day, and when?" — the
// data an owner needs to request an appointment that cannot clash.
func (s *server) getAvailability(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	petID, err := strconv.ParseInt(q.Get("pet_id"), 10, 64)
	if err != nil || petID <= 0 {
		writeErr(w, 400, "pet_id is required")
		return
	}
	date := q.Get("date")
	if !validDate(date) {
		writeErr(w, 400, "date must be YYYY-MM-DD")
		return
	}
	duration := 30
	if d := q.Get("duration"); d != "" {
		duration, err = strconv.Atoi(d)
		if err != nil || duration <= 0 {
			writeErr(w, 400, "invalid duration")
			return
		}
	}

	// Owners may only look at their own pets; staff at patients they may treat.
	if own, ok := isOwner(r); ok {
		if !s.petOwnedBy(w, petID, own) {
			return
		}
	} else if vetID, ok := vetScope(r); ok {
		if !s.petInCaseload(w, petID, vetID) {
			return
		}
	} else if !s.rowExists(w, "pets", petID, "pet") {
		return
	}

	day, _ := time.Parse("2006-01-02", date)
	resp := availabilityResponse{Date: date, DurationMin: duration, Vets: []slotVet{}}
	if !clinicOpenOn(day) {
		resp.Closed = true
		writeJSON(w, 200, resp)
		return
	}

	vets, err := s.eligibleVets(petID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	now := time.Now()
	for _, v := range vets {
		busy, err := s.busyFor(v.ID, date)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		slots := freeSlots(date, duration, busy, now)
		if len(slots) == 0 {
			continue // fully booked (or past) — do not offer this vet
		}
		resp.Vets = append(resp.Vets, slotVet{
			VetID: v.ID, VetName: v.Name, Specialty: v.Specialty, Slots: slots,
		})
	}
	writeJSON(w, 200, resp)
}

// ownerSlotAllowed validates an owner's requested slot against the same rules
// the availability list is built from, so a hand-crafted request cannot book
// outside opening hours, in the past, or with a vet who may not treat the pet.
func (s *server) ownerSlotAllowed(petID int64, vetID *int64, scheduledAt string, duration int) string {
	if vetID == nil {
		return "choose the vet you would like to see"
	}
	start, err := time.Parse("2006-01-02T15:04", scheduledAt)
	if err != nil {
		return "scheduled_at must be YYYY-MM-DDTHH:MM"
	}
	now := time.Now()
	if start.Before(now) {
		return "that time has already passed"
	}
	if start.After(now.AddDate(0, 0, bookingHorizon)) {
		return "appointments can only be requested up to 60 days ahead"
	}
	if !clinicOpenOn(start) {
		return "the clinic is closed at weekends"
	}
	date := start.Format("2006-01-02")
	busy, err := s.busyFor(*vetID, date)
	if err != nil {
		return "could not check that vet's calendar"
	}
	wanted := start.Format("15:04")
	for _, slot := range freeSlots(date, duration, busy, now) {
		if slot == wanted {
			return ""
		}
	}
	// Distinguish "not on the grid / outside hours" from "already taken".
	for _, b := range busy {
		if start.Before(b.end) && b.start.Before(start.Add(time.Duration(duration)*time.Minute)) {
			return "that slot has just been taken — pick another time"
		}
	}
	return "pick one of the offered times inside clinic hours (09:00–17:00)"
}

// vetTreatsPet reports whether the vet is on the patient's care team, using the
// same rule as the availability list.
func (s *server) vetTreatsPet(petID, vetID int64) (bool, error) {
	vets, err := s.eligibleVets(petID)
	if err != nil {
		return false, err
	}
	for _, v := range vets {
		if v.ID == vetID {
			return true, nil
		}
	}
	return false, nil
}
