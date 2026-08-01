package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"
)

type server struct {
	db     *DB
	mailer *mailer
}

func main() {
	// Real environment variables take precedence over the .env file.
	loadDotEnv(".env", "../.env")

	addr := flag.String("addr", envOr("ADDR", ":8080"), "listen address")
	dbPath := flag.String("db", envOr("SQLITE_PATH", "vetcare.db"), "sqlite database path (ignored when DATABASE_URL is set)")
	seed := flag.Bool("seed", false, "insert sample data and exit")
	reminderEvery := flag.Duration("reminder-interval", time.Hour, "how often the reminder worker runs")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	db, err := openDB(databaseURL, *dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	dialect, dsn := resolveDSN(databaseURL, *dbPath)
	log.Printf("database connected (%s)", describeDSN(dialect, dsn))

	s := &server{db: db, mailer: mailerFromEnv()}

	if *seed {
		if err := seedData(db); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Println("sample data inserted")
		return
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/auth/signup", s.signup)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.me))

	mux.HandleFunc("GET /api/stats", s.requireAuth(s.getStats))

	mux.HandleFunc("GET /api/owners", s.requireVet(s.listOwners))
	mux.HandleFunc("POST /api/owners", s.requireVet(s.createOwner))
	mux.HandleFunc("GET /api/owners/{id}", s.requireAuth(s.getOwner))
	mux.HandleFunc("PUT /api/owners/{id}", s.requireAuth(s.updateOwner))
	mux.HandleFunc("DELETE /api/owners/{id}", s.requireVet(s.deleteOwner))

	mux.HandleFunc("GET /api/pets", s.requireAuth(s.listPets))
	mux.HandleFunc("POST /api/pets", s.requireAuth(s.createPet))
	mux.HandleFunc("GET /api/pets/{id}", s.requireAuth(s.getPet))
	mux.HandleFunc("PUT /api/pets/{id}", s.requireAuth(s.updatePet))
	mux.HandleFunc("DELETE /api/pets/{id}", s.requireAuth(s.deletePet))
	mux.HandleFunc("GET /api/pets/{id}/records", s.requireAuth(s.listPetRecords))

	mux.HandleFunc("GET /api/vets", s.requireAuth(s.listVets))
	mux.HandleFunc("POST /api/vets", s.requireAdmin(s.createVet))
	mux.HandleFunc("PUT /api/vets/{id}", s.requireAdmin(s.updateVet))
	mux.HandleFunc("DELETE /api/vets/{id}", s.requireAdmin(s.deleteVet))
	mux.HandleFunc("PUT /api/pets/{id}/vet", s.requireAdmin(s.assignPet))

	mux.HandleFunc("POST /api/records", s.requireVet(s.createRecord))
	mux.HandleFunc("PUT /api/records/{id}", s.requireVet(s.updateRecord))
	mux.HandleFunc("DELETE /api/records/{id}", s.requireVet(s.deleteRecord))

	mux.HandleFunc("GET /api/availability", s.requireAuth(s.getAvailability))
	mux.HandleFunc("GET /api/surgeries", s.requireAuth(s.listSurgeries))
	// Owners may book here too — their bookings land as pending requests.
	mux.HandleFunc("POST /api/surgeries", s.requireAuth(s.createSurgery))
	mux.HandleFunc("PUT /api/surgeries/{id}", s.requireAuth(s.updateSurgery))
	mux.HandleFunc("POST /api/surgeries/{id}/respond", s.requireVet(s.respondSurgery))
	mux.HandleFunc("DELETE /api/surgeries/{id}", s.requireVet(s.deleteSurgery))

	mux.HandleFunc("GET /api/vaccinations", s.requireAuth(s.listVaccinations))
	mux.HandleFunc("POST /api/vaccinations", s.requireVet(s.createVaccination))
	mux.HandleFunc("PUT /api/vaccinations/{id}", s.requireVet(s.updateVaccination))
	mux.HandleFunc("DELETE /api/vaccinations/{id}", s.requireVet(s.deleteVaccination))

	mux.HandleFunc("GET /api/reminders", s.requireAuth(s.listReminders))
	mux.HandleFunc("POST /api/reminders/run", s.requireVet(s.runReminders))
	mux.HandleFunc("DELETE /api/reminders/{id}", s.requireVet(s.deleteReminder))

	s.startReminderWorker(*reminderEvery)

	log.Printf("VetCare API listening on %s", *addr)
	if err := http.ListenAndServe(*addr, cors(mux)); err != nil {
		log.Fatal(err)
	}
}
