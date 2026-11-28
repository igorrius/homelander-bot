import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useMe } from '../api/public'
import { useMyData, type PublicItem } from '../api/publicData'
import { isOurPageUrl } from '../lib/pageUrl'

/** The frame may run scripts and open links, never share our origin (no allow-same-origin). */
export const PAGE_SANDBOX =
  'allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation'

const MIN_HEIGHT = 100
const MAX_HEIGHT = 20000

function withLang(src: string, lang: string) {
  return `${src}${src.includes('?') ? '&' : '?'}lang=${encodeURIComponent(lang)}`
}

/** What the page learns about the viewer: residents data reaches it only through this message. */
export type FrameData =
  | { type: 'homelander:data'; viewer: { signedIn: false }; items: null }
  | { type: 'homelander:data'; viewer: { signedIn: true; resident: boolean; admin: boolean }; items: Record<string, PublicItem> | null }

/** The homelander:data message for the current viewer; null while it is not known yet. */
function useFrameData(enabled: boolean): FrameData | null {
  const me = useMe()
  const user = enabled ? me.data : undefined
  const mine = useMyData(user ? user.id : undefined)
  return useMemo(() => {
    if (!enabled || me.isPending) return null
    if (!me.data) return { type: 'homelander:data', viewer: { signedIn: false }, items: null }
    if (mine.isPending) return null
    // A failed load still tells the page who is looking, so it shows the public view instead of waiting.
    return {
      type: 'homelander:data',
      viewer: { signedIn: true, resident: me.data.isResident, admin: me.data.isAdmin },
      items: mine.data ?? null,
    }
  }, [enabled, me.isPending, me.data, mine.isPending, mine.data])
}

/**
 * A published (or preview) site page in a sandboxed frame. The page reports its height with
 * { type: 'homelander:height', height }; language changes are posted to it as
 * { type: 'homelander:lang', lang } so the frame does not reload. In a draft preview the server injects a
 * script that posts { type: 'homelander:navigate', path } for links to the draft's pages; onNavigate gets
 * such a path only from this frame and only when it is one of pages.
 *
 * When the page posts { type: 'homelander:ready' }, it gets a homelander:data message (FrameData): who is
 * looking and, for a signed-in user, the items of /api/v1/me/data (residents items for a resident or an
 * admin). It is posted again after a language change and when the data is reloaded. Only our own page URLs
 * get it, and only from this frame.
 */
export function PageFrame({ src, lang, title, pages, onNavigate }: {
  src: string
  lang: string
  title: string
  pages?: string[]
  onNavigate?: (path: string) => void
}) {
  const ref = useRef<HTMLIFrameElement>(null)
  const [height, setHeight] = useState(600)
  // The language goes into the URL only when the frame (re)loads; later switches are posted.
  const [loaded, setLoaded] = useState({ src, lang })
  if (loaded.src !== src) setLoaded({ src, lang })
  const url = withLang(loaded.src, loaded.lang)
  const posted = useRef(lang)
  const nav = useRef({ pages, onNavigate })
  useEffect(() => {
    nav.current = { pages, onNavigate }
  })

  const ours = isOurPageUrl(loaded.src)
  const data = useFrameData(ours)
  const ready = useRef(false)
  const dataRef = useRef(data)
  // The sandboxed page has an opaque origin, so '*' is the only target that reaches it.
  const postData = useCallback(() => {
    const message = dataRef.current
    if (ready.current && message) ref.current?.contentWindow?.postMessage(message, '*')
  }, [])
  // A new document in the frame says ready again.
  useEffect(() => {
    ready.current = false
  }, [url])
  useEffect(() => {
    dataRef.current = data
    postData()
  }, [data, postData])

  useEffect(() => {
    const onMessage = (e: MessageEvent) => {
      const win = ref.current?.contentWindow
      if (!win || e.source !== win) return
      const msg = e.data as { type?: unknown; height?: unknown; path?: unknown } | null
      if (msg?.type === 'homelander:ready') {
        ready.current = ours
        postData()
        return
      }
      if (msg?.type === 'homelander:navigate') {
        const { pages: list, onNavigate: go } = nav.current
        if (typeof msg.path === 'string' && go && list?.includes(msg.path)) go(msg.path)
        return
      }
      if (msg?.type !== 'homelander:height' || typeof msg.height !== 'number' || !Number.isFinite(msg.height)) return
      setHeight(Math.round(Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, msg.height))))
    }
    window.addEventListener('message', onMessage)
    return () => window.removeEventListener('message', onMessage)
  }, [ours, postData])

  useEffect(() => {
    if (posted.current === lang) return
    posted.current = lang
    ref.current?.contentWindow?.postMessage({ type: 'homelander:lang', lang }, '*')
    postData()
  }, [lang, postData])

  return (
    <iframe
      ref={ref}
      src={url}
      title={title}
      sandbox={PAGE_SANDBOX}
      style={{ width: '100%', border: 0, height, display: 'block' }}
    />
  )
}
