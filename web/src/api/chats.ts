import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { qka } from './admin'
import { api, ApiError } from './client'
import type { Chat, Label, LabelTargetType, NotificationSettings, OutboxStatus } from './types'

const json = (method: string, body?: unknown): RequestInit => ({ method, body: body === undefined ? undefined : JSON.stringify(body) })

/** Query keys of chats, labels and notification settings. */
export const qkc = {
  chats: ['admin', 'chats'] as const,
  labels: ['admin', 'labels'] as const,
  notifications: ['admin', 'notifications'] as const,
  outbox: (id: number) => ['admin', 'outbox', id] as const,
}

/** After any change to chats or labels: both lists and the menu counter may differ. */
function chatsChanged(qc: QueryClient) {
  void qc.invalidateQueries({ queryKey: qkc.chats })
  void qc.invalidateQueries({ queryKey: qkc.labels })
  void qc.invalidateQueries({ queryKey: qka.summary })
}

export function useChats() {
  return useQuery({ queryKey: qkc.chats, queryFn: () => api<Chat[]>('/admin/chats') })
}

export function useLabels() {
  return useQuery({ queryKey: qkc.labels, queryFn: () => api<Label[]>('/admin/labels') })
}

export function useChatAction() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, { id: number; action: 'accept' | 'leave' }>({
    mutationFn: ({ id, action }) => api<null>(`/admin/chats/${id}/${action}`, json('POST')),
    onSuccess: () => chatsChanged(qc),
  })
}

const targetPath = (type: LabelTargetType, id: number) => (type === 'chat' ? `/admin/chats/${id}` : `/admin/topics/${id}`)

export function useSetLabels() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, { type: LabelTargetType; id: number; labelIds: number[] }>({
    mutationFn: ({ type, id, labelIds }) => api<null>(`${targetPath(type, id)}/labels`, json('PUT', { labelIds })),
    onSuccess: () => chatsChanged(qc),
  })
}

export function useRenameTopic() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, { id: number; title: string }>({
    mutationFn: ({ id, title }) => api<null>(`/admin/topics/${id}`, json('PATCH', { title })),
    onSuccess: () => chatsChanged(qc),
  })
}

export interface LabelInput {
  name: string
  description: string
}

export function useCreateLabel() {
  const qc = useQueryClient()
  return useMutation<Label, ApiError, LabelInput>({
    mutationFn: (body) => api<Label>('/admin/labels', json('POST', body)),
    onSuccess: () => chatsChanged(qc),
  })
}

export function useUpdateLabel(id: number) {
  const qc = useQueryClient()
  return useMutation<Label, ApiError, LabelInput>({
    mutationFn: (body) => api<Label>(`/admin/labels/${id}`, json('PATCH', body)),
    onSuccess: () => chatsChanged(qc),
  })
}

export function useDeleteLabel() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, number>({
    mutationFn: (id) => api<null>(`/admin/labels/${id}`, json('DELETE')),
    onSuccess: () => {
      chatsChanged(qc)
      void qc.invalidateQueries({ queryKey: qkc.notifications })
    },
  })
}

export type TestTarget = { type: 'chat' | 'topic' | 'label'; id: number }

const testPath = (t: TestTarget) => `/admin/${t.type === 'chat' ? 'chats' : t.type === 'topic' ? 'topics' : 'labels'}/${t.id}/test`

export function useTestSend() {
  return useMutation<{ outboxIds: number[] }, ApiError, TestTarget>({
    mutationFn: (t) => api<{ outboxIds: number[] }>(testPath(t), json('POST')),
  })
}

/** State of a queued message, polled every second while it is pending. */
export function useOutboxStatus(id: number | null) {
  return useQuery({
    queryKey: qkc.outbox(id ?? 0),
    queryFn: () => api<OutboxStatus>(`/admin/outbox/${id}`),
    enabled: id !== null,
    refetchInterval: (q) => (q.state.data?.state === 'pending' ? 1000 : false),
  })
}

export function useNotifications() {
  return useQuery({ queryKey: qkc.notifications, queryFn: () => api<NotificationSettings>('/admin/settings/notifications') })
}

export function useSaveNotifications() {
  const qc = useQueryClient()
  return useMutation<NotificationSettings, ApiError, NotificationSettings>({
    mutationFn: (body) => api<NotificationSettings>('/admin/settings/notifications', json('PUT', body)),
    onSuccess: (data) => {
      qc.setQueryData(qkc.notifications, data)
      void qc.invalidateQueries({ queryKey: qkc.labels })
    },
  })
}
