import { useEffect, useRef, useState } from 'react'

/** The frame may run scripts and open links, never share our origin (no allow-same-origin). */
export const PAGE_SANDBOX =
  'allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation'

const MIN_HEIGHT = 100
const MAX_HEIGHT = 20000

function withLang(src: string, lang: string) {
  return `${src}${src.includes('?') ? '&' : '?'}lang=${encodeURIComponent(lang)}`
}

/**
 * A published (or preview) home page in a sandboxed frame. The page reports its height with
 * { type: 'homelander:height', height }; language changes are posted to it as
 * { type: 'homelander:lang', lang } so the frame does not reload. In a draft preview the server injects a
 * script that posts { type: 'homelander:navigate', path } for links to the draft's pages; onNavigate gets
 * such a path only from this frame and only when it is one of pages.
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

  useEffect(() => {
    const onMessage = (e: MessageEvent) => {
      const win = ref.current?.contentWindow
      if (!win || e.source !== win) return
      const data = e.data as { type?: unknown; height?: unknown; path?: unknown } | null
      if (data?.type === 'homelander:navigate') {
        const { pages: list, onNavigate: go } = nav.current
        if (typeof data.path === 'string' && go && list?.includes(data.path)) go(data.path)
        return
      }
      if (data?.type !== 'homelander:height' || typeof data.height !== 'number' || !Number.isFinite(data.height)) return
      setHeight(Math.round(Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, data.height))))
    }
    window.addEventListener('message', onMessage)
    return () => window.removeEventListener('message', onMessage)
  }, [])

  useEffect(() => {
    if (posted.current === lang) return
    posted.current = lang
    // The sandboxed page has an opaque origin, so '*' is the only target that reaches it.
    ref.current?.contentWindow?.postMessage({ type: 'homelander:lang', lang }, '*')
  }, [lang])

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
