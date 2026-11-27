import { Badge, Box, Group, Text, useComputedColorScheme } from '@mantine/core'
import { useElementSize } from '@mantine/hooks'
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { quarterLayout } from '../lib/quarterLayout'

export interface QuarterHouse {
  id: number
  name: string
  floors: number | null
}

// Isometric grid: one unit along c (right-down) or r (left-down) is TW/2 × TH/2 pixels of the viewBox.
const TW = 64
const TH = 32
const CELL = 2.4 // grid cell, in units
const FOOT = 1.15 // house footprint
const MARGIN = 0.9 // yard around the grid
const SIDEWALK = 0.55
const ROAD = 1.5
const SLAB = 14 // thickness of the ground slab, px
const FLOOR = 5.2
const GROUND_FLOOR = 7
const MIN_FLOORS = 3
const MAX_FLOORS = 25
const DEFAULT_FLOORS = 9
const LABEL_CHARS = 18
const LABEL_PX = 12 // label size on screen, whatever the scale
const MAX_HEIGHT = 420
const PAD = 18

type Pt = [number, number]

const iso = (c: number, r: number): Pt => [((c - r) * TW) / 2, ((c + r) * TH) / 2]
const r1 = (n: number) => Math.round(n * 10) / 10
const pts = (p: Pt[]) => p.map(([x, y]) => `${r1(x)},${r1(y)}`).join(' ')
const up = ([x, y]: Pt, h: number): Pt => [x, y - h]
const quad = (c0: number, r0: number, c1: number, r1_: number): Pt[] => [iso(c0, r0), iso(c1, r0), iso(c1, r1_), iso(c0, r1_)]

/** A stable pseudo-random number in [0, 1) for a seed, so the drawing never changes between renders. */
function rand(seed: number): number {
  const x = Math.sin(seed * 12.9898 + 78.233) * 43758.5453
  return x - Math.floor(x)
}

function mix(hex: string, to: string, t: number): string {
  const a = parseInt(hex.slice(1), 16)
  const b = parseInt(to.slice(1), 16)
  const ch = (v: number, s: number) => (v >> s) & 255
  const m = (s: number) => Math.round(ch(a, s) + (ch(b, s) - ch(a, s)) * t)
  return `#${((m(16) << 16) | (m(8) << 8) | m(0)).toString(16).padStart(6, '0')}`
}

function houseFloors(floors: number | null | undefined): number {
  if (typeof floors !== 'number' || !Number.isFinite(floors)) return DEFAULT_FLOORS
  return Math.min(MAX_FLOORS, Math.max(MIN_FLOORS, Math.round(floors)))
}

function houseLabel(name: string): string {
  const chars = [...name.trim()]
  return chars.length > LABEL_CHARS ? `${chars.slice(0, LABEL_CHARS - 1).join('')}…` : chars.join('')
}

interface Palette {
  houses: string[]
  grass: string
  grassStripe: string
  path: string
  sidewalk: string
  curb: string
  road: string
  marking: string
  soilL: string
  soilR: string
  trunk: string
  leaf: [string, string, string]
  conifer: [string, string]
  bush: string
  glass: string
  glassAlt: string
  lit: string
  door: string
  roofBox: string
  sand: string
  shadow: string
  labelBg: string
  labelText: string
  labelBorder: string
  lampHead: string
  cars: string[]
  dark: boolean
}

const light: Palette = {
  houses: ['#f2b880', '#8fc1d4', '#dba9bb', '#efd27c', '#a9c3ec', '#b4d6a3'],
  grass: '#a8d48f',
  grassStripe: '#b6dc9e',
  path: '#efe4cb',
  sidewalk: '#ddd7cc',
  curb: '#b9b2a6',
  road: '#5d6470',
  marking: '#f4f4f2',
  soilL: '#bb8f66',
  soilR: '#a37b55',
  trunk: '#8a5a3b',
  leaf: ['#3f8f3a', '#5cae4d', '#337a30'],
  conifer: ['#2f7d4f', '#3d9560'],
  bush: '#4f9b44',
  glass: '#d9e9f3',
  glassAlt: '#f5f9fb',
  lit: '#fff0c2',
  door: '#5d4636',
  roofBox: '#cfc8bb',
  sand: '#ecd9a6',
  shadow: 'rgba(30, 60, 40, 0.16)',
  labelBg: 'rgba(255, 255, 255, 0.94)',
  labelText: '#1c332a',
  labelBorder: 'rgba(28, 51, 42, 0.14)',
  lampHead: '#f7f1d8',
  cars: ['#e4572e', '#2e86ab', '#f2c14e'],
  dark: false,
}

