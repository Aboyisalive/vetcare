export interface Owner {
  id: number
  name: string
  email: string
  phone: string
  address: string
  created_at: string
  pet_count?: number
}

export interface Pet {
  id: number
  owner_id: number
  owner_name?: string
  vet_id: number | null
  vet_name?: string
  name: string
  species: string
  breed: string
  sex: 'male' | 'female' | 'unknown'
  birth_date: string
  weight_kg: number
  microchip_id: string
  notes: string
  created_at: string
}

export interface Vet {
  id: number
  name: string
  specialty: string
  email: string
  phone: string
  // Only the staff directory (GET /api/vets) fills these in.
  is_admin?: boolean
  patient_count?: number
  upcoming_count?: number
}

export interface MedicalRecord {
  id: number
  pet_id: number
  pet_name?: string
  vet_id: number | null
  vet_name?: string
  visit_date: string
  category: string
  diagnosis: string
  treatment: string
  notes: string
  created_at: string
}

export interface Surgery {
  id: number
  pet_id: number
  pet_name?: string
  vet_id: number | null
  vet_name?: string
  procedure: string
  scheduled_at: string
  duration_min: number
  // 'requested' is an owner's booking waiting for the vet to accept it.
  status: 'requested' | 'scheduled' | 'completed' | 'cancelled' | 'declined'
  notes: string
  created_at: string
}

/** One eligible vet and the start times they are free on the requested day. */
export interface VetSlots {
  vet_id: number
  vet_name: string
  specialty: string
  slots: string[]
}

export interface Availability {
  date: string
  duration_min: number
  closed: boolean
  vets: VetSlots[]
}

export interface Vaccination {
  id: number
  pet_id: number
  pet_name?: string
  vet_id: number | null
  vet_name?: string
  vaccine: string
  administered_at: string
  next_due: string
  notes: string
  created_at: string
}

export interface Reminder {
  id: number
  vaccination_id: number
  vaccine?: string
  pet_name?: string
  owner_name?: string
  owner_email?: string
  due_date: string
  status: 'pending' | 'sent' | 'failed'
  channel: string
  message: string
  sent_at: string
  created_at: string
}

export interface AuthUser {
  id: number
  email: string
  role: 'owner' | 'vet'
  is_admin: boolean
  owner?: Owner
  vet?: Vet
}

export interface SignupData {
  role: 'owner' | 'vet'
  name: string
  email: string
  password: string
  phone?: string
  address?: string
  specialty?: string
}

export interface Stats {
  owners: number
  pets: number
  vets: number
  upcoming_surgeries: number
  vaccinations_due_soon: number
  pending_reminders: number
  pending_requests: number
}

// Empty in dev, where vite proxies /api to localhost:8080. In production the
// static site and the API are separate Render services, so calls need the
// absolute backend URL (see .env.production).
const API_BASE = import.meta.env.VITE_API_URL ?? ''

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE + path, {
    headers: { 'Content-Type': 'application/json' },
    // The session cookie is cross-site once the two services are split apart.
    credentials: 'include',
    ...options,
  })
  if (res.status === 204) return undefined as T
  const body = await res.json()
  if (!res.ok) throw new Error(body.error ?? `HTTP ${res.status}`)
  return body as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, data: unknown) =>
    request<T>(path, { method: 'POST', body: JSON.stringify(data) }),
  put: <T>(path: string, data: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (path: string) => request<void>(path, { method: 'DELETE' }),
}

export const auth = {
  me: () => api.get<AuthUser>('/api/auth/me'),
  login: (email: string, password: string) =>
    api.post<AuthUser>('/api/auth/login', { email, password }),
  signup: (data: SignupData) => api.post<AuthUser>('/api/auth/signup', data),
  logout: () => api.post<void>('/api/auth/logout', {}),
}
