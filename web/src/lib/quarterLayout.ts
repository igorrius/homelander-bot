export interface QuarterCell {
  col: number
  /** 0 is the back row; rows - 1 runs along the street. */
  row: number
  /** Index of the house standing here, or null for a green cell. */
  house: number | null
}

export interface QuarterLayout {
  cols: number
  rows: number
  cells: QuarterCell[]
}

/**
 * The grid of the built-in page's quarter: cols = max(2, ceil(sqrt(n × 1.8))), rows = ceil(n / cols).
 * Houses fill the rows along the street first; the back row takes the rest, centred, and its
 * free cells stay green. With no houses it is one green yard of two cells.
 */
export function quarterLayout(count: number): QuarterLayout {
  const n = Math.max(0, Math.floor(Number.isFinite(count) ? count : 0))
  const cols = Math.max(2, Math.ceil(Math.sqrt(n * 1.8)))
  const rows = Math.max(1, Math.ceil(n / cols))
  const cells: QuarterCell[] = []
  let next = 0
  for (let row = rows - 1; row >= 0; row--) {
    const inRow = Math.min(cols, n - next)
    const from = row === rows - 1 || inRow === cols ? 0 : Math.floor((cols - inRow) / 2)
    for (let col = 0; col < cols; col++) {
      const here = col >= from && col < from + inRow
      cells.push({ col, row, house: here ? next + col - from : null })
    }
    next += Math.max(0, inRow)
  }
  return { cols, rows, cells }
}