const dark: Palette = {
  houses: ['#9a6b4b', '#4f7889', '#86606f', '#9a8546', '#5d6f96', '#5f7f53'],
  grass: '#2c5236',
  grassStripe: '#30583a',
  path: '#454a40',
  sidewalk: '#4a4d53',
  curb: '#393b40',
  road: '#23272d',
  marking: '#8d939a',
  soilL: '#4c3b2d',
  soilR: '#3e3125',
  trunk: '#4f3727',
  leaf: ['#1f4a2a', '#2b6035', '#183b22'],
  conifer: ['#173f2b', '#1f5138'],
  bush: '#22492a',
  glass: '#26303d',
  glassAlt: '#2d3948',
  lit: '#ffd27a',
  door: '#ffcf7a',
  roofBox: '#3d4149',
  sand: '#5a5240',
  shadow: 'rgba(0, 0, 0, 0.28)',
  labelBg: 'rgba(22, 28, 36, 0.92)',
  labelText: '#eef2f5',
  labelBorder: 'rgba(255, 255, 255, 0.12)',
  lampHead: '#ffe7a8',
  cars: ['#b5452a', '#2a6a86', '#c29a3c'],
  dark: true,
}

interface Drawn {
  depth: number
  node: ReactNode
}

interface Anchor {
  key: number
  /** The house's place in the list, shown as its number when names do not fit. */
  n: number
  x: number
  y: number
  text: string
}

interface Scene {
  ground: ReactNode[]
  objects: Drawn[]
  anchors: Anchor[]
  bounds: { minX: number; minY: number; maxX: number; maxY: number }
}

function tree(key: string, [x, y]: Pt, seed: number, p: Palette): Drawn & { shadow: ReactNode } {
  const k = 0.85 + rand(seed) * 0.35
  const conifer = rand(seed + 1) < 0.3
  const shadow = <ellipse key={`s${key}`} cx={r1(x + 4 * k)} cy={r1(y + 1)} rx={r1(9 * k)} ry={r1(4 * k)} fill={p.shadow} />
  const node = conifer ? (
    <g key={key} data-testid="tree">
      <rect x={r1(x - 1.2)} y={r1(y - 6)} width={2.4} height={6} fill={p.trunk} />
      <polygon points={pts([[x - 8 * k, y - 5], [x + 8 * k, y - 5], [x, y - 24 * k]])} fill={p.conifer[0]} />
      <polygon points={pts([[x - 6 * k, y - 13 * k], [x + 6 * k, y - 13 * k], [x, y - 31 * k]])} fill={p.conifer[1]} />
    </g>
  ) : (
    <g key={key} data-testid="tree">
      <rect x={r1(x - 1.6)} y={r1(y - 9)} width={3.2} height={9} fill={p.trunk} />
      <circle cx={r1(x)} cy={r1(y - 16 * k)} r={r1(9 * k)} fill={p.leaf[0]} />
      <circle cx={r1(x + 3 * k)} cy={r1(y - 13 * k)} r={r1(5.5 * k)} fill={p.leaf[2]} />
      <circle cx={r1(x - 3 * k)} cy={r1(y - 19 * k)} r={r1(5.5 * k)} fill={p.leaf[1]} />
    </g>
  )
  return { depth: 0, node, shadow }
}

function bush(key: string, [x, y]: Pt, p: Palette): ReactNode {
  return (
    <g key={key}>
      <ellipse cx={r1(x)} cy={r1(y - 2)} rx={6.5} ry={3.8} fill={p.bush} />
      <ellipse cx={r1(x - 1.5)} cy={r1(y - 3.2)} rx={3.2} ry={1.8} fill={p.leaf[1]} opacity={0.8} />
    </g>
  )
}

