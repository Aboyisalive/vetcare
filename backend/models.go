package main

type Owner struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
	PetCount  int    `json:"pet_count,omitempty"`
}

type Pet struct {
	ID          int64   `json:"id"`
	OwnerID     int64   `json:"owner_id"`
	OwnerName   string  `json:"owner_name,omitempty"`
	VetID       *int64  `json:"vet_id"`
	VetName     string  `json:"vet_name,omitempty"`
	Name        string  `json:"name"`
	Species     string  `json:"species"`
	Breed       string  `json:"breed"`
	Sex         string  `json:"sex"`
	BirthDate   string  `json:"birth_date"`
	WeightKg    float64 `json:"weight_kg"`
	MicrochipID string  `json:"microchip_id"`
	Notes       string  `json:"notes"`
	CreatedAt   string  `json:"created_at"`
}

type Vet struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Specialty string `json:"specialty"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	// Populated by the staff directory only; both describe the vet's workload.
	IsAdmin       bool `json:"is_admin"`
	PatientCount  int  `json:"patient_count"`
	UpcomingCount int  `json:"upcoming_count"`
}

type MedicalRecord struct {
	ID        int64  `json:"id"`
	PetID     int64  `json:"pet_id"`
	PetName   string `json:"pet_name,omitempty"`
	VetID     *int64 `json:"vet_id"`
	VetName   string `json:"vet_name,omitempty"`
	VisitDate string `json:"visit_date"`
	Category  string `json:"category"`
	Diagnosis string `json:"diagnosis"`
	Treatment string `json:"treatment"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
}

type Surgery struct {
	ID          int64  `json:"id"`
	PetID       int64  `json:"pet_id"`
	PetName     string `json:"pet_name,omitempty"`
	VetID       *int64 `json:"vet_id"`
	VetName     string `json:"vet_name,omitempty"`
	Procedure   string `json:"procedure"`
	ScheduledAt string `json:"scheduled_at"`
	DurationMin int    `json:"duration_min"`
	Status      string `json:"status"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"created_at"`
}

type Vaccination struct {
	ID             int64  `json:"id"`
	PetID          int64  `json:"pet_id"`
	PetName        string `json:"pet_name,omitempty"`
	VetID          *int64 `json:"vet_id"`
	VetName        string `json:"vet_name,omitempty"`
	Vaccine        string `json:"vaccine"`
	AdministeredAt string `json:"administered_at"`
	NextDue        string `json:"next_due"`
	Notes          string `json:"notes"`
	CreatedAt      string `json:"created_at"`
}

type Reminder struct {
	ID            int64  `json:"id"`
	VaccinationID int64  `json:"vaccination_id"`
	Vaccine       string `json:"vaccine,omitempty"`
	PetName       string `json:"pet_name,omitempty"`
	OwnerName     string `json:"owner_name,omitempty"`
	OwnerEmail    string `json:"owner_email,omitempty"`
	DueDate       string `json:"due_date"`
	Status        string `json:"status"`
	Channel       string `json:"channel"`
	Message       string `json:"message"`
	SentAt        string `json:"sent_at"`
	CreatedAt     string `json:"created_at"`
}
