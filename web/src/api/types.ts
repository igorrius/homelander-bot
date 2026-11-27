// Shapes of the JSON API (internal/httpapi), field names as the server sends them.
import type { Locale } from '../i18n'

export type { Locale }
export type LocalizedText = Partial<Record<Locale, string>>
export type UserStatus = 'pending_approval' | 'awaiting_activation' | 'active' | 'rejected' | 'disabled'
export const userStatuses: UserStatus[] = ['pending_approval', 'awaiting_activation', 'active', 'rejected', 'disabled']

export interface Residency {
  id: number
  houseId: number
  houseName: string
  apartment: number
  statusId: number
  statusNames: LocalizedText
}

export interface Me {
  id: number
  phone: string
  firstName: string
  lastName: string
  locale: string
  isAdmin: boolean
  isResident: boolean
  residencies: Residency[]
}

export interface AdminUser extends Me {
  status: UserStatus
  source: 'application' | 'admin' | 'bootstrap'
  hasTelegram: boolean
  createdAt: string
  approvedAt?: string
  activatedAt?: string
}

export interface Site {
  communityName: LocalizedText
  welcomeText: LocalizedText
  defaultLocale: Locale
  pictureUrl: string | null
  botUrl: string
  registerUrl: string
}

export interface HouseOption {
  id: number
  name: string
  apartmentFrom: number
  apartmentTo: number
}

export interface ResidencyStatus {
  id: number
  code: string
  names: LocalizedText
  sortOrder: number
  archived: boolean
}

export interface RegisterOptions {
  houses: HouseOption[]
  statuses: ResidencyStatus[]
}

export interface House extends HouseOption {
  address: string
  latitude: number | null
  longitude: number | null
  floors: number | null
  sortOrder: number
  archived: boolean
}

export interface HouseStats {
  apartmentsTotal: number
  apartmentsWithResidents: number
  people: number
  byResidencyStatus: Record<string, number>
  byUserStatus: Record<string, number>
  apartments: Record<string, number>
}

export interface Channel {
  id: number
  type: string
  username: string
  linkedAt: string
}

export interface AuditEntry {
  id: number
  at: string
  action: string
  target: string
  actorUserId: number | null
  details: Record<string, unknown> | null
  ip: string
}

export interface UserCard {
  user: AdminUser
  channels: Channel[]
  residencies: Residency[]
  audit: AuditEntry[]
}

export interface Page<T> {
  items: T[]
  total: number
}

/** Counters for the admin menu and overview. */
export interface Summary {
  pendingApplications: number
  awaitingActivation: number
  activeUsers: number
}
