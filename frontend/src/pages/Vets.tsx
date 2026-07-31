import { useEffect, useState } from 'react'
import { api } from '../api'
import type { MedicalRecord, Owner, Pet, Surgery, Vaccination, Vet } from '../api'

export default function Vets({ owner }: { owner: Owner }) {
  const [vets, setVets] = useState<Vet[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const run = async () => {
      try {
        const [allVets, pets, surgeries, vaccinations] = await Promise.all([
          api.get<Vet[]>('/api/vets'),
          api.get<Pet[]>(`/api/pets?owner_id=${owner.id}`),
          api.get<Surgery[]>('/api/surgeries'),
          api.get<Vaccination[]>('/api/vaccinations'),
        ])
        const records = (
          await Promise.all(
            pets.map((p) => api.get<MedicalRecord[]>(`/api/pets/${p.id}/records`)),
          )
        ).flat()

        const petIds = new Set(pets.map((p) => p.id))
        const assigned = new Set<number>()
        // The staff member each pet is currently registered with.
        for (const p of pets) {
          if (p.vet_id !== null) assigned.add(p.vet_id)
        }
        for (const s of surgeries) {
          if (petIds.has(s.pet_id) && s.vet_id !== null) assigned.add(s.vet_id)
        }
        for (const v of vaccinations) {
          if (petIds.has(v.pet_id) && v.vet_id !== null) assigned.add(v.vet_id)
        }
        for (const r of records) {
          if (r.vet_id !== null) assigned.add(r.vet_id)
        }

        setVets(allVets.filter((v) => assigned.has(v.id)))
        setLoaded(true)
      } catch (e) {
        setError((e as Error).message)
      }
    }
    run()
  }, [owner.id])

  if (error) return <div className="error-banner">{error}</div>
  if (!loaded) return <p className="muted">Loading…</p>

  return (
    <div>
      <div className="toolbar">
        <h2>My Vets</h2>
      </div>
      <p className="muted">
        The veterinarians who have treated or are scheduled to treat your pets.
      </p>
      {vets.length === 0 ? (
        <p className="muted">No vets have been assigned to your pets yet.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Specialty</th>
              <th>Email</th>
              <th>Phone</th>
            </tr>
          </thead>
          <tbody>
            {vets.map((v) => (
              <tr key={v.id}>
                <td>{v.name}</td>
                <td>{v.specialty || '—'}</td>
                <td>{v.email || '—'}</td>
                <td>{v.phone || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
