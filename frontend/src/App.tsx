import { useEffect, useState } from 'react'
import Landing from './pages/Landing'
import AuthPage from './pages/AuthPage'
import Dashboard from './pages/Dashboard'
import Pets from './pages/Pets'
import PetDetail from './pages/PetDetail'
import Surgeries from './pages/Surgeries'
import Vaccinations from './pages/Vaccinations'
import Reminders from './pages/Reminders'
import Vets from './pages/Vets'
import VetDashboard from './pages/VetDashboard'
import VetPatients from './pages/VetPatients'
import VetAppointments from './pages/VetAppointments'
import VetStaff from './pages/VetStaff'
import { auth } from './api'
import type { AuthUser } from './api'

const ownerViews = [
  'Dashboard',
  'My Pets',
  'Appointments',
  'Vaccinations',
  'Reminders',
  'My Vets',
] as const

const vetViews = ['Dashboard', 'Patients', 'Appointments'] as const

// Administrators see the whole clinic and additionally manage the staff roster.
const adminViews = ['Dashboard', 'Patients', 'Appointments', 'Staff'] as const

type View =
  | (typeof ownerViews)[number]
  | (typeof vetViews)[number]
  | (typeof adminViews)[number]

export default function App() {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [ready, setReady] = useState(false)
  const [view, setView] = useState<View>('Dashboard')
  const [petId, setPetId] = useState<number | null>(null)
  const [screen, setScreen] = useState<'landing' | 'auth'>('landing')
  const [authMode, setAuthMode] = useState<'signin' | 'signup'>('signin')

  useEffect(() => {
    auth
      .me()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setReady(true))
  }, [])

  const onAuth = (u: AuthUser) => {
    setUser(u)
    setView('Dashboard')
    setPetId(null)
    setScreen('landing')
  }

  const signOut = () => {
    auth.logout().finally(() => {
      setUser(null)
      setPetId(null)
      setView('Dashboard')
      setScreen('landing')
    })
  }

  const openAuth = (mode: 'signin' | 'signup') => {
    setAuthMode(mode)
    setScreen('auth')
  }

  const petsView = user?.role === 'vet' ? 'Patients' : 'My Pets'

  const openPet = (id: number) => {
    setPetId(id)
    setView(petsView)
  }

  const navigate = (v: View) => {
    setPetId(null)
    setView(v)
  }

  if (!ready) return null
  if (!user) {
    return screen === 'auth' ? (
      <AuthPage mode={authMode} onAuth={onAuth} onBack={() => setScreen('landing')} />
    ) : (
      <Landing onSignIn={() => openAuth('signin')} onSignUp={() => openAuth('signup')} />
    )
  }

  const isVet = user.role === 'vet'
  const isAdmin = isVet && user.is_admin
  const views: readonly View[] = isAdmin ? adminViews : isVet ? vetViews : ownerViews
  const owner = user.owner
  const displayName = isVet ? user.vet?.name ?? user.email : owner?.name ?? user.email
  const tagline = isAdmin
    ? 'Clinic Administration'
    : isVet
      ? 'Veterinarian Portal'
      : 'Pet Owner Portal'

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">
          <h1>VetCare</h1>
          <div className="tagline">{tagline}</div>
        </div>
        <nav>
          {views.map((v) => (
            <button
              key={v}
              className={view === v ? 'active' : ''}
              onClick={() => navigate(v)}
            >
              {v}
            </button>
          ))}
        </nav>
        <div className="foot">
          <div className="signed-in">{displayName}</div>
          <button className="link" onClick={signOut}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="content">
        {isVet ? (
          <>
            {view === 'Dashboard' && <VetDashboard isAdmin={isAdmin} onOpenPet={openPet} />}
            {view === 'Patients' && petId === null && (
              <VetPatients isAdmin={isAdmin} onOpenPet={openPet} />
            )}
            {view === 'Patients' && petId !== null && (
              <PetDetail petId={petId} vet={user.vet} onBack={() => setPetId(null)} />
            )}
            {view === 'Appointments' && (
              <VetAppointments isAdmin={isAdmin} vet={user.vet} />
            )}
            {view === 'Staff' && isAdmin && <VetStaff />}
          </>
        ) : (
          owner && (
            <>
              {view === 'Dashboard' && <Dashboard owner={owner} onOpenPet={openPet} />}
              {view === 'My Pets' && petId === null && (
                <Pets owner={owner} onOpenPet={openPet} />
              )}
              {view === 'My Pets' && petId !== null && (
                <PetDetail petId={petId} ownerId={owner.id} onBack={() => setPetId(null)} />
              )}
              {view === 'Appointments' && <Surgeries owner={owner} />}
              {view === 'Vaccinations' && (
                <Vaccinations owner={owner} onOpenPet={openPet} />
              )}
              {view === 'Reminders' && <Reminders owner={owner} />}
              {view === 'My Vets' && <Vets owner={owner} />}
            </>
          )
        )}
      </main>
    </div>
  )
}
