/** A piece of text; href is set when the piece is a link. */
export interface TextPart {
  text: string
  href?: string
}

const urlPattern = /https?:\/\/[^\s<>"]+/gi
const trailing = new Set(['.', ',', ';', ':', '!', '?', "'", '"', '»', '…', ')', ']'])

const count = (s: string, ch: string) => s.split(ch).length - 1

/** Drops punctuation that ends a sentence rather than the URL; keeps a ")" the URL opened. */
function trimUrl(url: string): string {
  let u = url
  while (u.length > 0 && trailing.has(u[u.length - 1])) {
    const last = u[u.length - 1]
    if (last === ')' && count(u, '(') >= count(u, ')')) break
    if (last === ']' && count(u, '[') >= count(u, ']')) break
    u = u.slice(0, -1)
  }
  return u
}

/** Splits plain text into text and http(s) link parts, in order. */
export function splitLinks(text: string): TextPart[] {
  const parts: TextPart[] = []
  let at = 0
  for (const m of text.matchAll(urlPattern)) {
    const url = trimUrl(m[0])
    const start = m.index
    if (url.length <= 'https://'.length) continue
    if (start > at) parts.push({ text: text.slice(at, start) })
    parts.push({ text: url, href: url })
    at = start + url.length
  }
  if (at < text.length) parts.push({ text: text.slice(at) })
  return parts
}