/** One block of flats: two lit faces, windows per floor, a flat roof, a door towards the street. */
function house(i: number, q: QuarterHouse, c0: number, r0: number, colour: string, p: Palette): Drawn & { shadow: ReactNode; roof: Pt } {
  const c1 = c0 + FOOT
  const rr1 = r0 + FOOT
  const floors = houseFloors(q.floors)
  const h = GROUND_FLOOR + floors * FLOOR
  const left = mix(colour, '#000000', p.dark ? 0.28 : 0.16)
  const right = colour
  const top = mix(colour, '#ffffff', p.dark ? 0.12 : 0.42)
  const parapet = mix(colour, '#ffffff', p.dark ? 0.04 : 0.22)
  const nodes: ReactNode[] = []
  // Faces: r = r1 faces the street (left on screen), c = c1 the side (right on screen).
  const lf: Pt[] = [iso(c0, rr1), iso(c1, rr1), up(iso(c1, rr1), h), up(iso(c0, rr1), h)]
  const rf: Pt[] = [iso(c1, rr1), iso(c1, r0), up(iso(c1, r0), h), up(iso(c1, rr1), h)]
  nodes.push(<polygon key="l" points={pts(lf)} fill={left} />, <polygon key="r" points={pts(rf)} fill={right} />)
  // A darker plinth for the ground floor.
  nodes.push(
    <polygon key="pl" points={pts([iso(c0, rr1), iso(c1, rr1), up(iso(c1, rr1), 3), up(iso(c0, rr1), 3)])} fill={mix(left, '#000000', 0.18)} />,
    <polygon key="pr" points={pts([iso(c1, rr1), iso(c1, r0), up(iso(c1, r0), 3), up(iso(c1, rr1), 3)])} fill={mix(right, '#000000', 0.15)} />,
  )
  const cols = [0.2, 0.5, 0.8]
  const ww = 0.13
  for (let f = 0; f < floors; f++) {
    const y0 = GROUND_FLOOR + f * FLOOR + 1.4
    const y1 = y0 + 2.7
    cols.forEach((t, k) => {
      const seed = i * 997 + f * 31 + k
      const litL = p.dark ? rand(seed) < 0.42 : rand(seed) < 0.12
      const litR = p.dark ? rand(seed + 7) < 0.42 : rand(seed + 7) < 0.12
      const ca = c0 + FOOT * t - ww / 2
      const cb = ca + ww
      nodes.push(
        <polygon key={`wl${f}-${k}`} points={pts([up(iso(ca, rr1), y0), up(iso(cb, rr1), y0), up(iso(cb, rr1), y1), up(iso(ca, rr1), y1)])}
          fill={litL ? p.lit : p.glass} />,
      )
      const ra = r0 + FOOT * t - ww / 2
      const rb = ra + ww
      nodes.push(
        <polygon key={`wr${f}-${k}`} points={pts([up(iso(c1, ra), y0), up(iso(c1, rb), y0), up(iso(c1, rb), y1), up(iso(c1, ra), y1)])}
          fill={litR ? p.lit : p.glassAlt} />,
      )
    })
  }
  // Door with a small canopy, facing the street.
  const cm = c0 + FOOT / 2
  nodes.push(
    <polygon key="door" points={pts([iso(cm - 0.1, rr1), iso(cm + 0.1, rr1), up(iso(cm + 0.1, rr1), 6.5), up(iso(cm - 0.1, rr1), 6.5)])} fill={p.door} />,
    <polygon key="canopy" points={pts([up(iso(cm - 0.15, rr1), 7), up(iso(cm + 0.15, rr1), 7), up(iso(cm + 0.15, rr1 + 0.12), 7.5), up(iso(cm - 0.15, rr1 + 0.12), 7.5)])}
      fill={mix(left, '#000000', 0.3)} />,
  )
  // Roof with a parapet rim, and a lift room on taller blocks.
  nodes.push(<polygon key="roof" points={pts(quad(c0, r0, c1, rr1).map((q_) => up(q_, h)))} fill={parapet} />)
  nodes.push(<polygon key="roofin" points={pts(quad(c0 + 0.07, r0 + 0.07, c1 - 0.07, rr1 - 0.07).map((q_) => up(q_, h - 0.6)))} fill={top} />)
  let roofTop = h
  if (floors >= 9) {
    const bc0 = c0 + 0.22
    const br0 = r0 + 0.22
    const bh = 6
    const bc1 = bc0 + 0.34
    const br1 = br0 + 0.3
    nodes.push(
      <polygon key="bl" points={pts([up(iso(bc0, br1), h), up(iso(bc1, br1), h), up(iso(bc1, br1), h + bh), up(iso(bc0, br1), h + bh)])} fill={mix(p.roofBox, '#000000', 0.15)} />,
      <polygon key="br" points={pts([up(iso(bc1, br1), h), up(iso(bc1, br0), h), up(iso(bc1, br0), h + bh), up(iso(bc1, br1), h + bh)])} fill={p.roofBox} />,
      <polygon key="bt" points={pts(quad(bc0, br0, bc1, br1).map((q_) => up(q_, h + bh)))} fill={mix(p.roofBox, '#ffffff', 0.3)} />,
    )
    roofTop = h + bh
  }
  // Shadow to the right (light from the upper left), proportional to the height.
  const s = h / 55
  const shadow = (
    <polygon key={`hs${i}`} fill={p.shadow}
      points={pts([iso(c0, rr1), iso(c1, rr1), iso(c1 + s, rr1 - s * 0.35), iso(c1 + s, r0 - s * 0.35), iso(c0 + s, r0 - s * 0.35), iso(c0, r0)])} />
  )
  const node = (
    <g key={`h${q.id}`} data-testid="house" className="q-house">
      <title>{q.name}</title>
      {nodes}
    </g>
  )
  const [cx, cy] = iso(c0 + FOOT / 2, r0 + FOOT / 2)
  return { depth: c0 + r0 + FOOT, node, shadow, roof: [cx, cy - roofTop] }
}

