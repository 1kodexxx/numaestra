/* Фирменная марка Numaestra — упрощённый эквалайзер из 5 скруглённых столбиков.
   Читается даже в мелком размере (навбар, favicon). Тянет цвет из пропа color;
   столбики получают класс brand-bar для анимации (см. global.css). */

const HEIGHTS = [9, 15, 22, 15, 9]
const W = 3
const GAP = 2.25

export function BrandMark({ size = 24, color = '#00e5c0' }: { size?: number; color?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill={color} role="img" aria-label="Numaestra">
      {HEIGHTS.map((h, i) => (
        <rect
          key={i}
          className="brand-bar"
          x={i * (W + GAP)}
          y={(24 - h) / 2}
          width={W}
          height={h}
          rx={W / 2}
          style={{ animationDelay: `${i * 0.12}s` }}
        />
      ))}
    </svg>
  )
}
