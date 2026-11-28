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

/** One apartment of the signed-in user, with its house: the cabinet's "My home". */
export interface Home {
  id: number
  houseName: string
  address: string
  floors: number | null
  apartment: number
  statusNames: LocalizedText
}

export interface MyHome {
  homes: Home[]
  telegramLinked: boolean
}

export interface AdminUser extends Me {
  status: UserStatus
  source: 'application' | 'admin' | 'bootstrap'
  hasTelegram: boolean
  /** Stored MCP flag; effective only together with Admin on an active user. */
  isMcp?: boolean
  createdAt: string
  approvedAt?: string
  activatedAt?: string
}

export interface Site {
  communityName: LocalizedText
  welcomeText: LocalizedText
  defaultLocale: Locale
  botUrl: string
  registerUrl: string
  /** The published home page, framed under our header; null shows the built-in page. */
  pageUrl: string | null
  /** Site paths of the published pages ("/", "/contacts"); [] when nothing is published. */
  pages: string[]
  /** Public data items shown as cards on the built-in page, in order. */
  homeItems: HomeItem[]
}

export interface HomeItem {
  key: string
  /** Card title; empty falls back to the prettified key. */
  title: LocalizedText
}

export interface HouseOption {
  id: number
  name: string
  apartmentFrom: number
  apartmentTo: number
  /** For the built-in page's quarter; null draws a default height. */
  floors: number | null
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

/** An AI agent connected through MCP on behalf of a user. */
export interface McpGrant {
  id: number
  clientName: string
  createdAt: string
  lastUsedAt: string | null
}

export interface UserCard {
  user: AdminUser
  channels: Channel[]
  residencies: Residency[]
  audit: AuditEntry[]
  mcpGrants?: McpGrant[]
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
export const valueTypes: DataType[] = ['text', 'localized_text', 'number', 'bool', 'image', 'url', 'phone', 'json']
export const columnTypes: DataType[] = ['text', 'localized_text', 'number', 'bool', 'image', 'url', 'phone']

export interface DataColumn {
  name: string
  label: string
  type: DataType
}

/** Who reads an item outside the admin panel: everyone, signed-in residents (and admins), or admins only. */
export type Visibility = 'public' | 'residents' | 'admins'
export const visibilities: Visibility[] = ['public', 'residents', 'admins']

/** A value or a table of the data space; value is JSON (image values are asset ids). */
export interface DataItem {
  key: string
  kind: DataKind
  description: string
  visibility: Visibility
  valueType?: DataType
  value?: unknown
  columns?: DataColumn[]
  rowCount: number
  updatedAt: string
  updatedBy: { id: number; name: string } | null
  /** Code of the module that writes this item ("feed"); null or absent for admins' own items. */
  owner?: string | null
  /** Whether the owning module is turned on; owned items of a module that is off are not served. */
  ownerEnabled?: boolean
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
export interface SitePage {
  name: string
  path: string
}

export interface PageVersion {
  id: number
  status: PageStatus
  note: string
  files: PageFileInfo[]
  /** Root *.html files that are pages: index → "/", contacts → "/contacts". */
  pages: SitePage[]
  totalSize: number
  createdAt: string
  createdBy: { id: number; name: string } | null
  /** The agent's client name when written through MCP; empty otherwise. */
  createdVia: string
  publishedAt: string | null
  publishedBy: { id: number; name: string } | null
}

/** A cabinet navigation entry of a module; icon is a name the front end maps to an icon. */
export interface ModuleNavEntry {
  key: string
  path: string
  labelKey: string
  icon: string
}

/** A module that is turned on, as the signed-in user sees it. */
export interface CabinetModule {
  code: string
  nav: ModuleNavEntry[]
}

/** A built-in module in the admin "Modules" section. */
export interface AdminModule {
  code: string
  nameKey: string
  descriptionKey: string
  enabled: boolean
  defaultEnabled: boolean
  updatedAt: string | null
  updatedBy: { id: number; name: string } | null
}

export interface FeedSettings {
  defaultLabelIds: number[]
}

export type PostState = 'draft' | 'published'
export type DeliveryState = 'pending' | 'sent' | 'failed'

/** One Telegram copy of a published post (a chat or a forum topic). */
export interface PostDelivery {
  id: number
  chatTitle: string
  /** '' for the chat itself. */
  topicTitle: string
  labelId: number | null
  state: DeliveryState
  error: string
  sentAt: string | null
}

/** A feed post as admins see it. */
export interface AdminPost {
  id: number
  body: string
  imageAssetId: number | null
  imageUrl: string | null
  state: PostState
  publishedAt: string | null
  labelIds: number[]
  createdAt: string
  updatedAt: string
  createdBy: { id: number; name: string } | null
  updatedBy: { id: number; name: string } | null
  delivery: { pending: number; sent: number; failed: number }
  /** The outbox no longer keeps this post's delivery rows. */
  deliveryExpired: boolean
  /** Only in the answer for one post. */
  deliveries?: PostDelivery[]
}

/** A published feed post in the cabinet. */
export interface CabinetPost {
  id: number
  body: string
  imageUrl: string | null
  publishedAt: string
  /** The author's first name. */
  authorName: string
}
