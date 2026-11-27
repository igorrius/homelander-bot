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