function lamp(key: string, c: number, r: number, p: Palette): Drawn & { glow?: ReactNode } {
  const [x, y] = iso(c, r)
  const [hx, hy] = [x - 5, y - 27]
  const glow = p.dark ? (
    <g key={`g${key}`}>
      <ellipse cx={r1(x - 8)} cy={r1(y + 5)} rx={18} ry={8} fill="url(#q-pool)" />
    </g>
  ) : undefined
  const node = (
    <g key={key}>
      <rect x={r1(x - 0.8)} y={r1(y - 27)} width={1.6} height={27} fill={p.dark ? '#6b7079' : '#4c535c'} />
      <path d={`M${r1(x)},${r1(y - 26)} q-2,-2 -5,-1`} stroke={p.dark ? '#6b7079' : '#4c535c'} strokeWidth={1.2} fill="none" />
      {p.dark && <circle cx={r1(hx)} cy={r1(hy + 1.5)} r={11} fill="url(#q-glow)" />}
      <ellipse cx={r1(hx)} cy={r1(hy + 1.5)} rx={2.6} ry={1.6} fill={p.lampHead} />
    </g>
  )
  return { depth: c + r, node, glow }
}

function car(key: string, c: number, r: number, colour: string, p: Palette): Drawn {
  const L = 0.72
  const W = 0.36
  const c0 = c - L / 2
  const r0 = r - W / 2
  const c1 = c0 + L
  const rr1 = r0 + W
  const body = 6
  const side = mix(colour, '#000000', 0.2)
  const cab = { c0: c0 + 0.17, c1: c1 - 0.2, r0: r0 + 0.04, r1: rr1 - 0.04 }
  const cabH = 4.5
  return {
    depth: c + r,
    node: (
      <g key={key}>
        <polygon points={pts([iso(c0 + 0.05, rr1 + 0.05), iso(c1 + 0.15, rr1 + 0.05), iso(c1 + 0.15, r0), iso(c0 + 0.05, r0)])} fill={p.shadow} />
        <polygon points={pts([iso(c0, rr1), iso(c1, rr1), up(iso(c1, rr1), body), up(iso(c0, rr1), body)])} fill={side} />
        <polygon points={pts([iso(c1, rr1), iso(c1, r0), up(iso(c1, r0), body), up(iso(c1, rr1), body)])} fill={colour} />
        <polygon points={pts(quad(c0, r0, c1, rr1).map((q_) => up(q_, body)))} fill={mix(colour, '#ffffff', 0.25)} />
        <polygon points={pts([up(iso(cab.c0, cab.r1), body), up(iso(cab.c1, cab.r1), body), up(iso(cab.c1, cab.r1), body + cabH), up(iso(cab.c0, cab.r1), body + cabH)])}
          fill={p.dark ? '#3a4656' : '#cfe2ee'} />
        <polygon points={pts([up(iso(cab.c1, cab.r1), body), up(iso(cab.c1, cab.r0), body), up(iso(cab.c1, cab.r0), body + cabH), up(iso(cab.c1, cab.r1), body + cabH)])}
          fill={p.dark ? '#46546a' : '#e3eff6'} />
        <polygon points={pts(quad(cab.c0, cab.r0, cab.c1, cab.r1).map((q_) => up(q_, body + cabH)))} fill={mix(colour, '#ffffff', 0.15)} />
        {p.dark && <circle cx={r1(iso(c1, rr1 - 0.08)[0])} cy={r1(iso(c1, rr1 - 0.08)[1] - 3)} r={1.3} fill="#fff4c8" />}
      </g>
    ),
  }
}

