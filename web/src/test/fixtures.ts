import type { AdminUser, Chat, DataItem, DataRow, House, Label, Me, RegisterOptions, ResidencyStatus, Site, UserCard } from '../api/types'

// Synthetic data only: no real names, phones or addresses.
export const site: Site = {
  communityName: { uk: 'Тестова громада', en: 'Test Community' },
  welcomeText: { uk: 'Привіт' },
  defaultLocale: 'uk',
  pictureUrl: null,
  botUrl: 'https://t.me/test_bot',
  registerUrl: 'http://localhost/register',
  pageUrl: null,
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
  houses: houses.map(({ id, name, apartmentFrom, apartmentTo }) => ({ id, name, apartmentFrom, apartmentTo })),
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
    audit: [{ id: 1, at: '2026-11-21T09:00:00Z', action: 'user.approved', target: 'user:5', targetName: 'Olena Test', targetNames: null, actorUserId: 1, actorName: 'Adam Admin', details: null, ip: '127.0.0.1' }],
  }
}

/** Routes every page needs: site, register options and a guest /me. */
export const publicRoutes = {
  'GET /site': { body: site },
  'GET /register/options': { body: registerOptions },
  'GET /me': { status: 401, body: { error: { code: 'unauthorized' } } },
}

/** Routes for an admin session with an empty queue. */
export const adminRoutes = {
  ...publicRoutes,
  'GET /me': { body: admin },
  'GET /admin/summary': { body: { pendingApplications: 0, awaitingActivation: 0, activeUsers: 1, disabledUsers: 0, rejectedUsers: 0 } },
  'GET /admin/users': { body: { items: [], total: 0 } },
  'GET /admin/houses': { body: houses },
  'GET /admin/statuses': { body: statuses },
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
  { id: 1, name: 'residents', description: '', targets: [{ type: 'chat', id: 2 }], usedByApplications: false },
  { id: 2, name: 'journal', description: 'admins', targets: [{ type: 'chat_topic', id: 21 }], usedByApplications: true },
]

export const dataItems: DataItem[] = [
  { key: 'contacts', kind: 'table', description: 'Important phones', public: true, rowCount: 2,
    columns: [{ name: 'name', label: 'Name', type: 'text' }, { name: 'phone', label: 'Phone', type: 'phone' }],
    updatedAt: '2026-11-27T10:00:00Z', updatedBy: { id: 1, name: 'Admin A' } },
  { key: 'house.address', kind: 'value', description: '', public: false, valueType: 'text', value: 'Main st 1', rowCount: 0,
    updatedAt: '2026-11-27T10:00:00Z', updatedBy: null },
]

export const contactRows: DataRow[] = [
  { id: 7, position: 0, cells: { name: 'Office', phone: '+380 44 000 00 00' } },
  { id: 8, position: 1, cells: { name: 'Guard' } },
]
