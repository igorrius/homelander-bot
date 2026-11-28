import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useMe } from '../api/public'
import { useMyData, type PublicItem } from '../api/publicData'
import { isOurPageUrl } from '../lib/pageUrl'

/** The frame may run scripts and open links, never share our origin (no allow-same-origin). */
export const PAGE_SANDBOX =
  'allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation'

const MIN_HEIGHT = 100
const MAX_HEIGHT = 20000

/** The frame URL: the page, its first language and, for our pages, the nonce of this load. */
function frameUrl(src: string, lang: string, nonce: string | null) {
  const url = `${src}${src.includes('?') ? '&' : '?'}lang=${encodeURIComponent(lang)}`
  return nonce ? `${url}&hl=${nonce}` : url
}

function newNonce(): string {
  const b = new Uint8Array(16)
  crypto.getRandomValues(b)
  return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')
}

/** The end of the MessageChannel the page's bridge script hands over with homelander:ready. */
type Port = Pick<MessagePort, 'postMessage' | 'close'>

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
 * Viewer data (FrameData: who is looking and, when signed in, the items of /api/v1/me/data) goes only to
 * our own page URLs. Each frame load gets a random nonce in its URL (hl=…). The server injects a bridge
 * script into every page that posts { type: 'homelander:ready', nonce } with a MessagePort; the data is
 * posted only through the port of a ready that came from this frame with the current nonce, and again on
 * a language change or a re-fetch. A port belongs to the document that made it, so a frame that navigated
 * elsewhere cannot receive data; besides, every load event of the frame drops the port, and only a ready
 * that comes after the latest load is taken.
 * When the viewer changes (logout, login, another user) the frame is reloaded with a new nonce.
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
  // A different viewer (logout, login, another user) gets a freshly loaded page: a page may keep what an
  // earlier homelander:data gave it, so the old viewer's residents data must not stay on screen.
  const me = useMe()
  const viewer = me.isPending ? undefined : me.data ? `user:${me.data.id}` : 'guest'
  const [loaded, setLoaded] = useState(() => ({ src, lang, nonce: newNonce(), viewer }))
  if (loaded.src !== src) setLoaded({ src, lang, nonce: newNonce(), viewer })
  else if (viewer !== undefined && loaded.viewer !== viewer) {
    // Learning who is looking for the first time is not a change.
    setLoaded(loaded.viewer === undefined ? { ...loaded, viewer } : { src, lang, nonce: newNonce(), viewer })
  }
  const ours = isOurPageUrl(loaded.src)
  const nonce = ours ? loaded.nonce : null
  const url = frameUrl(loaded.src, loaded.lang, nonce)
  const posted = useRef(lang)
  const nav = useRef({ pages, onNavigate })
  useEffect(() => {
    nav.current = { pages, onNavigate }
  })

  const data = useFrameData(ours)
  const dataRef = useRef(data)
  // awaiting: the frame has loaded a document and no ready was taken since; only then is a ready taken.
  const link = useRef<{ port: Port | null; awaiting: boolean }>({ port: null, awaiting: false })
  const postData = useCallback(() => {
    const message = dataRef.current
    if (message) link.current.port?.postMessage(message)
  }, [])
  const dropPort = useCallback(() => {
    link.current.port?.close()
    link.current.port = null
  }, [])
  // A new frame URL is a new load: nothing is ready until its page says so.
  useEffect(() => {
    link.current = { port: null, awaiting: false }
    return dropPort
  }, [url, dropPort])
  useEffect(() => {
    dataRef.current = data
    postData()
  }, [data, postData])

  useEffect(() => {
    const onMessage = (e: MessageEvent) => {
      const win = ref.current?.contentWindow
      if (!win || e.source !== win) return
      const msg = e.data as { type?: unknown; height?: unknown; path?: unknown; nonce?: unknown } | null
      if (msg?.type === 'homelander:ready') {
        const port = e.ports?.[0] as Port | undefined
        if (!nonce || msg.nonce !== nonce || !port || !link.current.awaiting) return
        link.current.port = port
        link.current.awaiting = false
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
  }, [nonce, postData, dropPort])

  // Every load is a new document in the frame (ours, a reload, or wherever a link took it): the old port
  // goes, and only a ready that comes after this load is taken. The bridge posts ready a task after its
  // own load event, so it always comes after this one.
  const onLoad = () => {
    dropPort()
    link.current.awaiting = true
  }

  useEffect(() => {
    if (posted.current === lang) return
    posted.current = lang
    // The sandboxed page has an opaque origin, so '*' is the only target that reaches it; no data here.
    ref.current?.contentWindow?.postMessage({ type: 'homelander:lang', lang }, '*')
    postData()
  }, [lang, postData])

  return (
    <iframe
      ref={ref}
      src={url}
      title={title}
      sandbox={PAGE_SANDBOX}
      onLoad={onLoad}
      style={{ width: '100%', border: 0, height, display: 'block' }}
    />
  )
}
