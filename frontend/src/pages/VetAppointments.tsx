import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import type { Pet, Surgery, Vet } from '../api'
import RowMenu from '../components/RowMenu'

const empty = {
  pet_id: 0,
  vet_id: 0,
  procedure: '',
  scheduled_at: '',
  duration_min: 60,
  status: 'scheduled' as Surgery['status'],
  notes: '',
}

export default function VetAppointments({ isAdmin, vet }: { isAdmin: boolean; vet?: Vet }) {
  const [surgeries, setSurgeries] = useState<Surgery[]>([])
  const [pets, setPets] = useState<Pet[]>([])
  const [vets, setVets] = useState<Vet[]>([])
  const [filter, setFilter] = useState('')
  // Admin-only: '' = whole clinic, 'unassigned', or a staff id.
  const [vetFilter, setVetFilter] = useState('')
  const [form, setForm] = useState({ ...empty, vet_id: vet?.id ?? 0 })
  const [editingId, setEditingId] = useState<number | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(() => {
    const params = new URLSearchParams()
    if (filter) params.set('status', filter)
    if (isAdmin && vetFilter) params.set('vet_id', vetFilter)
    const q = params.toString() ? `?${params}` : ''
    Promise.all([
      api.get<Surgery[]>(`/api/surgeries${q}`),
      api.get<Pet[]>('/api/pets'),
      api.get<Vet[]>('/api/vets'),
    ])
      .then(([s, p, v]) => {
        setSurgeries(s)
        setPets(p)
        setVets(v)
      })
      .catch((e) => setError(e.message))
  }, [filter, isAdmin, vetFilter])
  useEffect(load, [load])

  // Non-admin staff always book onto their own calendar; the server enforces
  // this too, so send it explicitly rather than letting a stale form win.
  const payload = () => ({
    ...form,
    vet_id: (isAdmin ? form.vet_id : vet?.id ?? 0) || null,
  })

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      if (editingId !== null) {
        await api.put(`/api/surgeries/${editingId}`, payload())
      } else {
        await api.post('/api/surgeries', payload())
      }
      setForm({ ...empty, vet_id: vet?.id ?? 0 })
      setEditingId(null)
      setShowForm(false)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const edit = (s: Surgery) => {
    setForm({
      pet_id: s.pet_id,
      vet_id: s.vet_id ?? 0,
      procedure: s.procedure,
      scheduled_at: s.scheduled_at,
      duration_min: s.duration_min,
      status: s.status,
      notes: s.notes,
    })
    setEditingId(s.id)
    setShowForm(true)
  }

  const setStatus = async (s: Surgery, status: Surgery['status']) => {
    setError('')
    try {
      await api.put(`/api/surgeries/${s.id}`, {
        pet_id: s.pet_id,
        vet_id: s.vet_id,
        procedure: s.procedure,
        scheduled_at: s.scheduled_at,
        duration_min: s.duration_min,
        status,
        notes: s.notes,
      })
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const remove = async (s: Surgery) => {
    if (!confirm(`Delete "${s.procedure}" for ${s.pet_name}?`)) return
    try {
      await api.delete(`/api/surgeries/${s.id}`)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <div>
      <div className="toolbar">
        <h2>{isAdmin ? 'Clinic schedule' : 'My appointments'}</h2>
        <select value={filter} onChange={(e) => setFilter(e.target.value)}>
          <option value="">All statuses</option>
          <option value="scheduled">Scheduled</option>
          <option value="completed">Completed</option>
          <option value="cancelled">Cancelled</option>
        </select>
        {isAdmin && (
          <select value={vetFilter} onChange={(e) => setVetFilter(e.target.value)}>
            <option value="">All staff</option>
            <option value="unassigned">Unassigned</option>
            {vets.map((v) => (
              <option key={v.id} value={v.id}>
                {v.name}
              </option>
            ))}
          </select>
        )}
        <button
          className="btn primary"
          onClick={() => {
            setShowForm(!showForm)
            setEditingId(null)
            setForm({ ...empty, vet_id: vet?.id ?? 0 })
          }}
        >
          {showForm ? 'Close' : '+ Schedule'}
        </button>
      </div>
      {!isAdmin && (
        <p className="muted">Showing only the appointments assigned to you.</p>
      )}
      {error && <div className="error-banner">{error}</div>}

      {showForm && (
        <div className="card">
          <form className="entity-form" onSubmit={submit}>
            <label>
              Patient*
              <select
                value={form.pet_id}
                onChange={(e) => setForm({ ...form, pet_id: Number(e.target.value) })}
                required
              >
                <option value={0} disabled>
                  Choose patient…
                </option>
                {pets.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({p.owner_name})
                  </option>
                ))}
              </select>
            </label>
            {isAdmin && (
              <label>
                Assigned staff
                <select
                  value={form.vet_id}
                  onChange={(e) => setForm({ ...form, vet_id: Number(e.target.value) })}
                >
                  <option value={0}>Unassigned</option>
                  {vets.map((v) => (
                    <option key={v.id} value={v.id}>
                      {v.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <label>
              Procedure*
              <input
                value={form.procedure}
                onChange={(e) => setForm({ ...form, procedure: e.target.value })}
                required
              />
            </label>
            <label>
              When*
              <input
                type="datetime-local"
                value={form.scheduled_at}
                onChange={(e) => setForm({ ...form, scheduled_at: e.target.value })}
                required
              />
            </label>
            <label>
              Duration (min)
              <input
                type="number"
                min="5"
                step="5"
                value={form.duration_min}
                onChange={(e) => setForm({ ...form, duration_min: Number(e.target.value) })}
              />
            </label>
            <label>
              Status
              <select
                value={form.status}
                onChange={(e) =>
                  setForm({ ...form, status: e.target.value as Surgery['status'] })
                }
              >
                <option value="scheduled">Scheduled</option>
                <option value="completed">Completed</option>
                <option value="cancelled">Cancelled</option>
              </select>
            </label>
            <label>
              Notes
              <input
                value={form.notes}
                onChange={(e) => setForm({ ...form, notes: e.target.value })}
              />
            </label>
            <div className="form-actions">
              <button className="btn primary" type="submit">
                {editingId !== null ? 'Save' : 'Schedule'}
              </button>
            </div>
          </form>
        </div>
      )}

      {surgeries.length === 0 ? (
        <p className="muted">No appointments found.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>When</th>
              <th>Pet</th>
              <th>Procedure</th>
              <th>Duration</th>
              {isAdmin && <th>Assigned to</th>}
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
                {isAdmin && <td>{s.vet_name || 'Unassigned'}</td>}
                <td>
                  <span className={`badge ${s.status}`}>{s.status}</span>
                </td>
                <td className="row-actions">
                  <RowMenu
                    actions={[
                      { label: 'Edit / reschedule', onClick: () => edit(s) },
                      ...(s.status === 'scheduled'
                        ? [
                            { label: 'Mark completed', onClick: () => setStatus(s, 'completed') },
                            { label: 'Cancel', onClick: () => setStatus(s, 'cancelled') },
                          ]
                        : []),
                      { label: 'Delete', danger: true, onClick: () => remove(s) },
                    ]}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
