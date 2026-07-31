import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import type { Owner, Pet, Surgery } from '../api'
import RowMenu from '../components/RowMenu'

export default function Surgeries({ owner }: { owner: Owner }) {
  const [surgeries, setSurgeries] = useState<Surgery[]>([])
  const [filter, setFilter] = useState('')
  const [error, setError] = useState('')

  const load = useCallback(() => {
    const q = filter ? `?status=${filter}` : ''
    Promise.all([
      api.get<Surgery[]>(`/api/surgeries${q}`),
      api.get<Pet[]>(`/api/pets?owner_id=${owner.id}`),
    ])
      .then(([s, p]) => {
        const petIds = new Set(p.map((pet) => pet.id))
        setSurgeries(s.filter((sg) => petIds.has(sg.pet_id)))
      })
      .catch((e) => setError(e.message))
  }, [filter, owner.id])
  useEffect(load, [load])

  const cancel = async (s: Surgery) => {
    if (!confirm(`Cancel "${s.procedure}" for ${s.pet_name}? The clinic will be notified.`))
      return
    setError('')
    try {
      await api.put(`/api/surgeries/${s.id}`, {
        pet_id: s.pet_id,
        vet_id: s.vet_id,
        procedure: s.procedure,
        scheduled_at: s.scheduled_at,
        duration_min: s.duration_min,
        status: 'cancelled',
        notes: s.notes,
      })
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <div>
      <div className="toolbar">
        <h2>Appointments & Surgeries</h2>
        <select value={filter} onChange={(e) => setFilter(e.target.value)}>
          <option value="">All statuses</option>
          <option value="scheduled">Scheduled</option>
          <option value="completed">Completed</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>
      {error && <div className="error-banner">{error}</div>}
      <p className="muted">
        Procedures the clinic has scheduled for your pets. Contact the clinic to book or
        reschedule.
      </p>

      {surgeries.length === 0 ? (
        <p className="muted">No appointments found for your pets.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>When</th>
              <th>Pet</th>
              <th>Procedure</th>
              <th>Duration</th>
              <th>Vet</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {surgeries.map((s) => (
              <tr key={s.id}>
                <td>{s.scheduled_at.replace('T', ' ')}</td>
                <td>{s.pet_name}</td>
                <td>{s.procedure}</td>
                <td>{s.duration_min} min</td>
                <td>{s.vet_name || '—'}</td>
                <td>
                  <span className={`badge ${s.status}`}>{s.status}</span>
                </td>
                <td className="row-actions">
                  {s.status === 'scheduled' && (
                    <RowMenu
                      actions={[
                        {
                          label: 'Cancel appointment',
                          danger: true,
                          onClick: () => cancel(s),
                        },
                      ]}
                    />
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