function buildScene(houses: QuarterHouse[], p: Palette): Scene {
  const layout = quarterLayout(houses.length)
  const { cols, rows } = layout
  const cMin = -MARGIN
  const cMax = cols * CELL + MARGIN
  const rMin = -MARGIN
  const yardEnd = rows * CELL + 0.15
  const walkEnd = yardEnd + SIDEWALK
  const rMax = walkEnd + ROAD
  const roadMid = walkEnd + ROAD / 2
  const ground: ReactNode[] = []
  const shadows: ReactNode[] = []
  const objects: Drawn[] = []
  const anchors: Anchor[] = []

  // The slab: soil sides under the front edges, with a thin band of what lies on top.
  const sideL = (a: number, b: number, band: string, key: string) => {
    const pa = iso(a, rMax)
    const pb = iso(b, rMax)
    ground.push(
      <polygon key={`sl${key}`} points={pts([pa, pb, [pb[0], pb[1] + SLAB], [pa[0], pa[1] + SLAB]])} fill={p.soilL} />,
      <polygon key={`slb${key}`} points={pts([pa, pb, [pb[0], pb[1] + 3], [pa[0], pa[1] + 3]])} fill={band} />,
    )
  }
  const sideR = (a: number, b: number, band: string, key: string) => {
    const pa = iso(cMax, a)
    const pb = iso(cMax, b)
    ground.push(
      <polygon key={`sr${key}`} points={pts([pa, pb, [pb[0], pb[1] + SLAB], [pa[0], pa[1] + SLAB]])} fill={p.soilR} />,
      <polygon key={`srb${key}`} points={pts([pa, pb, [pb[0], pb[1] + 3], [pa[0], pa[1] + 3]])} fill={band} />,
    )
  }
  sideL(cMin, cMax, mix(p.road, '#000000', 0.25), 'road')
  sideR(rMin, yardEnd, mix(p.grass, '#000000', 0.25), 'yard')
  sideR(yardEnd, walkEnd, mix(p.sidewalk, '#000000', 0.25), 'walk')
  sideR(walkEnd, rMax, mix(p.road, '#000000', 0.35), 'road')

  // Lawn with mowing stripes.
  ground.push(<polygon key="yard" points={pts(quad(cMin, rMin, cMax, yardEnd))} fill={p.grass} />)
  for (let c = cMin, k = 0; c < cMax; c += 0.7, k++) {
    if (k % 2) ground.push(<polygon key={`st${k}`} points={pts(quad(c, rMin, Math.min(cMax, c + 0.7), yardEnd))} fill={p.grassStripe} />)
  }
  // Paths between the cells, and from every door to the next path.
  const pw = 0.11
  for (let k = 1; k < cols; k++) ground.push(<polygon key={`pc${k}`} points={pts(quad(k * CELL - pw, rMin * 0.45, k * CELL + pw, yardEnd))} fill={p.path} />)
  for (let k = 1; k < rows; k++) ground.push(<polygon key={`pr${k}`} points={pts(quad(cMin * 0.45, k * CELL - pw, cMax - MARGIN * 0.55, k * CELL + pw))} fill={p.path} />)
  // Sidewalk, curb, road with a dashed centre line and a zebra crossing.
  ground.push(
    <polygon key="walk" points={pts(quad(cMin, yardEnd, cMax, walkEnd))} fill={p.sidewalk} />,
    <polygon key="curb" points={pts(quad(cMin, walkEnd - 0.06, cMax, walkEnd))} fill={p.curb} />,
    <polygon key="road" points={pts(quad(cMin, walkEnd, cMax, rMax))} fill={p.road} />,
  )
  const zebra = (cols * CELL) / 2
  for (let c = cMin + 0.25, k = 0; c < cMax - 0.4; c += 0.75, k++) {
    if (Math.abs(c + 0.2 - zebra) < 0.9) continue
    ground.push(<polygon key={`m${k}`} points={pts(quad(c, roadMid - 0.035, c + 0.4, roadMid + 0.035))} fill={p.marking} opacity={0.9} />)
  }
  for (let k = 0; k < 5; k++) {
    const c = zebra - 0.6 + k * 0.26
    ground.push(<polygon key={`z${k}`} points={pts(quad(c, walkEnd + 0.12, c + 0.13, rMax - 0.12))} fill={p.marking} opacity={0.92} />)
  }

  // Houses, their paths, greenery around them; empty cells get a playground or a grove.
  let firstEmpty = true
  for (const cell of layout.cells) {
    const bc = cell.col * CELL
    const br = cell.row * CELL
    const seed = cell.col * 17 + cell.row * 131
    const greens: [number, number][] = []
    if (cell.house !== null) {
      const q = houses[cell.house]
      const c0 = bc + (CELL - FOOT) / 2
      const r0 = br + (CELL - FOOT) / 2
      const colour = p.houses[(cell.col + cell.row * 2) % p.houses.length]
      const d = house(cell.house, q, c0, r0, colour, p)
      shadows.push(d.shadow)
      objects.push({ depth: d.depth, node: d.node })
      anchors.push({ key: q.id, n: cell.house + 1, x: d.roof[0], y: d.roof[1], text: houseLabel(q.name) })
      const cm = c0 + FOOT / 2
      ground.push(<polygon key={`e${q.id}`} points={pts(quad(cm - 0.09, r0 + FOOT, cm + 0.09, br + CELL))} fill={p.path} />)
      // Bushes along the front, trees in two of the four corners.
      ground.push(bush(`b1${q.id}`, iso(c0 + 0.22, r0 + FOOT + 0.22), p), bush(`b2${q.id}`, iso(c0 + FOOT - 0.2, r0 + FOOT + 0.22), p))
      const corners: [number, number][] = [[0.38, 0.38], [CELL - 0.4, 0.42], [0.42, CELL - 0.38], [CELL - 0.38, CELL - 0.4]]
      corners.forEach((cr, k) => {
        if (rand(seed + k * 5) < 0.55) greens.push(cr)
      })
    } else if (firstEmpty) {
      firstEmpty = false
      // A sandbox and a little climbing frame.
      const s0 = bc + 0.75
      const s1 = bc + 1.65
      ground.push(
        <polygon key={`sb${seed}`} points={pts(quad(s0, br + 0.75, s1, br + 1.65))} fill={mix(p.path, '#000000', 0.12)} />,
        <polygon key={`sa${seed}`} points={pts(quad(s0 + 0.06, br + 0.81, s1 - 0.06, br + 1.59))} fill={p.sand} />,
      )
      const [fx, fy] = iso(bc + 1.2, br + 1.2)
      objects.push({
        depth: bc + br + 2.4,
        node: (
          <g key={`pg${seed}`}>
            <path d={`M${r1(fx - 10)},${r1(fy + 3)} L${r1(fx - 6)},${r1(fy - 14)} L${r1(fx - 2)},${r1(fy + 1)} M${r1(fx + 2)},${r1(fy + 5)} L${r1(fx + 6)},${r1(fy - 12)} L${r1(fx + 10)},${r1(fy + 3)} M${r1(fx - 6)},${r1(fy - 14)} L${r1(fx + 6)},${r1(fy - 12)}`}
              stroke={p.dark ? '#8a6f4f' : '#c0583c'} strokeWidth={1.6} fill="none" strokeLinecap="round" />
            <line x1={r1(fx - 1)} y1={r1(fy - 13)} x2={r1(fx - 1)} y2={r1(fy - 4)} stroke={p.dark ? '#777' : '#555'} strokeWidth={0.6} />
            <rect x={r1(fx - 3)} y={r1(fy - 4)} width={4} height={1.4} fill={p.dark ? '#8d7150' : '#2e86ab'} />
          </g>
        ),
      })
      greens.push([0.4, 0.4], [CELL - 0.4, 0.45], [0.45, CELL - 0.35], [CELL - 0.35, CELL - 0.4])
    } else {
      greens.push([0.7, 0.8], [1.6, 0.7], [1.15, 1.55], [0.5, 1.9], [1.95, 1.75])
      ground.push(bush(`gb${seed}`, iso(bc + 1.9, br + 1.2), p), bush(`gb2${seed}`, iso(bc + 0.55, br + 1.3), p))
    }
    greens.forEach(([dc, dr], k) => {
      const t = tree(`t${seed}-${k}`, iso(bc + dc, br + dr), seed * 7 + k, p)
      shadows.push(t.shadow)
      objects.push({ depth: bc + dc + br + dr, node: t.node })
    })
  }
  // A grove along the two back edges, a few trees along the right edge.
  let s = 0
  for (let r = rMin + 0.4; r < yardEnd - 0.3; r += 0.8) {
    const t = tree(`bl${s}`, iso(cMin + 0.38, r), 500 + s, p)
    shadows.push(t.shadow)
    objects.push({ depth: cMin + 0.38 + r, node: t.node })
    s++
  }
  for (let c = cMin + 1.1; c < cMax - 0.3; c += 0.8) {
    const t = tree(`br${s}`, iso(c, rMin + 0.38), 500 + s, p)
    shadows.push(t.shadow)
    objects.push({ depth: c + rMin + 0.38, node: t.node })
    s++
  }
  for (let r = rMin + 1.9; r < yardEnd - 0.6; r += CELL) {
    const t = tree(`rr${s}`, iso(cMax - 0.35, r), 500 + s, p)
    shadows.push(t.shadow)
    objects.push({ depth: cMax - 0.35 + r, node: t.node })
    s++
  }
  // Street lamps on the sidewalk between the houses, two cars on the road.
  const glows: ReactNode[] = []
  for (let k = 0; k <= cols; k++) {
    const l = lamp(`l${k}`, Math.min(cMax - 0.3, Math.max(cMin + 0.3, k * CELL)), walkEnd - 0.14, p)
    if (l.glow) glows.push(l.glow)
    objects.push({ depth: l.depth, node: l.node })
  }
  objects.push(car('car1', cMin + (cMax - cMin) * 0.24, roadMid + 0.36, p.cars[0], p))
  if (cols >= 3) objects.push(car('car2', cMin + (cMax - cMin) * 0.74, roadMid - 0.36, p.cars[1], p))

  ground.push(...shadows, ...glows)
  objects.sort((a, b) => a.depth - b.depth)

  const [lx] = iso(cMin, rMax)
  const [rx] = iso(cMax, rMin)
  const [, ty] = iso(cMin, rMin)
  const [, by] = iso(cMax, rMax)
  const tallest = Math.min(...anchors.map((a) => a.y), ty - 40)
  return { ground, objects, anchors, bounds: { minX: lx - PAD, maxX: rx + PAD, minY: tallest - PAD, maxY: by + SLAB + PAD } }
}

