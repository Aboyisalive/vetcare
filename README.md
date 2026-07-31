# VetCare — Pet Clinic Management & Vaccination Tracker

Clinic system: animal medical histories, surgery scheduling, automated vaccination reminders.

- **Backend:** Go + SQLite (`modernc.org/sqlite`, no cgo)
- **Frontend:** React + TypeScript (Vite), light mode

## Run

Backend (port 8080):

```sh
cd backend
go run . -seed        # optional: insert sample data (first run only)
go run .
```

Frontend (port 5173, proxies `/api` to 8080):

```sh
cd frontend
npm install
npm run dev
```

## Database

The backend runs on **SQLite** (default, zero setup) or **PostgreSQL**, chosen
by `DATABASE_URL`. Copy `.env.example` to `.env` and set it:

```sh
cp .env.example .env
```

```ini
# PostgreSQL — Neon, Supabase, Railway, RDS, local, anything
DATABASE_URL=postgres://user:password@host:5432/vetcare?sslmode=require

# or SQLite (also the default when DATABASE_URL is empty)
DATABASE_URL=sqlite://./vetcare.db
```

`.env` is read from `backend/.env` first, then the project root. Real
environment variables always win, so this works too:

```sh
DATABASE_URL=postgres://... go run .
```

Tables are created on startup (`CREATE TABLE IF NOT EXISTS`) on either engine —
point at an empty database and it migrates itself. Then `go run . -seed` loads
sample data and demo logins.

Everything else is engine-agnostic: `backend/dialect.go` handles the
differences (`?` vs `$1` placeholders, `LastInsertId` vs `RETURNING id`,
date arithmetic, upsert syntax). Date columns are TEXT in `YYYY-MM-DD` form on
both engines, so ordering and comparisons behave identically.

## Backend configuration

| Env var | Flag | Default | Purpose |
|---------|------|---------|---------|
| `DATABASE_URL` | — | empty (SQLite) | `postgres://…` or `sqlite://…` connection URL |
| `SQLITE_PATH` | `-db` | `vetcare.db` | SQLite file, used only when `DATABASE_URL` is empty |
| `ADDR` | `-addr` | `:8080` | listen address |
| — | `-seed` | off | insert sample data and exit |
| — | `-reminder-interval` | `1h` | reminder worker cadence |

## Email reminders

Worker runs on an interval (and at startup). It finds vaccinations with `next_due`
within 14 days (or overdue), creates one reminder per vaccination+due date, and
delivers it. With no SMTP config, reminders are written to the server log
(channel `log`). To send real email set:

```
SMTP_HOST, SMTP_PORT (default 587), SMTP_USER, SMTP_PASS, SMTP_FROM
```

Manual trigger: `POST /api/reminders/run`.

## Auth

Cookie-session auth (bcrypt password hashes, `sessions` table, 7-day expiry).
Two roles, plus an `is_admin` flag on staff accounts:

- **owner** — sees and manages only their own pets/appointments/vaccinations/reminders.
- **vet** (clinic staff) — sees only the patients assigned to them (`pets.vet_id`)
  and the appointments assigned to them (`surgeries.vet_id`). They can read and
  write the chart of any patient they are assigned to *or* are booked to operate
  on, but cannot reassign patients or manage the staff roster.
- **vet + `is_admin`** — clinic administrator. Sees every owner, patient and
  appointment, sees which staff member each patient is assigned to, can filter
  any list by staff member, reassign patients, and manage the staff roster. The
  UI gives admins an extra **Staff** page and clinic-wide dashboard.

Sign up via the UI or `POST /api/auth/signup` with
`{role: "owner"|"vet", name, email, password, phone?, address?, specialty?}`.
Signup never grants `is_admin`; promote an account with
`UPDATE users SET is_admin = TRUE WHERE email = '…'`.

`go run . -seed` also backfills demo logins (password `password123`) for the
sample owners (`alice@example.com`, …), vets (`skim@vetcare.local`,
`jpatel@vetcare.local`) and the administrator (`admin@vetcare.local`).
Seeding also assigns any patient without a staff member to the vet who most
recently treated them, so an upgraded database has no invisible caseload.

| Endpoint | Purpose |
|----------|---------|
| `POST /api/auth/signup` | create account (+ owner or vet profile) and session |
| `POST /api/auth/login` | start session |
| `POST /api/auth/logout` | end session |
| `GET /api/auth/me` | current user (`{id, email, role, owner?, vet?}`) |

All other `/api` routes require a session; writes to records/vaccinations,
surgery create/delete, and owner/vet administration require the vet role.

## API

Base: `http://localhost:8080/api`

| Resource | Endpoints |
|----------|-----------|
| Health | `GET /health` |
| Stats | `GET /stats` |
| Owners | `GET,POST /owners` · `GET,PUT,DELETE /owners/{id}` |
| Pets | `GET,POST /pets` (`?owner_id=&vet_id=`) · `GET,PUT,DELETE /pets/{id}` · `PUT /pets/{id}/vet` (admin) |
| Medical records | `GET /pets/{id}/records` · `POST /records` · `PUT,DELETE /records/{id}` |
| Vets | `GET,POST /vets` · `PUT,DELETE /vets/{id}` |
| Surgeries | `GET,POST /surgeries` (`?status=&pet_id=&vet_id=&from=&to=`) · `PUT,DELETE /surgeries/{id}` |
| Vaccinations | `GET,POST /vaccinations` (`?pet_id=&due_within=`) · `PUT,DELETE /vaccinations/{id}` |
| Reminders | `GET /reminders` (`?status=`) · `POST /reminders/run` · `DELETE /reminders/{id}` |

Notes:

- Dates are `YYYY-MM-DD`; surgery `scheduled_at` is `YYYY-MM-DDTHH:MM`.
- Scheduling a surgery returns `409` when the vet has an overlapping scheduled surgery.
- Deleting an owner cascades to their pets, records, vaccinations, reminders.
