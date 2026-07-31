const features = [
  {
    icon: '🐾',
    title: 'Your Pets, One Place',
    text: "Every pet on your account with its full medical history — visits, diagnoses, treatments and notes from the clinic.",
  },
  {
    icon: '🗓️',
    title: 'Appointments & Surgeries',
    text: 'See every procedure the clinic has scheduled for your pets — when, with which vet, and its current status.',
  },
  {
    icon: '💉',
    title: 'Vaccination Tracking',
    text: 'Every dose on record with due dates. Know at a glance what is current, due soon, or overdue.',
  },
  {
    icon: '📧',
    title: 'Automatic Reminders',
    text: 'The clinic emails you when a vaccination comes due within 14 days — nothing slips through.',
  },
  {
    icon: '🩺',
    title: 'Built for Vets Too',
    text: 'Veterinarians sign in to manage patients, schedule appointments and keep records up to date.',
  },
  {
    icon: '📊',
    title: 'Personal Dashboard',
    text: 'Your pets, upcoming appointments and due vaccinations the moment you sign in.',
  },
]

export default function Landing({
  onSignIn,
  onSignUp,
}: {
  onSignIn: () => void
  onSignUp: () => void
}) {
  return (
    <div className="landing">
      <header className="landing-nav">
        <div className="landing-brand">
          <span className="paw">🐾</span> VetCare
        </div>
        <div className="landing-nav-actions">
          <button className="btn" onClick={onSignUp}>
            Create Account
          </button>
          <button className="btn primary" onClick={onSignIn}>
            Sign In
          </button>
        </div>
      </header>

      <section className="hero">
        <div className="hero-badge">Pet Owner & Vet Portal</div>
        <h1>
          Modern care for every patient,
          <br />
          on two legs or four.
        </h1>
        <p className="hero-sub">
          VetCare keeps you close to your pets' health — medical histories,
          upcoming appointments and vaccination reminders in one calm, fast
          place.
        </p>
        <div className="hero-actions">
          <button className="btn primary large" onClick={onSignIn}>
            Sign In
          </button>
          <button className="btn large ghost" onClick={onSignUp}>
            Create Account
          </button>
        </div>
      </section>

      <section id="features" className="features-grid">
        {features.map((f) => (
          <div key={f.title} className="feature card">
            <div className="feature-icon">{f.icon}</div>
            <h3>{f.title}</h3>
            <p>{f.text}</p>
          </div>
        ))}
      </section>

      <section className="landing-cta card">
        <h2>Ready to check in on your pets?</h2>
        <p className="muted">Sign in to see their records, appointments and reminders.</p>
        <button className="btn primary large" onClick={onSignIn}>
          Sign In
        </button>
      </section>

      <footer className="landing-foot">
        VetCare — Veterinary Clinic Portal
      </footer>
    </div>
  )
}
