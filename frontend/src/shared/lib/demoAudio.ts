// Синтез приятного демо-звучания (аккордовый пэд + арпеджио + бас) в WAV blob URL.
// Это осознанная заглушка для страницы примеров, пока нет реальных треков: плеер
// звучит, а не молчит. Полностью клиентский, без внешних зависимостей и лицензий.

const SR = 22050

function noteFreq(semitonesFromA4: number): number {
  return 440 * Math.pow(2, semitonesFromA4 / 12)
}

/** Детерминированный хэш строки → число (для стабильного «звучания» примера). */
export function hashStr(s: string): number {
  let h = 5381
  for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) >>> 0
  return h
}

function bufferToWavURL(buffer: AudioBuffer): string {
  const ch = buffer.getChannelData(0)
  const len = ch.length
  const view = new DataView(new ArrayBuffer(44 + len * 2))
  let p = 0
  const wstr = (s: string) => { for (let i = 0; i < s.length; i++) view.setUint8(p++, s.charCodeAt(i)) }
  const w16 = (v: number) => { view.setUint16(p, v, true); p += 2 }
  const w32 = (v: number) => { view.setUint32(p, v, true); p += 4 }
  wstr('RIFF'); w32(36 + len * 2); wstr('WAVE')
  wstr('fmt '); w32(16); w16(1); w16(1); w32(SR); w32(SR * 2); w16(2); w16(16)
  wstr('data'); w32(len * 2)
  for (let i = 0; i < len; i++) {
    const s = Math.max(-1, Math.min(1, ch[i]))
    view.setInt16(p, s < 0 ? s * 0x8000 : s * 0x7fff, true); p += 2
  }
  return URL.createObjectURL(new Blob([view], { type: 'audio/wav' }))
}

/**
 * Синтезирует короткий демо-трек. seed задаёт тональность/мажор-минор, чтобы разные
 * примеры и варианты звучали по-разному. Возвращает object URL и длительность (сек).
 * При отсутствии Web Audio (старый браузер) возвращает null — вызывающий код решает,
 * что делать.
 */
export async function synthDemoTrack(seed: number, seconds = 16): Promise<{ url: string; duration: number } | null> {
  const Ctx = window.OfflineAudioContext || (window as unknown as { webkitOfflineAudioContext?: typeof OfflineAudioContext }).webkitOfflineAudioContext
  if (!Ctx) return null

  const length = Math.floor(SR * seconds)
  const ctx = new Ctx(1, length, SR)

  const master = ctx.createGain()
  master.gain.value = 0.24
  master.connect(ctx.destination)

  const rootShift = (seed % 5) - 2          // сдвиг тональности −2..2 полутона
  const isMinor = seed % 2 === 0
  const third = isMinor ? 3 : 4
  const progression = [-3, 4, -8, -1]        // i–VI–IV–v-подобная последовательность
  const chordDur = seconds / progression.length

  for (let c = 0; c < progression.length; c++) {
    const root = progression[c] + rootShift
    const t0 = c * chordDur
    const chord = [root, root + third, root + 7, root + 12]

    // мягкий пэд (синусы)
    chord.forEach((st) => {
      const osc = ctx.createOscillator()
      osc.type = 'sine'
      osc.frequency.value = noteFreq(st)
      const g = ctx.createGain()
      g.gain.setValueAtTime(0, t0)
      g.gain.linearRampToValueAtTime(0.16, t0 + 0.5)
      g.gain.setValueAtTime(0.16, t0 + chordDur - 0.5)
      g.gain.linearRampToValueAtTime(0, t0 + chordDur)
      osc.connect(g); g.connect(master)
      osc.start(t0); osc.stop(t0 + chordDur)
    })

    // арпеджио (треугольная волна, октавой выше)
    const steps = 8
    const stepDur = chordDur / steps
    for (let s = 0; s < steps; s++) {
      const st = chord[s % chord.length] + 12
      const ta = t0 + s * stepDur
      const osc = ctx.createOscillator()
      osc.type = 'triangle'
      osc.frequency.value = noteFreq(st)
      const g = ctx.createGain()
      g.gain.setValueAtTime(0, ta)
      g.gain.linearRampToValueAtTime(0.22, ta + 0.02)
      g.gain.exponentialRampToValueAtTime(0.0008, ta + stepDur * 0.95)
      osc.connect(g); g.connect(master)
      osc.start(ta); osc.stop(ta + stepDur)
    }

    // бас
    const bass = ctx.createOscillator()
    bass.type = 'sine'
    bass.frequency.value = noteFreq(root - 12)
    const bg = ctx.createGain()
    bg.gain.setValueAtTime(0, t0)
    bg.gain.linearRampToValueAtTime(0.28, t0 + 0.1)
    bg.gain.setValueAtTime(0.28, t0 + chordDur - 0.25)
    bg.gain.linearRampToValueAtTime(0, t0 + chordDur)
    bass.connect(bg); bg.connect(master)
    bass.start(t0); bass.stop(t0 + chordDur)
  }

  const rendered = await ctx.startRendering()
  return { url: bufferToWavURL(rendered), duration: seconds }
}
