import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { ApiError } from '../api/client'
import { qk } from '../api/public'

const isUnauthorized = (err: unknown) => err instanceof ApiError && err.status === 401

/**
 * Marks the visitor as signed out when any request answers 401 (session
 * expired or revoked), so guarded pages redirect to /login.
 */
export function SessionWatcher() {
  const qc = useQueryClient()
  useEffect(() => {
    const signOut = () => {
      if (qc.getQueryData(qk.me)) qc.setQueryData(qk.me, null)
    }
    const unQ = qc.getQueryCache().subscribe((e) => {
      if (e.type === 'updated' && e.action.type === 'error' && isUnauthorized(e.action.error)) signOut()
    })
    const unM = qc.getMutationCache().subscribe((e) => {
      if (e.type === 'updated' && e.action.type === 'error' && isUnauthorized(e.action.error)) signOut()
    })
    return () => {
      unQ()
      unM()
    }
  }, [qc])
  return null
}
