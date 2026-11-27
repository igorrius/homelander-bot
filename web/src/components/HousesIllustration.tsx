import { useTranslation } from 'react-i18next'

const width = 150
const gap = 30
const ground = 340
const floorsPattern = [6, 8, 5, 7]
const palette = ['#e8a87c', '#85b7c9', '#c38d9e', '#9ab87a', '#e2c275']

/** One stylized house: body with windows, a roof, a door and a label. */
function House({ index, x, name }: { index: number; x: number; name?: string }) {
  const floors = floorsPattern[index % floorsPattern.length]
  const floorH = 28
  const h = floors * floorH + 20
  const top = ground - h
  const windows = []
  for (let f = 0; f < floors - 1; f++) {
    for (let c = 0; c < 4; c++) {
      windows.push(<rect key={`${f}-${c}`} x={x + 18 + c * 31} y={top + 14 + f * floorH} width={18} height={16} rx={2} fill="#fdf6d8" stroke="#5b5b5b" strokeWidth={0.5} />)
    }
  }
  return (
    <g data-testid="house">
      <polygon points={`${x - 6},${top} ${x + width / 2},${top - 34} ${x + width + 6},${top}`} fill="#8c4f3f" />
      <rect x={x} y={top} width={width} height={h} fill={palette[index % palette.length]} stroke="#5b5b5b" strokeWidth={1} />
      {windows}
      <rect x={x + width / 2 - 13} y={ground - 34} width={26} height={34} rx={3} fill="#6b4a3a" />
      {name !== undefined && (
        <text x={x + width / 2} y={ground + 26} textAnchor="middle" fontSize={18} fontWeight={600} fill="currentColor">
          {name}
        </text>
      )}
    </g>
  )
}

/**
 * Built-in picture of the community drawn from the house list: one house per
 * record, labelled with its name. Used when no picture was uploaded.
 */
export function HousesIllustration({ houses }: { houses: { id: number; name: string }[] }) {
  const { t } = useTranslation()
  const items = houses.length > 0 ? houses.map((h) => ({ key: String(h.id), name: h.name })) : [{ key: 'empty', name: undefined }]
  const total = items.length * width + (items.length + 1) * gap
  return (
    <svg role="img" aria-label={t('home.illustration')} viewBox={`0 0 ${total} ${ground + 44}`} width="100%" style={{ maxHeight: 360 }}>
      <circle cx={total - 40} cy={36} r={20} fill="#f6d365" />
      <rect x={0} y={ground} width={total} height={6} fill="#7a9a5a" />
      {items.map((it, i) => (
        <House key={it.key} index={i} x={gap + i * (width + gap)} name={it.name} />
      ))}
    </svg>
  )
}