interface Placed extends Anchor {
  w: number
  h: number
  cy: number
}

/**
 * Puts labels above the roofs; a label that would cover one already placed moves up. Returns null
 * when names were asked for and any of them had to move: names then give way to numbers.
 */
function placeLabels(anchors: Anchor[], font: number, numbers: boolean): Placed[] | null {
  const placed: Placed[] = []
  const order = [...anchors].sort((a, b) => b.y - a.y)
  for (const a of order) {
    const text = numbers ? String(a.n) : a.text
    const h = font * (numbers ? 1.6 : 1.75)
    const w = numbers ? Math.max(h, text.length * font * 0.62 + font * 0.8) : [...text].length * font * 0.58 + font * 1.3
    let cy = a.y - font * 0.7 - h / 2
    for (let tries = 0; tries < 8; tries++) {
      const hit = placed.some((o) => Math.abs(o.x - a.x) < (o.w + w) / 2 + 3 && Math.abs(o.cy - cy) < (o.h + h) / 2 + 3)
      if (!hit) break
      if (!numbers) return null
      cy -= h + 3
    }
    placed.push({ ...a, text, w, h, cy })
  }
  return placed
}

/**
 * The built-in page's picture: an isometric quarter with one block of flats per house (height from
 * its floors), a shared green yard with paths and trees, and a street with lamps along the front.
 * Light by day, lit windows and lamps by night (dark theme).
 */
