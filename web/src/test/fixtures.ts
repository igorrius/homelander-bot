import type { AdminUser, House, Me, RegisterOptions, ResidencyStatus, Site, UserCard } from '../api/types'

// Synthetic data only: no real names, phones or addresses.
export const site: Site = {
  communityName: { uk: 'Тестова громада', en: 'Test Community' },
  welcomeText: { uk: 'Привіт' },
  defaultLocale: 'uk',
  pictureUrl: null,
  botUrl: 'https://t.me/test_bot',
  registerUrl: 'http://localhost/register',
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
    audit: [{ id: 1, at: '2026-11-21T09:00:00Z', action: 'user.approved', target: 'user:5', actorUserId: 1, details: null, ip: '127.0.0.1' }],
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
  'GET /admin/summary': { body: { pendingApplications: 0 } },
  'GET /admin/users': { body: { items: [], total: 0 } },
}
