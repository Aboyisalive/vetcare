import { useEffect, useState } from 'react'
import { api } from '../api'
import type { Owner, Pet, Vaccination } from '../api'

export default function Vaccinations({
  owner,
  onOpenPet,
}: {
  owner: Owner
  onOpenPet: (id: number) => void
}) {
  const [vaccinations, setVaccinations] = useState<Vaccination[]>([])
  const [dueOnly, setDueOnly] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const q = dueOnly ? '?due_within=30' : ''
    Promise.all([
      api.get<Vaccination[]>(`/api/vaccinations${q}`),
      api.get<Pet[]>(`/api/pets?owner_id=${owner.id}`),
    ])
      .then(([vx, p]) => {
        const petIds = new Set(p.map((pet) => pet.id))
        setVaccinations(vx.filter((v) => petIds.has(v.pet_id)))
      })
      .catch((e) => setError(e.message))
  }, [dueOnly, owner.id])

  const today = new Date().toISOString().slice(0, 10)

  return (
    <div>
      <div className="toolbar">
        <h2>Vaccinations</h2>
        <label style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <input type="checkbox" checked={dueOnly} onChange={(e) => setDueOnly(e.target.checked)} />
          Due within 30 days only
        </label>
      </div>
      {error && <div className="error-banner">{error}</div>}
      <p className="muted">
        Vaccinations the clinic has on record for your pets. You'll be emailed automatically
        when one comes due.
      </p>
      {vaccinations.length === 0 ? (
        <p className="muted">No vaccinations on record for your pets.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Pet</th>
              <th>Vaccine</th>
              <th>Administered</th>
              <th>Next due</th>
              <th>Status</th>
              <th>Vet</th>
            </tr>
          </thead>
          <tbody>
            {vaccinations.map((v) => (
              <tr key={v.id}>
                <td>
                  <button className="link" onClick={() => onOpenPet(v.pet_id)}>
                    {v.pet_name}
                  </button>
                </td>
                <td>{v.vaccine}</td>
                <td>{v.administered_at}</td>
                <td>{v.next_due || '—'}</td>
                <td>
                  {v.next_due === '' ? (
                    <span className="muted">—</span>
                  ) : v.next_due < today ? (
                    <span className="badge overdue">overdue</span>
                  ) : (
                    <span className="badge due-soon">upcoming</span>
                  )}
                </td>
                <td>{v.vet_name || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
