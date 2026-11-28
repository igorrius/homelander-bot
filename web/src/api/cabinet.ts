import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import { qk } from './public'
import type { Me, MyHome } from './types'

/** The signed-in user's apartments and linked messenger. */
export function useMyHome() {
  return useQuery({ queryKey: ['cabinet', 'home'], queryFn: () => api<MyHome>('/me/home') })
}

/** Spends a login link from the bot: opens a session and says which cabinet page to open. */
export function useLoginLink() {
  const qc = useQueryClient()
  return useMutation<{ user: Me; next: string }, ApiError, { token: string }>({
    mutationFn: (body) => api<{ user: Me; next: string }>('/auth/link', { method: 'POST', body: JSON.stringify(body) }),
    onSuccess: ({ user }) => {
      // Another account may have been signed in before: drop what it loaded.
      qc.removeQueries({ queryKey: ['admin'] })
      qc.removeQueries({ queryKey: ['cabinet'] })
      qc.setQueryData(qk.me, user)
    },
  })
}
