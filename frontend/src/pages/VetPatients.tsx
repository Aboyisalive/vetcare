import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import type { Owner, Pet, Vet } from '../api'
import RowMenu from '../components/RowMenu'

const empty = {
  owner_id: 0,
  vet_id: 0,
  name: '',
  species: '',
  breed: '',
  sex: 'unknown' as Pet['sex'],
  birth_date: '',
  weight_kg: 0,
  microchip_id: '',
  notes: '',
}

export default function VetPatients({
  isAdmin,
  onOpenPet,
}: {
  isAdmin: boolean
  onOpenPet: (id: number) => void
}) {
  const [pets, setPets] = useState<Pet[]>([])
  const [owners, setOwners] = useState<Owner[]>([])
  const [staff, setStaff] = useState<Vet[]>([])
  const [search, setSearch] = useState('')
  // Admin-only: '' = whole clinic, 'unassigned', or a staff id.
  const [vetFilter, setVetFilter] = useState('')
  const [form, setForm] = useState(empty)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(() => {
    const q = isAdmin && vetFilter ? `?vet_id=${vetFilter}` : ''
    Promise.all([
      api.get<Pet[]>(`/api/pets${q}`),
      api.get<Owner[]>('/api/owners'),
      api.get<Vet[]>('/api/vets'),
    ])
      .then(([p, o, v]) => {
        setPets(p)
        setOwners(o)
        setStaff(v)
      })
      .catch((e) => setError(e.message))
  }, [isAdmin, vetFilter])
  useEffect(load, [load])

  // Only admins pick the treating vet; for everyone else the server pins the
  // patient to the signed-in staff member.
  const payload = () => ({
    ...form,
    vet_id: isAdmin ? form.vet_id || null : null,
  })

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      if (editingId !== null) {
        await api.put(`/api/pets/${editingId}`, payload())
      } else {
        await api.post('/api/pets', payload())
      }
      setForm(empty)
      setEditingId(null)
      setShowForm(false)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const edit = (p: Pet) => {
    setForm({
      owner_id: p.owner_id,
      vet_id: p.vet_id ?? 0,
      name: p.name,
      species: p.species,
      breed: p.breed,
      sex: p.sex,
      birth_date: p.birth_date,
      weight_kg: p.weight_kg,
      microchip_id: p.microchip_id,
      notes: p.notes,
    })
    setEditingId(p.id)
    setShowForm(true)
  }

  const reassign = async (p: Pet, vetID: string) => {
    setError('')
    try {
      await api.put(`/api/pets/${p.id}/vet`, { vet_id: vetID ? Number(vetID) : null })
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const remove = async (p: Pet) => {
    if (!confirm(`Remove ${p.name} and all their records from the clinic?`)) return
    try {
      await api.delete(`/api/pets/${p.id}`)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const q = search.trim().toLowerCase()
  const filtered = q
    ? pets.filter(
        (p) =>
          p.name.toLowerCase().includes(q) ||
          p.species.toLowerCase().includes(q) ||
          (p.owner_name ?? '').toLowerCase().includes(q),
      )
    : pets

  return (
    <div>
      <div className="toolbar">
        <h2>{isAdmin ? 'All patients' : 'My patients'}</h2>
        <input
          placeholder="Search by pet, species or owner…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        {isAdmin && (
          <select value={vetFilter} onChange={(e) => setVetFilter(e.target.value)}>
            <option value="">All staff</option>
            <option value="unassigned">Unassigned</option>
            {staff.map((v) => (
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
            setForm(empty)
          }}
        >
          {showForm ? 'Close' : '+ Register patient'}
        </button>
      </div>
      {!isAdmin && (
        <p className="muted">Showing only the patients assigned to you.</p>
      )}
      {error && <div className="error-banner">{error}</div>}

      {showForm && (
        <div className="card">
          <form className="entity-form" onSubmit={submit}>
            <label>
              Owner*
              <select
                value={form.owner_id}
                onChange={(e) => setForm({ ...form, owner_id: Number(e.target.value) })}
                required
              >
                <option value={0} disabled>
                  Choose owner…
                </option>
                {owners.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name} ({o.email})
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
                  {staff.map((v) => (
                    <option key={v.id} value={v.id}>
                      {v.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <label>
              Name*
              <input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                required
              />
            </label>
            <label>
              Species*
              <input
                value={form.species}
                placeholder="dog, cat, rabbit…"
                onChange={(e) => setForm({ ...form, species: e.target.value })}
                required
              />
            </label>
            <label>
              Breed
              <input
                value={form.breed}
                onChange={(e) => setForm({ ...form, breed: e.target.value })}
              />
            </label>
            <label>
              Sex
              <select
                value={form.sex}
                onChange={(e) => setForm({ ...form, sex: e.target.value as Pet['sex'] })}
              >
                <option value="unknown">Unknown</option>
                <option value="male">Male</option>
                <option value="female">Female</option>
              </select>
            </label>
            <label>
              Birth date
              <input
                type="date"
                value={form.birth_date}
                onChange={(e) => setForm({ ...form, birth_date: e.target.value })}
              />
            </label>
            <label>
              Weight (kg)
              <input
                type="number"
                step="0.1"
                min="0"
                value={form.weight_kg || ''}
                onChange={(e) => setForm({ ...form, weight_kg: Number(e.target.value) })}
              />
            </label>
            <label>
              Microchip ID
              <input
                value={form.microchip_id}
                onChange={(e) => setForm({ ...form, microchip_id: e.target.value })}
              />
            </label>
            <div className="form-actions">
              <button className="btn primary" type="submit">
                {editingId !== null ? 'Save' : 'Register'}
              </button>
            </div>
          </form>
        </div>
      )}

      {filtered.length === 0 ? (
        <p className="muted">No patients found.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Owner</th>
              <th>Species</th>
              <th>Breed</th>
              <th>Sex</th>
              <th>Born</th>
              {isAdmin && <th>Assigned to</th>}
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((p) => (
              <tr key={p.id}>
                <td>
                  <button className="link" onClick={() => onOpenPet(p.id)}>
                    {p.name}
                  </button>
                </td>
                <td>{p.owner_name}</td>
                <td>{p.species}</td>
                <td>{p.breed || '—'}</td>
                <td>{p.sex}</td>
                <td>{p.birth_date || '—'}</td>
                {isAdmin && (
                  <td>
                    <select
                      className="inline-select"
                      value={p.vet_id ?? ''}
                      onChange={(e) => reassign(p, e.target.value)}
                    >
                      <option value="">Unassigned</option>
                      {staff.map((v) => (
                        <option key={v.id} value={v.id}>
                          {v.name}
                        </option>
                      ))}
                    </select>
                  </td>
                )}
                <td className="row-actions">
                  <RowMenu
                    actions={[
                      { label: 'Open chart', onClick: () => onOpenPet(p.id) },
                      { label: 'Edit details', onClick: () => edit(p) },
                      { label: 'Remove', danger: true, onClick: () => remove(p) },
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
