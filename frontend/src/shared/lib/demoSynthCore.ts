// Чистый (без DOM/Web Audio) синтез демо-трека в WAV ArrayBuffer.
// Используется и в Web Worker, и как фолбэк на основном потоке. Не зависит от
// OfflineAudioContext → работает в любом окружении, включая воркер.

const SR = 22050

function noteFreq(semitonesFromA4: number): number {
  return 440 * Math.pow(2, semitonesFromA4 / 12)
}

// Огибающая: линейная атака, удержание, линейный спад внутри отрезка длиной dur.
function env(tc: number, dur: number, attack: number, release: number): number {
  if (tc < attack) return tc / attack
  if (tc > dur - release) return Math.max(0, (dur - tc) / release)
  return 1
}

function encodeWav(samples: Float32Array): ArrayBuffer {
  const len = samples.length
  const view = new DataView(new ArrayBuffer(44 + len * 2))
  let p = 0
  const wstr = (s: string) => { for (let i = 0; i < s.length; i++) view.setUint8(p++, s.charCodeAt(i)) }
  const w16 = (v: number) => { view.setUint16(p, v, true); p += 2 }
  const w32 = (v: number) => { view.setUint32(p, v, true); p += 4 }
  wstr('RIFF'); w32(36 + len * 2); wstr('WAVE')
  wstr('fmt '); w32(16); w16(1); w16(1); w32(SR); w32(SR * 2); w16(2); w16(16)
  wstr('data'); w32(len * 2)
  for (let i = 0; i < len; i++) {
    const s = Math.max(-1, Math.min(1, samples[i]))
    view.setInt16(p, s < 0 ? s * 0x8000 : s * 0x7fff, true); p += 2
  }
  return view.buffer
}

/** Рендерит демо-трек в WAV. seed задаёт тональность/мажор-минор. */
export function renderWavBuffer(seed: number, seconds = 16): ArrayBuffer {
  const n = Math.floor(SR * seconds)
  const data = new Float32Array(n)

  const rootShift = (seed % 5) - 2
  const third = seed % 2 === 0 ? 3 : 4
  const prog = [-3, 4, -8, -1]
  const chordDur = seconds / prog.length
  const steps = 8
  const stepDur = chordDur / steps

  for (let i = 0; i < n; i++) {
    const t = i / SR
    let c = Math.floor(t / chordDur); if (c >= prog.length) c = prog.length - 1
    const tc = t - c * chordDur
    const root = prog[c] + rootShift
    const chord = [root, root + third, root + 7, root + 12]

    let s = 0

    // пэд
    const padEnv = env(tc, chordDur, 0.5, 0.5) * 0.14
    for (let k = 0; k < chord.length; k++) {
      s += Math.sin(2 * Math.PI * noteFreq(chord[k]) * t) * padEnv
    }

    // арпеджио (треугольная волна, октавой выше)
    let stp = Math.floor(tc / stepDur); if (stp >= steps) stp = steps - 1
    const te = tc - stp * stepDur
    const af = noteFreq(chord[stp % chord.length] + 12)
    const ph = (af * t) % 1
    const tri = 4 * Math.abs(ph - 0.5) - 1
    s += tri * Math.exp(-te * 14) * 0.2

    // бас
    s += Math.sin(2 * Math.PI * noteFreq(root - 12) * t) * env(tc, chordDur, 0.1, 0.25) * 0.26

    data[i] = s * 0.5
  }

  return encodeWav(data)
}
