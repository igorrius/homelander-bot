import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import { qk } from './public'
import type { PageVersion } from './types'

/** Query keys of the home page versions. */
export const qkp = {
  list: ['admin', 'pages'] as const,
  item: (id: number) => ['admin', 'pages', id] as const,
  preview: (id: number) => ['admin', 'pages', id, 'preview'] as const,
}

// Any change may touch several versions' statuses and what the home page shows.
const changed = (qc: QueryClient) => {
  void qc.invalidateQueries({ queryKey: qkp.list })
  void qc.invalidateQueries({ queryKey: qk.site })
}

export function usePageVersions() {
  return useQuery({ queryKey: qkp.list, queryFn: () => api<PageVersion[]>('/admin/pages') })
}

export function usePageVersion(id: number) {
  return useQuery({ queryKey: qkp.item(id), queryFn: () => api<PageVersion>(`/admin/pages/${id}`) })
}

/** A 30-minute preview link; refreshed well before it runs out. */
export function usePagePreview(id: number) {
  return useQuery({
    queryKey: qkp.preview(id),
    queryFn: () => api<{ url: string }>(`/admin/pages/${id}/preview`, { method: 'POST' }),
    staleTime: 20 * 60_000,
    refetchInterval: 20 * 60_000,
    refetchOnWindowFocus: false,
  })
}

export function useUploadPage() {
  const qc = useQueryClient()
  return useMutation<PageVersion, ApiError, { file: File; note?: string }>({
    mutationFn: ({ file, note }) => {
      const body = new FormData()
      body.append('zip', file)
      if (note) body.append('note', note)
      return api<PageVersion>('/admin/pages', { method: 'POST', body })
    },
    onSuccess: () => changed(qc),
  })
}

export function useCopyPage() {
  const qc = useQueryClient()
  return useMutation<PageVersion, ApiError, number>({
    mutationFn: (id) => api<PageVersion>(`/admin/pages/${id}/copy`, { method: 'POST' }),
    onSuccess: () => changed(qc),
  })
}

export function usePublishPage() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, number>({
    mutationFn: (id) => api<null>(`/admin/pages/${id}/publish`, { method: 'POST' }),
    onSuccess: () => changed(qc),
  })
}

export function useUnpublishPage() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, void>({
    mutationFn: () => api<null>('/admin/pages/unpublish', { method: 'POST' }),
    onSuccess: () => changed(qc),
  })
}

export function useDeletePage() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, number>({
    mutationFn: (id) => api<null>(`/admin/pages/${id}`, { method: 'DELETE' }),
    onSuccess: (_, id) => {
      qc.removeQueries({ queryKey: qkp.item(id) })
      changed(qc)
    },
  })
}
