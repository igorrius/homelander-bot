import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import type { Me, RegisterOptions, Site } from './types'

/** Query keys of the public API. Admin keys all start with 'admin' (see api/admin.ts). */
export const qk = { site: ['site'], me: ['me'], registerOptions: ['registerOptions'] } as const

export function useSite() {
  return useQuery({ queryKey: qk.site, queryFn: () => api<Site>('/site'), staleTime: 60_000 })
}

/** The signed-in user, or null for a guest. */
export function useMe() {
  return useQuery({
    queryKey: qk.me,
    queryFn: async () => {
      try {
        return await api<Me>('/me')
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return null
        throw err
      }
    },
  })
}

export function useRegisterOptions() {
  return useQuery({ queryKey: qk.registerOptions, queryFn: () => api<RegisterOptions>('/register/options') })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation<void, ApiError, void>({
    mutationFn: () => api<void>('/auth/logout', { method: 'POST' }),
    onSuccess: () => {
      qc.setQueryData(qk.me, null)
      qc.removeQueries({ queryKey: ['admin'] })
    },
  })
}

export function useRequestCode() {
  return useMutation<void, ApiError, { phone: string }>({
    mutationFn: (body) => api<void>('/auth/code', { method: 'POST', body: JSON.stringify(body) }),
  })
}

export function useVerifyCode() {
  const qc = useQueryClient()
  return useMutation<{ user: Me }, ApiError, { phone: string; code: string }>({
    mutationFn: (body) => api<{ user: Me }>('/auth/verify', { method: 'POST', body: JSON.stringify(body) }),
    onSuccess: ({ user }) => qc.setQueryData(qk.me, user),
  })
}
