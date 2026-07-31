import { useEffect, useState } from 'react'
import { api } from '../api'
import type { Owner, Reminder } from '../api'

export default function Reminders({ owner }: { owner: Owner }) {
  const [reminders, setReminders] = useState<Reminder[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .get<Reminder[]>('/api/reminders')
      .then((all) => setReminders(all.filter((r) => r.owner_email === owner.email)))
      .catch((e) => setError(e.message))
  }, [owner.email])

  return (
    <div>
      <div className="toolbar">
        <h2>My Reminders</h2>
      </div>
      {error && <div className="error-banner">{error}</div>}
      <p className="muted">
        The clinic checks daily for vaccinations due within 14 days and emails you a reminder
        at {owner.email || 'your email on file'}.
      </p>
      {reminders.length === 0 ? (
        <p className="muted">No reminders for you yet.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Due</th>
              <th>Pet</th>
              <th>Vaccine</th>
              <th>Status</th>
              <th>Sent at</th>
            </tr>
          </thead>
          <tbody>
            {reminders.map((r) => (
              <tr key={r.id}>
                <td>{r.due_date}</td>
                <td>{r.pet_name}</td>
                <td>{r.vaccine}</td>
                <td>
                  <span className={`badge ${r.status}`}>{r.status}</span>
                </td>
                <td>{r.sent_at || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
