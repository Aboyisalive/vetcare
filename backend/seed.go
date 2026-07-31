package main

import (
	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

// seedData inserts sample rows for development. Idempotent-ish: skips base
// data when owners already exist, but always backfills demo user accounts
// (password "password123") for the sample owners and vets.
func seedData(db *DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM owners`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return seedUsers(db)
	}
	stmts := []string{
		`INSERT INTO owners (name, email, phone, address) VALUES
		 ('Alice Nguyen', 'alice@example.com', '555-0101', '12 Maple St'),
		 ('Bruno Costa', 'bruno@example.com', '555-0102', '34 Oak Ave'),
		 ('Carmen Diaz', 'carmen@example.com', '555-0103', '56 Pine Rd')`,
		`INSERT INTO vets (name, specialty, email, phone) VALUES
		 ('Dr. Sarah Kim', 'Surgery', 'skim@vetcare.local', '555-0201'),
		 ('Dr. James Patel', 'General', 'jpatel@vetcare.local', '555-0202')`,
		// vet_id is the staff member each patient is assigned to.
		`INSERT INTO pets (owner_id, vet_id, name, species, breed, sex, birth_date, weight_kg, microchip_id) VALUES
		 (1, 1, 'Rex', 'dog', 'German Shepherd', 'male', '2021-03-14', 32.5, 'CHIP-0001'),
		 (1, 2, 'Misty', 'cat', 'Siamese', 'female', '2022-07-01', 4.2, 'CHIP-0002'),
		 (2, 1, 'Bolt', 'dog', 'Border Collie', 'male', '2020-11-20', 19.8, 'CHIP-0003'),
		 (3, 2, 'Coco', 'rabbit', 'Holland Lop', 'female', '2023-01-05', 1.6, '')`,
		`INSERT INTO medical_records (pet_id, vet_id, visit_date, category, diagnosis, treatment) VALUES
		 (1, 2, '2026-05-10', 'exam', 'Annual checkup, healthy', 'None'),
		 (2, 2, '2026-06-02', 'illness', 'Mild ear infection', 'Ear drops, 7 days'),
		 (3, 1, '2026-04-18', 'injury', 'Sprained hind leg', 'Rest, anti-inflammatory')`,
		`INSERT INTO surgeries (pet_id, vet_id, procedure, scheduled_at, duration_min, status) VALUES
		 (3, 1, 'ACL repair', '2026-08-12T09:00', 120, 'scheduled'),
		 (2, 1, 'Dental cleaning', '2026-08-05T14:00', 45, 'scheduled'),
		 (1, 1, 'Neutering', '2026-02-20T10:00', 60, 'completed')`,
		`INSERT INTO vaccinations (pet_id, vet_id, vaccine, administered_at, next_due) VALUES
		 (1, 2, 'Rabies', '2025-08-01', '2026-08-01'),
		 (1, 2, 'DHPP', '2025-09-15', '2026-09-15'),
		 (2, 2, 'FVRCP', '2025-08-10', '2026-08-10'),
		 (3, 2, 'Rabies', '2025-07-25', '2026-07-25'),
		 (4, 2, 'RHDV2', '2026-01-15', '2027-01-15')`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return seedUsers(db)
}

const adminEmail = "admin@vetcare.local"

// seedUsers creates a login (password "password123") for every seeded owner
// and vet that does not have one yet, ensures the clinic administrator exists,
// and assigns any unassigned patient to a staff member.
func seedUsers(db *DB) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	stmts := []string{
		db.insertIgnore(`users (email, password_hash, role, owner_id)
		 SELECT o.email, ?, 'owner', o.id FROM owners o
		 WHERE o.email != '' AND NOT EXISTS (SELECT 1 FROM users u WHERE u.owner_id = o.id)`),
		db.insertIgnore(`users (email, password_hash, role, vet_id)
		 SELECT v.email, ?, 'vet', v.id FROM vets v
		 WHERE v.email != '' AND NOT EXISTS (SELECT 1 FROM users u WHERE u.vet_id = v.id)`),
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt, string(hash)); err != nil {
			return err
		}
	}
	if err := seedAdmin(db, string(hash)); err != nil {
		return err
	}
	return backfillAssignments(db)
}

// seedAdmin ensures the clinic administrator exists: a staff profile plus a
// user account carrying the is_admin flag. Safe to re-run.
func seedAdmin(db *DB, hash string) error {
	var vetID int64
	err := db.QueryRow(`SELECT id FROM vets WHERE email = ?`, adminEmail).Scan(&vetID)
	if err == sql.ErrNoRows {
		vetID, err = db.insertID(
			`INSERT INTO vets (name, specialty, email, phone) VALUES (?,?,?,?)`,
			"Dr. Maya Osei", "Clinic Director", adminEmail, "555-0200")
	}
	if err != nil {
		return err
	}
	var userID int64
	err = db.QueryRow(`SELECT id FROM users WHERE email = ?`, adminEmail).Scan(&userID)
	if err == sql.ErrNoRows {
		_, err = db.Exec(
			`INSERT INTO users (email, password_hash, role, vet_id, is_admin) VALUES (?,?,'vet',?,`+db.boolTrue()+`)`,
			adminEmail, hash, vetID)
		return err
	}
	if err != nil {
		return err
	}
	// Account already existed (e.g. seeded as a plain vet): promote it.
	_, err = db.Exec(`UPDATE users SET is_admin = `+db.boolTrue()+` WHERE id = ?`, userID)
	return err
}

// backfillAssignments gives every patient that has no assigned staff member the
// vet who most recently treated them, so an upgraded database is not left with
// a caseload nobody can see.
func backfillAssignments(db *DB) error {
	_, err := db.Exec(`
		UPDATE pets SET vet_id = COALESCE(
			(SELECT m.vet_id FROM medical_records m
			 WHERE m.pet_id = pets.id AND m.vet_id IS NOT NULL
			 ORDER BY m.visit_date DESC, m.id DESC LIMIT 1),
			(SELECT sg.vet_id FROM surgeries sg
			 WHERE sg.pet_id = pets.id AND sg.vet_id IS NOT NULL
			 ORDER BY sg.scheduled_at DESC, sg.id DESC LIMIT 1),
			(SELECT x.vet_id FROM vaccinations x
			 WHERE x.pet_id = pets.id AND x.vet_id IS NOT NULL
			 ORDER BY x.administered_at DESC, x.id DESC LIMIT 1))
		WHERE vet_id IS NULL`)
	return err
}
