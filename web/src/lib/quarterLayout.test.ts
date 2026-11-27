import { describe, expect, it } from 'vitest'
import { quarterLayout } from './quarterLayout'

describe('quarterLayout', () => {
  it.each([
    [0, 2, 1],
    [1, 2, 1],
    [2, 2, 1],
    [12, 5, 3],
    [24, 7, 4],
  ])('%i houses → %i × %i', (n, cols, rows) => {
    const l = quarterLayout(n)
    expect([l.cols, l.rows]).toEqual([cols, rows])
    expect(l.cells).toHaveLength(cols * rows)
    const placed = l.cells.filter((c) => c.house !== null).map((c) => c.house)
    expect(placed.sort((a, b) => a! - b!)).toEqual([...Array(n).keys()])
    // Every cell is a distinct grid position.
    expect(new Set(l.cells.map((c) => `${c.col},${c.row}`)).size).toBe(cols * rows)
  })

  it('treats a negative or fractional count safely', () => {
    expect(quarterLayout(-3).cells.every((c) => c.house === null)).toBe(true)
    expect(quarterLayout(2.7).cells.filter((c) => c.house !== null)).toHaveLength(2)
  })
})
