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
  /** The published home page, framed under our header; null shows the built-in page. */
  pageUrl: string | null
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
  /** A user's or house's name when the target is one, else "". */
  targetName: string
  /** A residency status's names when the target is one. */
  targetNames: LocalizedText | null
  actorUserId: number | null
  actorName: string
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
  disabledUsers: number
  rejectedUsers: number
  /** Chats the bot is in that no admin has accepted yet. */
  newChats?: number
}

export type ChatState = 'new' | 'accepted' | 'left'
export type BotStatus = 'member' | 'administrator' | 'restricted' | 'unknown'
export type LabelTargetType = 'chat' | 'chat_topic'

/** A forum topic; the General topic is the chat itself. */
export interface Topic {
  id: number
  externalId: string
  /** '' when the bot has not seen its name yet. */
  title: string
  titleSetByAdmin: boolean
  gone: boolean
  lastSeenAt: string
  labelIds: number[]
}

/** A group, forum or channel the bot is or was in. */
export interface Chat {
  id: number
  messenger: string
  kind: 'group' | 'supergroup' | 'channel'
  title: string
  isForum: boolean
  state: ChatState
  botStatus: BotStatus
  canSend: boolean
  addedBy: { externalId: string; userId: number | null }
  leaveAt: string | null
  leftAt: string | null
  firstSeenAt: string
  lastSeenAt: string
  labelIds: number[]
  topics: Topic[]
}

export interface Label {
  id: number
  name: string
  description: string
  targets: { type: LabelTargetType; id: number }[]
  usedByApplications: boolean
}

export interface NotificationSettings {
  applicationsLabelId: number | null
}

export interface OutboxStatus {
  state: 'pending' | 'sent' | 'failed'
  lastError: string
}

export type DataKind = 'value' | 'table'
export type DataType = 'text' | 'localized_text' | 'number' | 'bool' | 'image' | 'json' | 'url' | 'phone'
export const valueTypes: DataType[] = ['text', 'localized_text', 'number', 'bool', 'image', 'json']
export const columnTypes: DataType[] = ['text', 'localized_text', 'number', 'bool', 'image', 'url', 'phone']

export interface DataColumn {
  name: string
  label: string
  type: DataType
}

/** A value or a table of the data space; value is JSON (image values are asset ids). */
export interface DataItem {
  key: string
  kind: DataKind
  description: string
  public: boolean
  valueType?: DataType
  value?: unknown
  columns?: DataColumn[]
  rowCount: number
  updatedAt: string
  updatedBy: { id: number; name: string } | null
}

export interface DataRow {
  id?: number
  position: number
  cells: Record<string, unknown>
}

/** Where a home page version is: drafts are editable, published and archived ones are not. */
export type PageStatus = 'draft' | 'published' | 'archived'

export interface PageFileInfo {
  path: string
  contentType: string
  size: number
}

/** One uploaded set of home page files. */
export interface PageVersion {
  id: number
  status: PageStatus
  note: string
  files: PageFileInfo[]
  totalSize: number
  createdAt: string
  createdBy: { id: number; name: string } | null
  publishedAt: string | null
  publishedBy: { id: number; name: string } | null
}
