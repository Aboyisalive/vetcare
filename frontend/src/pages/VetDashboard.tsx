import { useEffect, useState } from 'react'
import { api } from '../api'
import type { Stats, Surgery, Vaccination, Vet } from '../api'

export default function VetDashboard({
  isAdmin,
  onOpenPet,
}: {
  isAdmin: boolean
  onOpenPet: (id: number) => void
}) {
  const [stats, setStats] = useState<Stats | null>(null)
  const [surgeries, setSurgeries] = useState<Surgery[]>([])
  const [due, setDue] = useState<Vaccination[]>([])
  const [staff, setStaff] = useState<Vet[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([
      api.get<Stats>('/api/stats'),
      api.get<Surgery[]>('/api/surgeries?status=scheduled'),
      api.get<Vaccination[]>('/api/vaccinations?due_within=14'),
      // The staff workload table is an admin-only breakdown.
      isAdmin ? api.get<Vet[]>('/api/vets') : Promise.resolve([]),
    ])
      .then(([st, sg, vx, vt]) => {
        setStats(st)
        setSurgeries(sg)
        setDue(vx)
        setStaff(vt)
      })
      .catch((e) => setError(e.message))
  }, [isAdmin])

  if (error) return <div className="error-banner">{error}</div>
  if (!stats) return <p className="muted">Loading…</p>

  const tiles: [string, number][] = isAdmin
    ? [
        ['Owners', stats.owners],
        ['Patients', stats.pets],
        ['Staff', stats.vets],
        ['Requests to answer', stats.pending_requests],
        ['Upcoming surgeries', stats.upcoming_surgeries],
        ['Vaccinations due (14d)', stats.vaccinations_due_soon],
      ]
    : [
        ['My clients', stats.owners],
        ['My patients', stats.pets],
        ['Requests to answer', stats.pending_requests],
        ['My upcoming surgeries', stats.upcoming_surgeries],
        ['Vaccinations due (14d)', stats.vaccinations_due_soon],
      ]

  const today = new Date().toISOString().slice(0, 10)

  return (
    <div>
      <h2>{isAdmin ? 'Clinic overview' : 'My caseload'}</h2>
      {!isAdmin && (
        <p className="muted">
          Showing only the patients and appointments assigned to you.
        </p>
      )}
      <div className="stats-grid">
        {tiles.map(([label, num]) => (
          <div className="stat" key={label}>
            <div className="num">{num}</div>
            <div className="label">{label}</div>
          </div>
        ))}
      </div>

      {isAdmin && (
        <>
          <h3>Staff workload</h3>
          {staff.length === 0 ? (
            <p className="muted">No staff on the roster.</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Staff</th>
                  <th>Specialty</th>
                  <th>Patients</th>
                  <th>Upcoming</th>
                </tr>
              </thead>
              <tbody>
                {staff.map((v) => (
                  <tr key={v.id}>
                    <td>
                      {v.name}
                      {v.is_admin && <span className="badge admin">admin</span>}
                    </td>
                    <td>{v.specialty || '—'}</td>
                    <td>{v.patient_count ?? 0}</td>
                    <td>{v.upcoming_count ?? 0}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </>
      )}

      <h3>{isAdmin ? 'Upcoming appointments' : 'My upcoming appointments'}</h3>
      {surgeries.length === 0 ? (
        <p className="muted">Nothing scheduled.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>When</th>
              <th>Pet</th>
              <th>Procedure</th>
              {isAdmin && <th>Assigned to</th>}
            </tr>
          </thead>
          <tbody>
            {surgeries.map((s) => (
              <tr key={s.id}>
                <td>{s.scheduled_at.replace('T', ' ')}</td>
                <td>
                  <button className="link" onClick={() => onOpenPet(s.pet_id)}>
                    {s.pet_name}
                  </button>
                </td>
                <td>{s.procedure}</td>
                {isAdmin && <td>{s.vet_name || 'Unassigned'}</td>}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h3>Vaccinations due within 14 days</h3>
      {due.length === 0 ? (
        <p className="muted">Nothing due soon.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Due</th>
              <th>Pet</th>
              <th>Vaccine</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {due.map((v) => (
              <tr key={v.id}>
                <td>{v.next_due}</td>
                <td>
                  <button className="link" onClick={() => onOpenPet(v.pet_id)}>
                    {v.pet_name}
                  </button>
                </td>
                <td>{v.vaccine}</td>
                <td>
                  <span className={`badge ${v.next_due < today ? 'overdue' : 'due-soon'}`}>
                    {v.next_due < today ? 'overdue' : 'due soon'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
