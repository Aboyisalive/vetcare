import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import type { Owner, Pet } from '../api'
import RowMenu from '../components/RowMenu'

const empty = {
  name: '',
  species: '',
  breed: '',
  sex: 'unknown' as Pet['sex'],
  birth_date: '',
  weight_kg: 0,
  microchip_id: '',
  notes: '',
}

export default function Pets({
  owner,
  onOpenPet,
}: {
  owner: Owner
  onOpenPet: (id: number) => void
}) {
  const [pets, setPets] = useState<Pet[]>([])
  const [form, setForm] = useState(empty)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(() => {
    api
      .get<Pet[]>(`/api/pets?owner_id=${owner.id}`)
      .then(setPets)
      .catch((e) => setError(e.message))
  }, [owner.id])
  useEffect(load, [load])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      const data = { ...form, owner_id: owner.id }
      if (editingId !== null) {
        await api.put(`/api/pets/${editingId}`, data)
      } else {
        await api.post('/api/pets', data)
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

  const remove = async (p: Pet) => {
    if (!confirm(`Remove ${p.name} and all their records from your account?`)) return
    try {
      await api.delete(`/api/pets/${p.id}`)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <div>
      <div className="toolbar">
        <h2>My Pets</h2>
        <button
          className="btn primary"
          onClick={() => {
            setShowForm(!showForm)
            setEditingId(null)
            setForm(empty)
          }}
        >
          {showForm ? 'Close' : '+ Register pet'}
        </button>
      </div>
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

      {pets.length === 0 ? (
        <p className="muted">No pets registered on your account yet.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Species</th>
              <th>Breed</th>
              <th>Sex</th>
              <th>Born</th>
              <th>Vet</th>
              <th></th>
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
                <td>{p.sex}</td>
                <td>{p.birth_date || '—'}</td>
                <td>{p.vet_name || 'Not yet assigned'}</td>
                <td className="row-actions">
                  <RowMenu
                    actions={[
                      { label: 'View', onClick: () => onOpenPet(p.id) },
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