export function Quarter({ houses }: { houses: QuarterHouse[] }) {
  const { t } = useTranslation()
  const scheme = useComputedColorScheme('light')
  const p = scheme === 'dark' ? dark : light
  const { ref, width } = useElementSize()
  const scene = useMemo(() => buildScene(houses, p), [houses, p])
  const { minX, maxX, minY, maxY } = scene.bounds
  const vw = maxX - minX
  const vh0 = maxY - minY
  const phone = width > 0 && width < 576
  // Screen pixels per viewBox unit, so labels keep a readable size whatever the quarter's scale.
  const scale = width > 0 ? Math.min(width / vw, phone ? Infinity : MAX_HEIGHT / vh0) : 1
  const font = (phone ? LABEL_PX - 2 : LABEL_PX) / scale
  const named = placeLabels(scene.anchors, font, false)
  const numbers = named === null
  const labels = named ?? placeLabels(scene.anchors, font, true) ?? []
  const top = Math.min(minY, ...labels.map((l) => l.cy - l.h / 2 - 4))
  const vh = maxY - top
  const names = houses.map((h) => h.name)
  const label = names.length ? `${t('home.illustration')}: ${names.join(', ')}` : t('home.illustration')

  return (
    <Box ref={ref} w="100%">
      <svg
        role="img"
        aria-label={label}
        viewBox={`${r1(minX)} ${r1(top)} ${r1(vw)} ${r1(vh)}`}
        width="100%"
        style={{ display: 'block', height: 'auto', maxHeight: phone ? undefined : MAX_HEIGHT, overflow: 'visible' }}
      >
        <defs>
          <radialGradient id="q-glow">
            <stop offset="0" stopColor="#ffd98a" stopOpacity={0.75} />
            <stop offset="1" stopColor="#ffd98a" stopOpacity={0} />
          </radialGradient>
          <radialGradient id="q-pool">
            <stop offset="0" stopColor="#ffd98a" stopOpacity={0.28} />
            <stop offset="1" stopColor="#ffd98a" stopOpacity={0} />
          </radialGradient>
        </defs>
        <style>{`.q-house{transition:transform .25s ease}.q-house:hover{transform:translateY(-3px)}@media (prefers-reduced-motion:reduce){.q-house{transition:none}}`}</style>
        {scene.ground}
        {scene.objects.map((o) => o.node)}
        <g>
          {labels.map((l) => (
            <g key={`lab${l.key}`} aria-hidden="true">
              {l.cy + l.h / 2 < l.y - font * 0.9 && (
                <line x1={r1(l.x)} y1={r1(l.y - 2)} x2={r1(l.x)} y2={r1(l.cy + l.h / 2)} stroke={p.labelText} strokeOpacity={0.35} strokeWidth={r1(Math.max(0.8, 1 / scale))} />
              )}
              <rect x={r1(l.x - l.w / 2)} y={r1(l.cy - l.h / 2 + font * 0.12)} width={r1(l.w)} height={r1(l.h)} rx={r1(l.h / 2)} fill="#000000" opacity={0.1} />
              <rect x={r1(l.x - l.w / 2)} y={r1(l.cy - l.h / 2)} width={r1(l.w)} height={r1(l.h)} rx={r1(l.h / 2)} fill={p.labelBg}
                stroke={p.labelBorder} strokeWidth={r1(Math.max(0.6, 1 / scale))} />
              <text x={r1(l.x)} y={r1(l.cy)} dy="0.35em" textAnchor="middle" fontSize={r1(font)} fontWeight={650} fill={p.labelText}
                style={{ fontFamily: 'var(--mantine-font-family)' }}>
                {l.text}
              </text>
            </g>
          ))}
        </g>
      </svg>
      {numbers && houses.length > 0 && (
        <Group gap={6} justify="center" mt="sm" data-testid="quarter-legend">
          {houses.map((h, i) => (
            <Badge key={h.id} variant="light" color="gray" radius="sm" tt="none" fw={500}
              leftSection={<Text span fw={700} size="xs">{i + 1}</Text>}>
              {h.name}
            </Badge>
          ))}
        </Group>
      )}
    </Box>
  )
}
