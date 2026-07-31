import { useState } from 'react'
import { auth } from '../api'
import type { AuthUser, SignupData } from '../api'

const emptySignup: SignupData = {
  role: 'owner',
  name: '',
  email: '',
  password: '',
  phone: '',
  address: '',
  specialty: '',
}

export default function AuthPage({
  mode: initialMode,
  onAuth,
  onBack,
}: {
  mode: 'signin' | 'signup'
  onAuth: (u: AuthUser) => void
  onBack: () => void
}) {
  const [mode, setMode] = useState(initialMode)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [signup, setSignup] = useState<SignupData>(emptySignup)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const switchMode = (m: 'signin' | 'signup') => {
    setMode(m)
    setError('')
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      onAuth(mode === 'signin' ? await auth.login(email, password) : await auth.signup(signup))
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-page">
      <button className="auth-brand" onClick={onBack}>
        <span className="paw">🐾</span> VetCare
      </button>

      <div className="auth-card card">
        <div className="tabs auth-tabs">
          <button
            className={mode === 'signin' ? 'active' : ''}
            onClick={() => switchMode('signin')}
          >
            Sign in
          </button>
          <button
            className={mode === 'signup' ? 'active' : ''}
            onClick={() => switchMode('signup')}
          >
            Create account
          </button>
        </div>

        <h2>{mode === 'signin' ? 'Welcome back' : 'Join VetCare'}</h2>
        <p className="muted auth-sub">
          {mode === 'signin'
            ? 'Sign in to your owner or veterinarian account.'
            : signup.role === 'owner'
              ? 'Track your pets’ health, appointments and vaccinations.'
              : 'Manage patients, records and the clinic schedule.'}
        </p>

        {error && <div className="error-banner">{error}</div>}

        <form className="auth-form" onSubmit={submit}>
          {mode === 'signup' && (
            <>
              <div className="role-toggle">
                <button
                  type="button"
                  className={signup.role === 'owner' ? 'active' : ''}
                  onClick={() => setSignup({ ...signup, role: 'owner' })}
                >
                  <span className="role-icon">🐾</span>
                  <span>
                    <strong>Pet owner</strong>
                    <small>My pets’ care</small>
                  </span>
                </button>
                <button
                  type="button"
                  className={signup.role === 'vet' ? 'active' : ''}
                  onClick={() => setSignup({ ...signup, role: 'vet' })}
                >
                  <span className="role-icon">🩺</span>
                  <span>
                    <strong>Veterinarian</strong>
                    <small>Clinic staff</small>
                  </span>
                </button>
              </div>
              <label>
                Full name
                <input
                  value={signup.name}
                  onChange={(e) => setSignup({ ...signup, name: e.target.value })}
                  placeholder={signup.role === 'vet' ? 'Dr. Jane Doe' : 'Jane Doe'}
                  required
                />
              </label>
            </>
          )}

          <label>
            Email
            <input
              type="email"
              value={mode === 'signin' ? email : signup.email}
              onChange={(e) =>
                mode === 'signin'
                  ? setEmail(e.target.value)
                  : setSignup({ ...signup, email: e.target.value })
              }
              placeholder="you@example.com"
              required
            />
          </label>

          <label>
            Password
            <input
              type="password"
              value={mode === 'signin' ? password : signup.password}
              onChange={(e) =>
                mode === 'signin'
                  ? setPassword(e.target.value)
                  : setSignup({ ...signup, password: e.target.value })
              }
              placeholder={mode === 'signup' ? 'At least 8 characters' : '••••••••'}
              minLength={mode === 'signup' ? 8 : undefined}
              required
            />
          </label>

          {mode === 'signup' && (
            <>
              <label>
                Phone <span className="optional">optional</span>
                <input
                  value={signup.phone}
                  onChange={(e) => setSignup({ ...signup, phone: e.target.value })}
                  placeholder="555-0100"
                />
              </label>
              {signup.role === 'owner' ? (
                <label>
                  Address <span className="optional">optional</span>
                  <input
                    value={signup.address}
                    onChange={(e) => setSignup({ ...signup, address: e.target.value })}
                    placeholder="12 Maple St"
                  />
                </label>
              ) : (
                <label>
                  Specialty <span className="optional">optional</span>
                  <input
                    value={signup.specialty}
                    onChange={(e) => setSignup({ ...signup, specialty: e.target.value })}
                    placeholder="Surgery, General…"
                  />
                </label>
              )}
            </>
          )}

          <button className="btn primary large auth-submit" type="submit" disabled={busy}>
            {busy
              ? 'One moment…'
              : mode === 'signin'
                ? 'Sign In'
                : signup.role === 'owner'
                  ? 'Create owner account'
                  : 'Create vet account'}
          </button>
        </form>

        <p className="auth-alt muted">
          {mode === 'signin' ? (
            <>
              New to VetCare?{' '}
              <button className="link" onClick={() => switchMode('signup')}>
                Create an account
              </button>
            </>
          ) : (
            <>
              Already have an account?{' '}
              <button className="link" onClick={() => switchMode('signin')}>
                Sign in
              </button>
            </>
          )}
        </p>
      </div>

      <button className="link auth-back" onClick={onBack}>
        ← Back to home
      </button>
    </div>
  )
}
