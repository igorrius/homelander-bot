import { describe, expect, it } from 'vitest'
import en from './en.json'
import uk from './uk.json'

function keys(obj: Record<string, unknown>, prefix = ''): string[] {
  return Object.entries(obj).flatMap(([k, v]) => {
    const key = prefix ? `${prefix}.${k}` : k
    return v !== null && typeof v === 'object' ? keys(v as Record<string, unknown>, key) : [key]
  })
}

describe('locale files', () => {
  it('uk and en have the same keys', () => {
    const ukKeys = new Set(keys(uk))
    const enKeys = new Set(keys(en))
    const missingInEn = [...ukKeys].filter((k) => !enKeys.has(k))
    const missingInUk = [...enKeys].filter((k) => !ukKeys.has(k))
    expect({ missingInEn, missingInUk }).toEqual({ missingInEn: [], missingInUk: [] })
  })
})
