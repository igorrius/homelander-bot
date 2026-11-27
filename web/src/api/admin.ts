import { keepPreviousData, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import { qk } from './public'
import type { AdminUser, AuditEntry, House, HouseStats, Locale, LocalizedText, Me, Page, Residency, ResidencyStatus, Site, UserCard, UserStatus } from './types'

/** Query keys of the admin API; all start with 'admin' so logout can drop them together. */
export const qka = {
  all: ['admin'] as const,
  summary: ['admin', 'summary'] as const,
  users: (q?: UserQuery) => (q ? (['admin', 'users', q] as const) : (['admin', 'users'] as const)),
  user: (id: number) => ['admin', 'user', id] as const,
  houses: (archived = false) => ['admin', 'houses', archived] as const,
  statuses: ['admin', 'statuses'] as const,
  house: (id: number) => ['admin', 'house', id] as const,
  apartment: (houseId: number, n: number | null) => ['admin', 'house', houseId, 'apartment', n] as const,
  site: ['admin', 'site'] as const,
  audit: (q: AuditQuery) => ['admin', 'audit', q] as const,
}

export interface UserQuery {
  q?: string
  houseId?: number
  status?: UserStatus
  isAdmin?: boolean
  isResident?: boolean
  page?: number
  pageSize?: number
}

/** Query string of the defined values of q, e.g. "?status=active&page=2". */
export function queryString(q: object): string {
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(q)) {
    if (v !== undefined && v !== null && v !== '') p.set(k, String(v))
  }
  const s = p.toString()
  return s ? `?${s}` : ''
}

const json = (method: string, body?: unknown): RequestInit => ({ method, body: body === undefined ? undefined : JSON.stringify(body) })

export function useSummary() {
  return useQuery({ queryKey: qka.summary, queryFn: () => api<{ pendingApplications: number }>('/admin/summary') })
}

export function useUsers(q: UserQuery) {
  return useQuery({
    queryKey: qka.users(q),
    queryFn: () => api<Page<AdminUser>>(`/admin/users${queryString(q)}`),
    placeholderData: keepPreviousData,
  })
}

/** After a change to a user: store the returned card, refresh lists and the counter. */
export function cardChanged(qc: QueryClient, card: UserCard) {
  qc.setQueryData(qka.user(card.user.id), card)
  void qc.invalidateQueries({ queryKey: qka.users() })
  void qc.invalidateQueries({ queryKey: qka.summary })
}

export type UserAction = 'approve' | 'reject' | 'disable' | 'enable'

export function useUserAction() {
  const qc = useQueryClient()
  return useMutation<UserCard, ApiError, { id: number; action: UserAction }>({
    mutationFn: ({ id, action }) => api<UserCard>(`/admin/users/${id}/${action}`, json('POST')),
    onSuccess: (card) => cardChanged(qc, card),
  })
}

export interface NewUser {
  phone: string
  firstName: string
  lastName: string
  isResident: boolean
  isAdmin: boolean
}

export function useAddUser() {
  const qc = useQueryClient()
  return useMutation<UserCard, ApiError, NewUser>({
    mutationFn: (body) => api<UserCard>('/admin/users', json('POST', body)),
    onSuccess: (card) => cardChanged(qc, card),
  })
}

export function useHouses(includeArchived = false) {
  return useQuery({
    queryKey: qka.houses(includeArchived),
    queryFn: () => api<House[]>(`/admin/houses${includeArchived ? '?archived=true' : ''}`),
  })
}

export function useUserCard(id: number) {
  return useQuery({ queryKey: qka.user(id), queryFn: () => api<UserCard>(`/admin/users/${id}`) })
}

export type UserPatch = Partial<Pick<Me, 'firstName' | 'lastName' | 'phone' | 'locale' | 'isResident' | 'isAdmin'>>

export function usePatchUser(id: number) {
  const qc = useQueryClient()
  return useMutation<UserCard, ApiError, UserPatch>({
    mutationFn: (body) => api<UserCard>(`/admin/users/${id}`, json('PATCH', body)),
    onSuccess: (card) => cardChanged(qc, card),
  })
}

export function useUnlinkChannel(userId: number) {
  const qc = useQueryClient()
  return useMutation<UserCard, ApiError, number>({
    mutationFn: (channelId) => api<UserCard>(`/admin/users/${userId}/channels/${channelId}`, json('DELETE')),
    onSuccess: (card) => cardChanged(qc, card),
  })
}

export interface ResidencyInput {
  houseId: number
  apartment: number
  statusId: number
}

export function useAddResidency(userId: number) {
  const qc = useQueryClient()
  return useMutation<UserCard, ApiError, ResidencyInput>({
    mutationFn: (body) => api<UserCard>(`/admin/users/${userId}/residencies`, json('POST', body)),
    onSuccess: (card) => cardChanged(qc, card),
  })
}

