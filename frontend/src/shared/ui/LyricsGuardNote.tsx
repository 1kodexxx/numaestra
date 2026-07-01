import { detectRiskyTerms } from '@shared/lib/lyricsGuard'

// LyricsGuardNote — предупреждение под полем текста песни. Всегда показывает
// общий совет (реальные артисты/бренды ломают генерацию Suno; мат — можно), а при
// обнаружении конкретных упоминаний подсвечивает их красным. Не блокирует ввод.
export function LyricsGuardNote({ text, style }: { text: string; style?: React.CSSProperties }) {
  const risky = detectRiskyTerms(text)

  if (risky.length > 0) {
    return (
      <div
        style={{
          display: 'flex',
          gap: '8px',
          padding: '10px 12px',
          borderRadius: '10px',
          background: 'rgba(239,68,68,0.1)',
          border: '1px solid rgba(239,68,68,0.35)',
          fontSize: '12px',
          color: '#f87171',
          lineHeight: 1.5,
          ...style,
        }}
      >
        <span style={{ flexShrink: 0 }}>🚫</span>
        <span>
          Уберите из текста:{' '}
          {risky.map((t, i) => (
            <span key={t}>
              <b>«{t}»</b>
              {i < risky.length - 1 ? ', ' : ''}
            </span>
          ))}{' '}
          — это реальные артисты/бренды, ИИ-студия такую песню не создаст, и демо не сгенерируется.
        </span>
      </div>
    )
  }

  return (
    <div
      style={{
        display: 'flex',
        gap: '8px',
        padding: '10px 12px',
        borderRadius: '10px',
        background: 'rgba(245,158,11,0.08)',
        border: '1px solid rgba(245,158,11,0.25)',
        fontSize: '12px',
        color: '#fbbf24',
        lineHeight: 1.5,
        ...style,
      }}
    >
      <span style={{ flexShrink: 0 }}>⚠️</span>
      <span>
        Не упоминайте <b>реальных артистов и названия брендов</b> (например «Ludacris»,
        «Wildberries») — ИИ-студия такие песни не создаёт, и демо не сгенерируется.
      </span>
    </div>
  )
}
