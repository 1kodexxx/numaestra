import { isCompleteSongLyrics } from '@shared/lib/sunoPrompt'

// ShortLyricsNote — пояснение под полем «Свой текст песни», когда введена
// фраза/припев, а не полный текст. Реальный кейс: клиент вписал одну фразу
// («с любовью к папе») — раньше Suno спел бы ровно её и оборвал песню на ~30с.
// Теперь сервер вплетает короткие строки в полную песню (Inspiration Mode);
// заметка объясняет это клиенту заранее. Не блокирует ввод.
export function ShortLyricsNote({ text, style }: { text: string; style?: React.CSSProperties }) {
  if (!text.trim() || isCompleteSongLyrics(text)) return null

  return (
    <div
      style={{
        display: 'flex',
        gap: '8px',
        padding: '10px 12px',
        borderRadius: '10px',
        background: 'rgba(0,229,192,0.06)',
        border: '1px solid rgba(0,229,192,0.22)',
        fontSize: '12px',
        color: 'rgba(255,255,255,0.72)',
        lineHeight: 1.5,
        ...style,
      }}
    >
      <span style={{ flexShrink: 0 }}>✍️</span>
      <span>
        Эти строки прозвучат в песне <b style={{ color: '#fff' }}>дословно</b>, а остальной текст
        (куплеты и припев) допишет нейросеть. Хотите, чтобы пелся только ваш текст целиком —
        вставьте всю песню, а не отдельную фразу.
      </span>
    </div>
  )
}
