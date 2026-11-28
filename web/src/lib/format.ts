/** Date and time in the UI language, e.g. "27.11.2026, 15:20". */
export function formatDateTime(iso: string | undefined | null, lang: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : new Intl.DateTimeFormat(lang, { dateStyle: 'short', timeStyle: 'short' }).format(d)
}

/** "Olena Test" with a fallback for users without a name yet (bootstrap admin). */
export function fullName(u: { firstName: string; lastName: string; phone: string }): string {
  return `${u.firstName} ${u.lastName}`.trim() || u.phone
}

/** A phone with every digit but the last four hidden: "+380500000002" → "+••••••••0002". */
export function maskPhone(phone: string): string {
  const digits = phone.replace(/\D/g, '').length
  let seen = 0
  return phone.replace(/\d/g, (d) => (++seen <= digits - 4 ? '•' : d))
}

/** A byte count as KB or MB. */
export function formatSize(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

/** A data key as a title: "house.address" → "House address". */
export function prettyKey(key: string): string {
  const s = key.replace(/[._]+/g, ' ').trim()
  return s.charAt(0).toUpperCase() + s.slice(1)
}
