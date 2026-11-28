import type { AdminModule, AdminPost, AdminUser, CabinetModule, CabinetPost, Chat, DataItem, DataRow, House, Label, Me, MyHome, RegisterOptions, ResidencyStatus, Site, UserCard } from '../api/types'

// Synthetic data only: no real names, phones or addresses.
export const site: Site = {
  communityName: { uk: 'Тестова громада', en: 'Test Community' },
  welcomeText: { uk: 'Привіт' },
  defaultLocale: 'uk',
  botUrl: 'https://t.me/test_bot',
  registerUrl: 'http://localhost/register',
  pageUrl: null,
  pages: [],
  homeItems: [],
}

export const statuses: ResidencyStatus[] = [
  { id: 1, code: 'owner', names: { uk: 'Власник', en: 'Owner' }, sortOrder: 1, archived: false },
  { id: 2, code: 'tenant', names: { uk: 'Орендар', en: 'Tenant' }, sortOrder: 2, archived: false },
]

export const houses: House[] = [
  { id: 1, name: 'House A', address: '', latitude: null, longitude: null, floors: null, apartmentFrom: 1, apartmentTo: 40, sortOrder: 0, archived: false },
  { id: 2, name: 'House B', address: '', latitude: null, longitude: null, floors: null, apartmentFrom: 41, apartmentTo: 80, sortOrder: 1, archived: false },
]

export const registerOptions: RegisterOptions = {
  houses: houses.map(({ id, name, apartmentFrom, apartmentTo, floors }) => ({ id, name, apartmentFrom, apartmentTo, floors })),
  statuses,
}

export const resident: Me = {
  id: 5,
  phone: '+380500000002',
  firstName: 'Olena',
  lastName: 'Test',
  locale: 'uk',
  isAdmin: false,
  isResident: true,
  residencies: [{ id: 11, houseId: 1, houseName: 'House A', apartment: 12, statusId: 1, statusNames: statuses[0].names }],
}

export const admin: Me = { ...resident, id: 1, phone: '+380500000001', firstName: 'Adam', lastName: 'Admin', isAdmin: true, isResident: false, residencies: [] }

export function adminUser(over: Partial<AdminUser> = {}): AdminUser {
  return { ...resident, status: 'active', source: 'application', hasTelegram: true, createdAt: '2026-11-20T10:00:00Z', ...over }
}

export function userCard(over: Partial<AdminUser> = {}): UserCard {
  return {
    user: adminUser(over),
    channels: [{ id: 9, type: 'telegram', username: 'olena_test', linkedAt: '2026-11-21T10:00:00Z' }],
    residencies: resident.residencies,
    labelIds: [],
    audit: [{ id: 1, at: '2026-11-21T09:00:00Z', action: 'user.approved', target: 'user:5', targetName: 'Olena Test', targetNames: null, actorUserId: 1, actorName: 'Adam Admin', details: null, ip: '127.0.0.1' }],
  }
}

/** Routes every page needs: site, register options and a guest /me. */
export const publicRoutes = {
  'GET /site': { body: site },
  'GET /register/options': { body: registerOptions },
  'GET /me': { status: 401, body: { error: { code: 'unauthorized' } } },
}

/** The resident's "My home" in the cabinet. */
export const myHome: MyHome = {
  telegramLinked: true,
  homes: [{ id: 11, houseName: 'House A', address: '1 Test Street', floors: 9, apartment: 12, statusNames: statuses[0].names }],
}

/** The feed as GET /modules lists it when it is on. */
export const feedCabinetModule: CabinetModule = {
  code: 'feed',
  nav: [{ key: 'feed', path: '/my/feed', labelKey: 'cabinet.nav.feed', icon: 'news' }],
}

/** The feed module in the admin "Modules" section. */
export function feedModule(over: Partial<AdminModule> = {}): AdminModule {
  return {
    code: 'feed', nameKey: 'modules.feed.name', descriptionKey: 'modules.feed.description', enabled: true, defaultEnabled: true,
    updatedAt: null, updatedBy: null, ...over,
  }
}

