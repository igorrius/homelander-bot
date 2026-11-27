import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import type { DataColumn, DataItem, DataKind, DataRow, DataType } from './types'

const json = (method: string, body?: unknown): RequestInit => ({ method, body: body === undefined ? undefined : JSON.stringify(body) })

/** Query keys of the data space. */
export const qkd = {
  list: ['admin', 'data'] as const,
  item: (key: string) => ['admin', 'data', key] as const,
}

const changed = (qc: QueryClient) => qc.invalidateQueries({ queryKey: qkd.list })

export interface DataView {
  item: DataItem
  rows: DataRow[]
}

export interface DataInput {
  key?: string
  kind?: DataKind
  description?: string
  public?: boolean
  valueType?: DataType
  value?: unknown
  columns?: DataColumn[]
}

export function useDataItems() {
  return useQuery({ queryKey: qkd.list, queryFn: () => api<DataItem[]>('/admin/data') })
}

export function useDataItem(key: string) {
  return useQuery({ queryKey: qkd.item(key), queryFn: () => api<DataView>(`/admin/data/${key}`) })
}

export function useCreateDataItem() {
  const qc = useQueryClient()
  return useMutation<DataItem, ApiError, DataInput>({
    mutationFn: (body) => api<DataItem>('/admin/data', json('POST', body)),
    onSuccess: () => void changed(qc),
  })
}

export function usePatchDataItem(key: string) {
  const qc = useQueryClient()
  return useMutation<DataItem, ApiError, DataInput>({
    mutationFn: (body) => api<DataItem>(`/admin/data/${key}`, json('PATCH', body)),
    onSuccess: (item) => {
      qc.setQueryData<DataView>(qkd.item(key), (v) => (v ? { ...v, item } : v))
      void changed(qc)
    },
  })
}

export function useDeleteDataItem() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, string>({
    mutationFn: (key) => api<null>(`/admin/data/${key}`, json('DELETE')),
    onSuccess: (_, key) => {
      qc.removeQueries({ queryKey: qkd.item(key) })
      void changed(qc)
    },
  })
}

export function usePutDataRows(key: string) {
  const qc = useQueryClient()
  return useMutation<{ rows: DataRow[] }, ApiError, DataRow[]>({
    mutationFn: (rows) => api<{ rows: DataRow[] }>(`/admin/data/${key}/rows`, json('PUT', { rows })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: qkd.item(key) })
      void changed(qc)
    },
  })
}

export function useDeleteDataRow(key: string) {
  const qc = useQueryClient()
  return useMutation<null, ApiError, number>({
    mutationFn: (id) => api<null>(`/admin/data/${key}/rows/${id}`, json('DELETE')),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: qkd.item(key) })
      void changed(qc)
    },
  })
}

/** Uploads an image for the data space and returns its asset id. */
export function useUploadDataImage() {
  return useMutation<{ id: number; url: string }, ApiError, File>({
    mutationFn: (file) => {
      const body = new FormData()
      body.append('file', file)
      return api<{ id: number; url: string }>('/admin/data/assets', { method: 'POST', body })
    },
  })
}

/** Where an admin sees an uploaded data image. */
export const adminAssetUrl = (id: number) => `/api/v1/admin/assets/${id}`

/** Where anyone reads a public item. */
export const publicDataUrl = (key: string) => `/api/v1/public/data/${key}`
