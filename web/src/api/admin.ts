import { keepPreviousData, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import type { AdminUser, Page, UserCard, UserStatus } from './types'

/** Query keys of the admin API; all start with 'admin' so logout can drop them together. */
export const qka = {
  all: ['admin'] as const,
  summary: ['admin', 'summary'] as const,
  users: (q?: UserQuery) => (q ? (['admin', 'users', q] as const) : (['admin', 'users'] as const)),
  user: (id: number) => ['admin', 'user', id] as const,
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

export { json }