export function useUpdateResidency() {
  const qc = useQueryClient()
  return useMutation<UserCard, ApiError, ResidencyInput & { id: number }>({
    mutationFn: ({ id, ...body }) => api<UserCard>(`/admin/residencies/${id}`, json('PATCH', body)),
    onSuccess: (card) => cardChanged(qc, card),
  })
}

export function useDeleteResidency() {
  const qc = useQueryClient()
  return useMutation<UserCard, ApiError, number>({
    mutationFn: (id) => api<UserCard>(`/admin/residencies/${id}`, json('DELETE')),
    onSuccess: (card) => cardChanged(qc, card),
  })
}

export function useStatuses() {
  return useQuery({ queryKey: qka.statuses, queryFn: () => api<ResidencyStatus[]>('/admin/statuses') })
}

export type HouseInput = Omit<House, 'id'>

/** After a house change: refresh house lists, details and the public register options. */
function housesChanged(qc: QueryClient) {
  void qc.invalidateQueries({ queryKey: ['admin', 'houses'] })
  void qc.invalidateQueries({ queryKey: ['admin', 'house'] })
  void qc.invalidateQueries({ queryKey: qk.registerOptions })
}

export function useCreateHouse() {
  const qc = useQueryClient()
  return useMutation<House, ApiError, HouseInput>({
    mutationFn: (body) => api<House>('/admin/houses', json('POST', body)),
    onSuccess: () => housesChanged(qc),
  })
}

export function useUpdateHouse(id: number) {
  const qc = useQueryClient()
  return useMutation<House, ApiError, HouseInput>({
    mutationFn: (body) => api<House>(`/admin/houses/${id}`, json('PATCH', body)),
    onSuccess: () => housesChanged(qc),
  })
}

export function useHouse(id: number) {
  return useQuery({ queryKey: qka.house(id), queryFn: () => api<{ house: House; stats: HouseStats }>(`/admin/houses/${id}`) })
}

export function useApartment(houseId: number, n: number | null) {
  return useQuery({
    queryKey: qka.apartment(houseId, n),
    queryFn: () => api<{ user: AdminUser; residency: Residency }[]>(`/admin/houses/${houseId}/apartments/${n}`),
    enabled: n !== null,
  })
}

export type StatusInput = Omit<ResidencyStatus, 'id'>

function statusesChanged(qc: QueryClient) {
  void qc.invalidateQueries({ queryKey: qka.statuses })
  void qc.invalidateQueries({ queryKey: qk.registerOptions })
}

export function useCreateStatus() {
  const qc = useQueryClient()
  return useMutation<ResidencyStatus, ApiError, StatusInput>({
    mutationFn: (body) => api<ResidencyStatus>('/admin/statuses', json('POST', body)),
    onSuccess: () => statusesChanged(qc),
  })
}

export function useUpdateStatus(id: number) {
  const qc = useQueryClient()
  return useMutation<ResidencyStatus, ApiError, StatusInput>({
    mutationFn: (body) => api<ResidencyStatus>(`/admin/statuses/${id}`, json('PATCH', body)),
    onSuccess: () => statusesChanged(qc),
  })
}

export interface SiteInput {
  communityName: LocalizedText
  welcomeText: LocalizedText
  defaultLocale: Locale
}

export function useAdminSite() {
  return useQuery({ queryKey: qka.site, queryFn: () => api<Site>('/admin/site') })
}

/** Every site mutation answers with the settings: store them, refresh the public view. */
function useSiteMutation<V>(fn: (v: V) => Promise<Site>) {
  const qc = useQueryClient()
  return useMutation<Site, ApiError, V>({
    mutationFn: fn,
    onSuccess: (site) => {
      qc.setQueryData(qka.site, site)
      void qc.invalidateQueries({ queryKey: qk.site })
    },
  })
}

export function useSaveSite() {
  return useSiteMutation((body: SiteInput) => api<Site>('/admin/site', json('PUT', body)))
}

export function useUploadPicture() {
  return useSiteMutation((file: File) => {
    const form = new FormData()
    form.append('picture', file)
    return api<Site>('/admin/site/picture', { method: 'PUT', body: form })
  })
}

export function useDeletePicture() {
  return useSiteMutation(() => api<Site>('/admin/site/picture', json('DELETE')))
}

export interface AuditQuery {
  userId?: number
  page: number
}

export function useAudit(q: AuditQuery) {
  return useQuery({
    queryKey: qka.audit(q),
    queryFn: () => api<Page<AuditEntry>>(`/admin/audit${queryString({ ...q, pageSize: 25 })}`),
    placeholderData: keepPreviousData,
  })
}

export { json }
