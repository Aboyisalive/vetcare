import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import type { Vet } from '../api'
import RowMenu from '../components/RowMenu'

const empty = { name: '', specialty: '', email: '', phone: '' }

// Admin-only roster: who works at the clinic and how loaded each of them is.
export default function VetStaff() {
  const [staff, setStaff] = useState<Vet[]>([])
  const [form, setForm] = useState(empty)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(() => {
    api
      .get<Vet[]>('/api/vets')
      .then(setStaff)
      .catch((e) => setError(e.message))
  }, [])
  useEffect(load, [load])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      if (editingId !== null) {
        await api.put(`/api/vets/${editingId}`, form)
      } else {
        await api.post('/api/vets', form)
      }
      setForm(empty)
      setEditingId(null)
      setShowForm(false)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const edit = (v: Vet) => {
    setForm({ name: v.name, specialty: v.specialty, email: v.email, phone: v.phone })
    setEditingId(v.id)
    setShowForm(true)
  }

  const remove = async (v: Vet) => {
    if (
      !confirm(
        `Remove ${v.name} from the roster? Their patients and appointments become unassigned.`,
      )
    )
      return
    try {
      await api.delete(`/api/vets/${v.id}`)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <div>
      <div className="toolbar">
        <h2>Clinic staff</h2>
        <button
          className="btn primary"
          onClick={() => {
            setShowForm(!showForm)
            setEditingId(null)
            setForm(empty)
          }}
        >
          {showForm ? 'Close' : '+ Add staff'}
        </button>
      </div>
      <p className="muted">
        Patients and appointments are scoped to the staff member they are assigned to.
        Reassign patients from the Patients page.
      </p>
      {error && <div className="error-banner">{error}</div>}

      {showForm && (
        <div className="card">
          <form className="entity-form" onSubmit={submit}>
            <label>
              Name*
              <input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                required
              />
            </label>
            <label>
              Specialty
              <input
                value={form.specialty}
                placeholder="Surgery, Dentistry…"
                onChange={(e) => setForm({ ...form, specialty: e.target.value })}
              />
            </label>
            <label>
              Email
              <input
                type="email"
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
            </label>
            <label>
              Phone
              <input
                value={form.phone}
                onChange={(e) => setForm({ ...form, phone: e.target.value })}
              />
            </label>
            <div className="form-actions">
              <button className="btn primary" type="submit">
                {editingId !== null ? 'Save' : 'Add'}
              </button>
            </div>
          </form>
        </div>
      )}

      {staff.length === 0 ? (
        <p className="muted">No staff on the roster.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Specialty</th>
              <th>Email</th>
              <th>Phone</th>
              <th>Patients</th>
              <th>Upcoming</th>
              <th></th>
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
                <td>{v.email || '—'}</td>
                <td>{v.phone || '—'}</td>
                <td>{v.patient_count ?? 0}</td>
                <td>{v.upcoming_count ?? 0}</td>
                <td className="row-actions">
                  <RowMenu
                    actions={[
                      { label: 'Edit', onClick: () => edit(v) },
                      ...(v.is_admin
                        ? []
                        : [{ label: 'Remove', danger: true, onClick: () => remove(v) }]),
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
