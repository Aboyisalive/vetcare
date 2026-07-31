package main

import (
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// reminderLeadDays: create a reminder when a vaccination's next_due falls
// within this many days (overdue ones included).
const reminderLeadDays = 14

const reminderSelect = `
	SELECT r.id, r.vaccination_id, x.vaccine, p.name, o.name, o.email,
	       r.due_date, r.status, r.channel, r.message, r.sent_at, r.created_at
	FROM reminders r
	JOIN vaccinations x ON x.id = r.vaccination_id
	JOIN pets p ON p.id = x.pet_id
	JOIN owners o ON o.id = p.owner_id`

func scanReminder(row interface{ Scan(...any) error }) (Reminder, error) {
	var rm Reminder
	err := row.Scan(&rm.ID, &rm.VaccinationID, &rm.Vaccine, &rm.PetName, &rm.OwnerName,
		&rm.OwnerEmail, &rm.DueDate, &rm.Status, &rm.Channel, &rm.Message, &rm.SentAt, &rm.CreatedAt)
	return rm, err
}

func (s *server) listReminders(w http.ResponseWriter, r *http.Request) {
	q := reminderSelect
	where := []string{}
	args := []any{}
	if own, ok := isOwner(r); ok {
		where = append(where, "o.id = ?")
		args = append(args, own)
	}
	if vetID, ok := vetScope(r); ok {
		// Non-admin staff only see reminders for their own patients.
		where = append(where, "p.vet_id = ?")
		args = append(args, vetID)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		where = append(where, "r.status = ?")
		args = append(args, status)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY r.due_date, r.id"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	reminders := []Reminder{}
	for rows.Next() {
		rm, err := scanReminder(rows)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		reminders = append(reminders, rm)
	}
	writeJSON(w, 200, reminders)
}

// runReminders is the manual trigger endpoint: POST /api/reminders/run
func (s *server) runReminders(w http.ResponseWriter, r *http.Request) {
	created, sent, err := s.processReminders()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]int{"created": created, "sent": sent})
}

func (s *server) deleteReminder(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	res, err := s.db.Exec(`DELETE FROM reminders WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "reminder not found")
		return
	}
	w.WriteHeader(204)
}

// startReminderWorker runs processReminders on an interval until the server stops.
func (s *server) startReminderWorker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if created, sent, err := s.processReminders(); err != nil {
				log.Printf("reminder worker: %v", err)
			} else if created > 0 || sent > 0 {
				log.Printf("reminder worker: created %d, sent %d", created, sent)
			}
			<-ticker.C
		}
	}()
}

// processReminders creates pending reminders for vaccinations due within the
// lead window, then attempts delivery of every pending reminder.
func (s *server) processReminders() (created, sent int, err error) {
	res, err := s.db.Exec(s.db.insertIgnore(`reminders (vaccination_id, due_date, message)
		SELECT x.id, x.next_due,
		       'Hello ' || o.name || ', ' || p.name || '''s ' || x.vaccine ||
		       ' vaccination is due on ' || x.next_due || '. Please contact the clinic to book an appointment.'
		FROM vaccinations x
		JOIN pets p ON p.id = x.pet_id
		JOIN owners o ON o.id = p.owner_id
		WHERE x.next_due != ''
		  AND x.next_due <= ` + s.db.datePlusExpr(reminderLeadDays)))
	if err != nil {
		return 0, 0, fmt.Errorf("create reminders: %w", err)
	}
	n, _ := res.RowsAffected()
	created = int(n)

	rows, err := s.db.Query(reminderSelect + ` WHERE r.status = 'pending'`)
	if err != nil {
		return created, 0, err
	}
	pending := []Reminder{}
	for rows.Next() {
		rm, err := scanReminder(rows)
		if err != nil {
			rows.Close()
			return created, 0, err
		}
		pending = append(pending, rm)
	}
	rows.Close()

	for _, rm := range pending {
		channel, sendErr := s.mailer.send(rm.OwnerEmail,
			fmt.Sprintf("Vaccination reminder for %s", rm.PetName), rm.Message)
		status := "sent"
		if sendErr != nil {
			status = "failed"
			log.Printf("reminder %d: send failed: %v", rm.ID, sendErr)
		}
		_, err := s.db.Exec(`UPDATE reminders SET status=?, channel=?, sent_at=`+s.db.nowExpr()+` WHERE id=?`,
			status, channel, rm.ID)
		if err != nil {
			return created, sent, err
		}
		if sendErr == nil {
			sent++
		}
	}
	return created, sent, nil
}

// mailer sends via SMTP when configured through env vars, otherwise logs.
type mailer struct {
	host, port, user, pass, from string
}

func mailerFromEnv() *mailer {
	return &mailer{
		host: os.Getenv("SMTP_HOST"),
		port: envOr("SMTP_PORT", "587"),
		user: os.Getenv("SMTP_USER"),
		pass: os.Getenv("SMTP_PASS"),
		from: envOr("SMTP_FROM", "noreply@vetcare.local"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// send returns the channel used ("email" or "log").
func (m *mailer) send(to, subject, body string) (string, error) {
	if m.host == "" || to == "" {
		log.Printf("REMINDER (log channel) to=%q subject=%q body=%q", to, subject, body)
		return "log", nil
	}
	msg := strings.Join([]string{
		"From: " + m.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")
	var auth smtp.Auth
	if m.user != "" {
		auth = smtp.PlainAuth("", m.user, m.pass, m.host)
	}
	err := smtp.SendMail(m.host+":"+m.port, auth, m.from, []string{to}, []byte(msg))
	if err != nil {
		return "email", err
	}
	return "email", nil
}
