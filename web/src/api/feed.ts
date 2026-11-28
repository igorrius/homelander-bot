import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, ApiError } from './client'
import type { AdminPost, CabinetPost, FeedSettings, Page } from './types'

const json = (method: string, body?: unknown): RequestInit => ({ method, body: body === undefined ? undefined : JSON.stringify(body) })

/** Query keys of the feed: admin ones under 'admin', the resident's under 'cabinet' (logout drops both). */
export const qkf = {
  admin: ['admin', 'feed'] as const,
  settings: ['admin', 'feed', 'settings'] as const,
  posts: (page: number, pageSize: number) => ['admin', 'feed', 'posts', page, pageSize] as const,
  post: (id: number) => ['admin', 'feed', 'post', id] as const,
  cabinet: ['cabinet', 'feed'] as const,
  cabinetList: ['cabinet', 'feed', 'list'] as const,
  cabinetPost: (id: number) => ['cabinet', 'feed', 'post', id] as const,
}

/** A 404 (missing post or the module turned off) will not change on a retry. */
const retryUnless404 = (count: number, err: unknown) => !(err instanceof ApiError && err.status === 404) && count < 1

/** What an admin writes; labelIds only for a draft (a published post keeps its labels). */
export interface PostInput {
  body: string
  imageAssetId: number | null
  labelIds?: number[]
}

/** After any change to a post: lists and the cabinet show it differently. */
function postsChanged(qc: QueryClient, post?: AdminPost) {
  if (post) qc.setQueryData(qkf.post(post.id), post)
  void qc.invalidateQueries({ queryKey: ['admin', 'feed', 'posts'] })
  void qc.invalidateQueries({ queryKey: qkf.cabinet })
}

export function useFeedSettings() {
  return useQuery({ queryKey: qkf.settings, queryFn: () => api<FeedSettings>('/admin/feed/settings') })
}

export function useSaveFeedSettings() {
  const qc = useQueryClient()
  return useMutation<FeedSettings, ApiError, FeedSettings>({
    mutationFn: (body) => api<FeedSettings>('/admin/feed/settings', json('PUT', body)),
    onSuccess: (data) => qc.setQueryData(qkf.settings, data),
  })
}

/** One page of posts, newest first. */
export function useFeedPosts(page: number, pageSize: number) {
  return useQuery({
    queryKey: qkf.posts(page, pageSize),
    queryFn: () => api<Page<AdminPost>>(`/admin/feed/posts?page=${page}&pageSize=${pageSize}`),
    placeholderData: keepPreviousData,
  })
}

/** One post with its deliveries, polled every 3 s while a Telegram copy is still pending. */
export function useFeedPost(id: number | null) {
  return useQuery({
    queryKey: qkf.post(id ?? 0),
    queryFn: () => api<AdminPost>(`/admin/feed/posts/${id}`),
    enabled: id !== null,
    refetchInterval: (q) => (q.state.data?.deliveries?.some((d) => d.state === 'pending') ? 3000 : false),
  })
}

export function useCreatePost() {
  const qc = useQueryClient()
  return useMutation<AdminPost, ApiError, PostInput>({
    mutationFn: (body) => api<AdminPost>('/admin/feed/posts', json('POST', body)),
    onSuccess: (post) => postsChanged(qc, post),
  })
}

export function useUpdatePost() {
  const qc = useQueryClient()
  return useMutation<AdminPost, ApiError, { id: number; input: PostInput }>({
    mutationFn: ({ id, input }) => api<AdminPost>(`/admin/feed/posts/${id}`, json('PATCH', input)),
    onSuccess: (post) => postsChanged(qc, post),
  })
}

export function usePublishPost() {
  const qc = useQueryClient()
  return useMutation<AdminPost, ApiError, { id: number; labelIds: number[] }>({
    mutationFn: ({ id, labelIds }) => api<AdminPost>(`/admin/feed/posts/${id}/publish`, json('POST', { labelIds })),
    onSuccess: (post) => {
      postsChanged(qc, post)
      // The answer to publish may not carry the new deliveries yet: load them.
      void qc.invalidateQueries({ queryKey: qkf.post(post.id) })
    },
  })
}

export function useDeletePost() {
  const qc = useQueryClient()
  return useMutation<null, ApiError, number>({
    mutationFn: (id) => api<null>(`/admin/feed/posts/${id}`, json('DELETE')),
    onSuccess: (_, id) => {
      qc.removeQueries({ queryKey: qkf.post(id) })
      postsChanged(qc)
    },
  })
}

/** The cabinet feed, 20 posts a page; the next page comes by the cursor of the last one. */
export function useCabinetFeed() {
  return useInfiniteQuery({
    queryKey: qkf.cabinetList,
    queryFn: ({ pageParam }) => api<{ items: CabinetPost[]; nextCursor: string | null }>(`/feed/posts${pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : ''}`),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor || undefined,
    retry: retryUnless404,
  })
}

/** One published post in the cabinet. */
export function useCabinetPost(id: number) {
  return useQuery({
    queryKey: qkf.cabinetPost(id),
    queryFn: () => api<CabinetPost>(`/feed/posts/${id}`),
    retry: retryUnless404,
  })
}