/** Routes for a resident session in the cabinet (no module turned on). */
export const residentRoutes = {
  ...publicRoutes,
  'GET /me': { body: resident },
  'GET /me/home': { body: myHome },
  'GET /modules': { body: [] },
}

/** Routes for an admin session with an empty queue; the feed module is on. */
export const adminRoutes = {
  ...publicRoutes,
  'GET /me': { body: admin },
  'GET /admin/summary': { body: { pendingApplications: 0, awaitingActivation: 0, activeUsers: 1, disabledUsers: 0, rejectedUsers: 0 } },
  'GET /admin/users': { body: { items: [], total: 0 } },
  'GET /admin/houses': { body: houses },
  'GET /admin/statuses': { body: statuses },
  'GET /admin/modules': { body: [feedModule()] },
}

/** A feed post as admins see it: a draft by default. */
export function adminPost(over: Partial<AdminPost> = {}): AdminPost {
  return {
    id: 1, body: 'Water is off on Friday', imageAssetId: null, imageUrl: null, state: 'draft', publishedAt: null, labelIds: [],
    createdAt: '2026-11-27T10:00:00Z', updatedAt: '2026-11-27T10:00:00Z', createdBy: { id: 1, name: 'Adam Admin' }, updatedBy: null,
    delivery: { pending: 0, sent: 0, failed: 0 }, deliveryExpired: false, ...over,
  }
}

/** A published post in the cabinet. */
export function cabinetPost(over: Partial<CabinetPost> = {}): CabinetPost {
  return { id: 1, body: 'Water is off on Friday', imageUrl: null, publishedAt: '2026-11-27T10:00:00Z', authorName: 'Adam', ...over }
}

/** A chat the bot is in that nobody accepted yet. */
export function chat(over: Partial<Chat> = {}): Chat {
  return {
    id: 1, messenger: 'telegram', kind: 'group', title: 'log', isForum: false, state: 'new', botStatus: 'member', canSend: true,
    addedBy: { externalId: '', userId: null }, leaveAt: null, leftAt: null, firstSeenAt: '2026-11-27T10:00:00Z',
    lastSeenAt: '2026-11-27T10:00:00Z', labelIds: [], topics: [], ...over,
  }
}

export const yard: Chat = chat({
  id: 2, kind: 'supergroup', title: 'Our yard', isForum: true, state: 'accepted', botStatus: 'administrator', labelIds: [1],
  topics: [
    { id: 21, externalId: '5', title: 'Announcements', titleSetByAdmin: false, gone: false, lastSeenAt: '2026-11-27T10:00:00Z', labelIds: [2] },
    { id: 22, externalId: '42', title: '', titleSetByAdmin: false, gone: false, lastSeenAt: '2026-11-27T10:00:00Z', labelIds: [] },
  ],
})

export const labels: Label[] = [
  { id: 1, name: 'residents', description: '', targets: [{ type: 'chat', id: 2 }], users: [], usedByApplications: false },
  { id: 2, name: 'journal', description: 'admins', targets: [{ type: 'chat_topic', id: 21 }], users: [], usedByApplications: true },
]

export const dataItems: DataItem[] = [
  { key: 'contacts', kind: 'table', description: 'Important phones', visibility: 'public', rowCount: 2,
    columns: [{ name: 'name', label: 'Name', type: 'text' }, { name: 'phone', label: 'Phone', type: 'phone' }],
    updatedAt: '2026-11-27T10:00:00Z', updatedBy: { id: 1, name: 'Admin A' } },
  { key: 'house.address', kind: 'value', description: '', visibility: 'admins', valueType: 'text', value: 'Main st 1', rowCount: 0,
    updatedAt: '2026-11-27T10:00:00Z', updatedBy: null },
]

export const contactRows: DataRow[] = [
  { id: 7, position: 0, cells: { name: 'Office', phone: '+380 44 000 00 00' } },
  { id: 8, position: 1, cells: { name: 'Guard' } },
]
