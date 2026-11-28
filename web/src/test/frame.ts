/** The page a PageFrame shows: its src without the per-load nonce, which must be there. */
export function framedPage(frame: HTMLIFrameElement): string {
  const src = frame.getAttribute('src') ?? ''
  const m = /^(.*)&hl=[0-9a-f]{32}$/.exec(src)
  if (!m) throw new Error(`frame src without a nonce: ${src}`)
  return m[1]
}
