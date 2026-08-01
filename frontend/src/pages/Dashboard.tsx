import { useEffect, useState } from 'react'
import { api } from '../api'
import type { Owner, Pet, Reminder, Surgery, Vaccination } from '../api'

export default function Dashboard({
  owner,
  onOpenPet,
}: {
  owner: Owner
  onOpenPet: (id: number) => void
}) {
  const [pets, setPets] = useState<Pet[]>([])
  const [surgeries, setSurgeries] = useState<Surgery[]>([])
  const [requested, setRequested] = useState<Surgery[]>([])
  const [due, setDue] = useState<Vaccination[]>([])
  const [reminders, setReminders] = useState<Reminder[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([
      api.get<Pet[]>(`/api/pets?owner_id=${owner.id}`),
      api.get<Surgery[]>('/api/surgeries?status=scheduled'),
      api.get<Vaccination[]>('/api/vaccinations?due_within=30'),
      api.get<Reminder[]>('/api/reminders?status=pending'),
      // Bookings the vet has not answered yet.
      api.get<Surgery[]>('/api/surgeries?status=requested'),
    ])
      .then(([p, sg, vx, rm, rq]) => {
        const petIds = new Set(p.map((pet) => pet.id))
        setPets(p)
        setSurgeries(sg.filter((s) => petIds.has(s.pet_id)))
        setRequested(rq.filter((s) => petIds.has(s.pet_id)))
        setDue(vx.filter((v) => petIds.has(v.pet_id)))
        setReminders(rm.filter((r) => r.owner_email === owner.email))
        setLoaded(true)
      })
      .catch((e) => setError(e.message))
  }, [owner])

  if (error) return <div className="error-banner">{error}</div>
  if (!loaded) return <p className="muted">Loading…</p>

  const tiles: [string, number][] = [
    ['My pets', pets.length],
    ['Upcoming appointments', surgeries.length],
    ['Awaiting confirmation', requested.length],
    ['Vaccinations due (30d)', due.length],
    ['Pending reminders', reminders.length],
  ]

  const today = new Date().toISOString().slice(0, 10)

  return (
    <div>
      <h2>Welcome back, {owner.name.split(' ')[0]}</h2>
      <div className="stats-grid">
        {tiles.map(([label, num]) => (
          <div className="stat" key={label}>
            <div className="num">{num}</div>
            <div className="label">{label}</div>
          </div>
        ))}
      </div>

      <h3>My pets</h3>
      {pets.length === 0 ? (
        <p className="muted">No pets registered on your account yet.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Species</th>
              <th>Breed</th>
              <th>Born</th>
            </tr>
          </thead>
          <tbody>
            {pets.map((p) => (
              <tr key={p.id}>
                <td>
                  <button className="link" onClick={() => onOpenPet(p.id)}>
                    {p.name}
                  </button>
                </td>
                <td>{p.species}</td>
                <td>{p.breed || '—'}</td>
                <td>{p.birth_date || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h3>Upcoming appointments</h3>
      {surgeries.length === 0 ? (
        <p className="muted">No appointments scheduled for your pets.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>When</th>
              <th>Pet</th>
              <th>Procedure</th>
              <th>Vet</th>
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
                <td>{s.vet_name || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h3>Vaccinations due within 30 days</h3>
      {due.length === 0 ? (
        <p className="muted">Nothing due soon — all caught up.</p>
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
