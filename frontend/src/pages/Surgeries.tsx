import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import type { Availability, Owner, Pet, Surgery } from '../api'
import RowMenu from '../components/RowMenu'

const durations = [15, 30, 45, 60, 90]

const today = () => new Date().toISOString().slice(0, 10)

const statusLabels: Record<Surgery['status'], string> = {
  requested: 'awaiting confirmation',
  scheduled: 'confirmed',
  completed: 'completed',
  cancelled: 'cancelled',
  declined: 'declined',
}

export default function Surgeries({ owner }: { owner: Owner }) {
  const [surgeries, setSurgeries] = useState<Surgery[]>([])
  const [pets, setPets] = useState<Pet[]>([])
  const [filter, setFilter] = useState('')
  const [error, setError] = useState('')

  // Booking form: pick pet + day + length, then choose from the free slots the
  // clinic offers back. Nothing outside that list can be submitted.
  const [showForm, setShowForm] = useState(false)
  const [petId, setPetId] = useState(0)
  const [date, setDate] = useState(today())
  const [duration, setDuration] = useState(30)
  const [reason, setReason] = useState('')
  const [notes, setNotes] = useState('')
  const [availability, setAvailability] = useState<Availability | null>(null)
  const [loadingSlots, setLoadingSlots] = useState(false)
  const [picked, setPicked] = useState<{ vetId: number; time: string } | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const load = useCallback(() => {
    const q = filter ? `?status=${filter}` : ''
    Promise.all([
      api.get<Surgery[]>(`/api/surgeries${q}`),
      api.get<Pet[]>(`/api/pets?owner_id=${owner.id}`),
    ])
      .then(([s, p]) => {
        const petIds = new Set(p.map((pet) => pet.id))
        setSurgeries(s.filter((sg) => petIds.has(sg.pet_id)))
        setPets(p)
      })
      .catch((e) => setError(e.message))
  }, [filter, owner.id])
  useEffect(load, [load])

  // Re-ask the clinic whenever the pet, day or length changes: who is free
  // depends on all three.
  useEffect(() => {
    if (!showForm || !petId || !date) {
      setAvailability(null)
      return
    }
    let cancelled = false
    setLoadingSlots(true)
    setPicked(null)
    api
      .get<Availability>(`/api/availability?pet_id=${petId}&date=${date}&duration=${duration}`)
      .then((a) => {
        if (!cancelled) setAvailability(a)
      })
      .catch((e) => {
        if (!cancelled) setError((e as Error).message)
      })
      .finally(() => {
        if (!cancelled) setLoadingSlots(false)
      })
    return () => {
      cancelled = true
    }
  }, [showForm, petId, date, duration])

  const resetForm = () => {
    setPetId(0)
    setDate(today())
    setDuration(30)
    setReason('')
    setNotes('')
    setAvailability(null)
    setPicked(null)
  }

  const book = async () => {
    if (!picked || !petId || !reason.trim()) return
    setError('')
    setSubmitting(true)
    try {
      await api.post('/api/surgeries', {
        pet_id: petId,
        vet_id: picked.vetId,
        procedure: reason.trim(),
        scheduled_at: `${date}T${picked.time}`,
        duration_min: duration,
        notes: notes.trim(),
      })
      resetForm()
      setShowForm(false)
      load()
    } catch (err) {
      setError((err as Error).message)
      // The slot may have gone while the form was open — refresh what is left.
      api
        .get<Availability>(`/api/availability?pet_id=${petId}&date=${date}&duration=${duration}`)
        .then(setAvailability)
        .catch(() => {})
      setPicked(null)
    } finally {
      setSubmitting(false)
    }
  }

  const cancel = async (s: Surgery) => {
    const what = s.status === 'requested' ? 'Withdraw the request for' : 'Cancel'
    if (!confirm(`${what} "${s.procedure}" for ${s.pet_name}? The clinic will be notified.`))
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

  const canBook = pets.length > 0
  const pendingCount = surgeries.filter((s) => s.status === 'requested').length

  return (
    <div>
      <div className="toolbar">
        <h2>Appointments & Surgeries</h2>
        <select value={filter} onChange={(e) => setFilter(e.target.value)}>
          <option value="">All statuses</option>
          <option value="requested">Awaiting confirmation</option>
          <option value="scheduled">Confirmed</option>
          <option value="completed">Completed</option>
          <option value="cancelled">Cancelled</option>
          <option value="declined">Declined</option>
        </select>
        <button
          className="btn primary"
          disabled={!canBook}
          onClick={() => {
            setShowForm(!showForm)
            resetForm()
          }}
        >
          {showForm ? 'Close' : '+ Book appointment'}
        </button>
      </div>
      {error && <div className="error-banner">{error}</div>}
      <p className="muted">
        Book a visit with one of the vets who look after your pet. Requests are confirmed
        by the vet before they become official
        {pendingCount > 0 && ` — ${pendingCount} waiting right now`}.
      </p>

      {showForm && (
        <div className="card">
          <form className="entity-form" onSubmit={(e) => e.preventDefault()}>
            <label>
              Pet*
              <select value={petId} onChange={(e) => setPetId(Number(e.target.value))} required>
                <option value={0} disabled>
                  Choose pet…
                </option>
                {pets.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({p.species})
                  </option>
                ))}
              </select>
            </label>
            <label>
              Reason for visit*
              <input
                value={reason}
                placeholder="Vaccination, limping, check-up…"
                onChange={(e) => setReason(e.target.value)}
                required
              />
            </label>
            <label>
              Date*
              <input
                type="date"
                min={today()}
                value={date}
                onChange={(e) => setDate(e.target.value)}
                required
              />
            </label>
            <label>
              Length
              <select value={duration} onChange={(e) => setDuration(Number(e.target.value))}>
                {durations.map((d) => (
                  <option key={d} value={d}>
                    {d} minutes
                  </option>
                ))}
              </select>
            </label>
            <label>
              Anything the vet should know
              <input value={notes} onChange={(e) => setNotes(e.target.value)} />
            </label>
          </form>

          <div className="slot-picker">
            {!petId ? (
              <p className="muted">Choose a pet to see who is available.</p>
            ) : loadingSlots ? (
              <p className="muted">Checking the clinic diary…</p>
            ) : availability?.closed ? (
              <p className="muted">The clinic is closed that day — pick a weekday.</p>
            ) : !availability || availability.vets.length === 0 ? (
              <p className="muted">
                No free slots that day. Try another date or a shorter appointment.
              </p>
            ) : (
              availability.vets.map((v) => (
                <div className="slot-vet" key={v.vet_id}>
                  <div className="slot-vet-name">
                    {v.vet_name}
                    {v.specialty && <span className="muted"> · {v.specialty}</span>}
                  </div>
                  <div className="slot-list">
                    {v.slots.map((t) => {
                      const active = picked?.vetId === v.vet_id && picked?.time === t
                      return (
                        <button
                          key={t}
                          type="button"
                          className={`slot${active ? ' active' : ''}`}
                          onClick={() => setPicked({ vetId: v.vet_id, time: t })}
                        >
                          {t}
                        </button>
                      )
                    })}
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="form-actions">
            <button
              className="btn primary"
              disabled={!picked || !reason.trim() || submitting}
              onClick={book}
            >
              {submitting ? 'Requesting…' : 'Request appointment'}
            </button>
            {picked && (
              <span className="muted">
                {availability?.vets.find((v) => v.vet_id === picked.vetId)?.vet_name} ·{' '}
                {date} {picked.time}
              </span>
            )}
          </div>
        </div>
      )}

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
                  <span className={`badge ${s.status}`}>{statusLabels[s.status]}</span>
                </td>
                <td className="row-actions">
                  {(s.status === 'scheduled' || s.status === 'requested') && (
                    <RowMenu
                      actions={[
                        {
                          label:
                            s.status === 'requested'
                              ? 'Withdraw request'
                              : 'Cancel appointment',
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
