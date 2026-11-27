import { keepPreviousData, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import type { AdminUser, House, Me, Page, ResidencyStatus, UserCard, UserStatus } from './types'

/** Query keys of the admin API; all start with 'admin' so logout can drop them together. */
export const qka = {
  all: ['admin'] as const,
  summary: ['admin', 'summary'] as const,
  users: (q?: UserQuery) => (q ? (['admin', 'users', q] as const) : (['admin', 'users'] as const)),
  user: (id: number) => ['admin', 'user', id] as const,
  houses: (archived = false) => ['admin', 'houses', archived] as const,
  statuses: ['admin', 'statuses'] as const,
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

export { json }
