import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import type { MedicalRecord, Pet, Vaccination, Vet } from '../api'

const emptyRecord = {
  visit_date: new Date().toISOString().slice(0, 10),
  category: 'exam',
  diagnosis: '',
  treatment: '',
  notes: '',
}

const emptyVaccination = {
  vaccine: '',
  administered_at: new Date().toISOString().slice(0, 10),
  next_due: '',
  notes: '',
}

export default function PetDetail({
  petId,
  onBack,
  ownerId,
  vet,
}: {
  petId: number
  onBack: () => void
  ownerId?: number
  vet?: Vet
}) {
  const [pet, setPet] = useState<Pet | null>(null)
  const [records, setRecords] = useState<MedicalRecord[]>([])
  const [vaccinations, setVaccinations] = useState<Vaccination[]>([])
  const [tab, setTab] = useState<'records' | 'vaccinations'>('records')
  const [recordForm, setRecordForm] = useState(emptyRecord)
  const [vaccinationForm, setVaccinationForm] = useState(emptyVaccination)
  const [showForm, setShowForm] = useState(false)
  const [error, setError] = useState('')

  const vetMode = !!vet
  const backLabel = vetMode ? '← Back to patients' : '← Back to my pets'

  const load = useCallback(() => {
    Promise.all([
      api.get<Pet>(`/api/pets/${petId}`),
      api.get<MedicalRecord[]>(`/api/pets/${petId}/records`),
      api.get<Vaccination[]>(`/api/vaccinations?pet_id=${petId}`),
    ])
      .then(([p, r, vx]) => {
        setPet(p)
        setRecords(r)
        setVaccinations(vx)
      })
      .catch((e) => setError(e.message))
  }, [petId])
  useEffect(load, [load])

  const addRecord = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      await api.post('/api/records', { ...recordForm, pet_id: petId, vet_id: vet!.id })
      setRecordForm(emptyRecord)
      setShowForm(false)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const addVaccination = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      await api.post('/api/vaccinations', {
        ...vaccinationForm,
        pet_id: petId,
        vet_id: vet!.id,
      })
      setVaccinationForm(emptyVaccination)
      setShowForm(false)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  if (error && !pet) return <div className="error-banner">{error}</div>
  if (!pet) return <p className="muted">Loading…</p>
  if (ownerId !== undefined && pet.owner_id !== ownerId) {
    return (
      <div>
        <button className="link" onClick={onBack}>
          {backLabel}
        </button>
        <div className="error-banner" style={{ marginTop: '1rem' }}>
          This pet is not registered to your account.
        </div>
      </div>
    )
  }

  return (
    <div>
      <button className="link" onClick={onBack}>
        {backLabel}
      </button>
      <h2>
        {pet.name} <span className="muted">({pet.species}{pet.breed ? `, ${pet.breed}` : ''})</span>
      </h2>
      <div className="card">
        {vetMode && (
          <>
            <strong>Owner:</strong> {pet.owner_name} &nbsp;·&nbsp;
          </>
        )}
        <strong>Sex:</strong> {pet.sex} &nbsp;·&nbsp;
        <strong>Born:</strong> {pet.birth_date || '—'} &nbsp;·&nbsp;
        <strong>Weight:</strong> {pet.weight_kg ? `${pet.weight_kg} kg` : '—'} &nbsp;·&nbsp;
        <strong>Microchip:</strong> {pet.microchip_id || '—'}
        {pet.notes && (
          <p className="muted" style={{ marginBottom: 0 }}>
            {pet.notes}
          </p>
        )}
      </div>

      <div className="toolbar">
        <div className="tabs">
          <button
            className={tab === 'records' ? 'active' : ''}
            onClick={() => {
              setTab('records')
              setShowForm(false)
            }}
          >
            Medical history ({records.length})
          </button>
          <button
            className={tab === 'vaccinations' ? 'active' : ''}
            onClick={() => {
              setTab('vaccinations')
              setShowForm(false)
            }}
          >
            Vaccinations ({vaccinations.length})
          </button>
        </div>
        {vetMode && (
          <button className="btn primary" onClick={() => setShowForm(!showForm)}>
            {showForm ? 'Close' : tab === 'records' ? '+ Add record' : '+ Add vaccination'}
          </button>
        )}
      </div>
      {error && <div className="error-banner">{error}</div>}

      {vetMode && showForm && tab === 'records' && (
        <div className="card">
          <form className="entity-form" onSubmit={addRecord}>
            <label>
              Visit date*
              <input
                type="date"
                value={recordForm.visit_date}
                onChange={(e) => setRecordForm({ ...recordForm, visit_date: e.target.value })}
                required
              />
            </label>
            <label>
              Category
              <select
                value={recordForm.category}
                onChange={(e) => setRecordForm({ ...recordForm, category: e.target.value })}
              >
                {['exam', 'injury', 'illness', 'dental', 'follow-up', 'other'].map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Diagnosis
              <input
                value={recordForm.diagnosis}
                onChange={(e) => setRecordForm({ ...recordForm, diagnosis: e.target.value })}
              />
            </label>
            <label>
              Treatment
              <input
                value={recordForm.treatment}
                onChange={(e) => setRecordForm({ ...recordForm, treatment: e.target.value })}
              />
            </label>
            <label>
              Notes
              <input
                value={recordForm.notes}
                onChange={(e) => setRecordForm({ ...recordForm, notes: e.target.value })}
              />
            </label>
            <div className="form-actions">
              <button className="btn primary" type="submit">
                Add record
              </button>
            </div>
          </form>
        </div>
      )}

      {vetMode && showForm && tab === 'vaccinations' && (
        <div className="card">
          <form className="entity-form" onSubmit={addVaccination}>
            <label>
              Vaccine*
              <input
                value={vaccinationForm.vaccine}
                onChange={(e) =>
                  setVaccinationForm({ ...vaccinationForm, vaccine: e.target.value })
                }
                required
              />
            </label>
            <label>
              Administered*
              <input
                type="date"
                value={vaccinationForm.administered_at}
                onChange={(e) =>
                  setVaccinationForm({ ...vaccinationForm, administered_at: e.target.value })
                }
                required
              />
            </label>
            <label>
              Next due
              <input
                type="date"
                value={vaccinationForm.next_due}
                onChange={(e) =>
                  setVaccinationForm({ ...vaccinationForm, next_due: e.target.value })
                }
              />
            </label>
            <label>
              Notes
              <input
                value={vaccinationForm.notes}
                onChange={(e) =>
                  setVaccinationForm({ ...vaccinationForm, notes: e.target.value })
                }
              />
            </label>
            <div className="form-actions">
              <button className="btn primary" type="submit">
                Add vaccination
              </button>
            </div>
          </form>
        </div>
      )}

      {tab === 'records' &&
        (records.length === 0 ? (
          <p className="muted">No medical records yet.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Date</th>
                <th>Category</th>
                <th>Diagnosis</th>
                <th>Treatment</th>
                <th>Vet</th>
              </tr>
            </thead>
            <tbody>
              {records.map((r) => (
                <tr key={r.id}>
                  <td>{r.visit_date}</td>
                  <td>{r.category}</td>
                  <td>{r.diagnosis || '—'}</td>
                  <td>{r.treatment || '—'}</td>
                  <td>{r.vet_name || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ))}

      {tab === 'vaccinations' &&
        (vaccinations.length === 0 ? (
          <p className="muted">No vaccinations recorded.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Vaccine</th>
                <th>Administered</th>
                <th>Next due</th>
                <th>Vet</th>
              </tr>
            </thead>
            <tbody>
              {vaccinations.map((v) => (
                <tr key={v.id}>
                  <td>{v.vaccine}</td>
                  <td>{v.administered_at}</td>
                  <td>{v.next_due || '—'}</td>
                  <td>{v.vet_name || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ))}
    </div>
  )
}
