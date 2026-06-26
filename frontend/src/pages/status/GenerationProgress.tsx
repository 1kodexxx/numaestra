import type { GenerationStatus } from "@entities/order";
import { theme } from "@shared/lib/theme";
import { useEffect, useRef, useState } from "react";

const ACCENT = theme.accent;
const ACCENT2 = theme.accent2;
const TEXT2 = theme.text2;
const TEXT3 = theme.text3;

const TRACK_COUNT = 4;
const ESTIMATE_SEC = 10 * 60;
const FALLBACK_CAP = 94;
// Потолок «ползущего» бара: тянемся к нему медленно, чтобы прогресс НИКОГДА не
// замирал, даже если сервер/Suno долго держит одно значение (плато генерации).
const CREEP_CAP = 96;

const PHASE_LABELS: Record<string, string> = {
  queued: "Заказ принят — становимся в очередь…",
  preparing: "Готовим заказ к генерации…",
  lyrics: "✍️ Пишем текст песни под ваш повод…",
  submitting: "Отправляем задание в AI-студию…",
  generating: "🎼 Создаём музыку…",
  uploading: "☁️ Сохраняем готовые версии…",
  completed: "Песня готова 🎉",
};

/* Этапы «студийного» конвейера — для степпера. */
const STAGES = [
  { icon: "📋", label: "Очередь" },
  { icon: "✍️", label: "Текст" },
  { icon: "🎼", label: "Музыка" },
  { icon: "🎚️", label: "Сведение" },
  { icon: "✨", label: "Готово" },
];
const PHASE_STAGE: Record<string, number> = {
  queued: 0,
  preparing: 1,
  lyrics: 1,
  submitting: 2,
  generating: 2,
  uploading: 3,
  completed: 4,
};

/* Живые подсказки «что сейчас происходит в студии» — крутятся внутри фазы,
 * чтобы процесс не выглядел застывшим, даже когда проценты ползут медленно. */
const FLAVOR: Record<string, string[]> = {
  queued: ["Резервируем студийное время", "Готовим всё к записи"],
  preparing: ["Настраиваем инструменты", "Прогреваем нейросеть"],
  lyrics: ["Подбираем рифмы", "Сочиняем припев", "Расставляем акценты в строках"],
  submitting: ["Передаём задание AI-студии", "Загружаем стиль и настроение"],
  generating: [
    "🎹 Подбираем аккорды",
    "🎸 Аранжируем партии",
    "🎤 Записываем вокал",
    "🥁 Выставляем ритм",
    "🎼 Шлифуем мелодию",
  ],
  uploading: ["Сводим дорожки", "Наводим финальный блеск", "Готовим версии к загрузке"],
};

function phaseMessage(phase: string | undefined, tracksReady: number): string {
  if (phase === "generating" && tracksReady > 0) {
    return `🎼 Готово ${tracksReady} из ${TRACK_COUNT} версий…`;
  }
  if (phase === "uploading" && tracksReady > 0) {
    return `☁️ Сохраняем версию ${tracksReady} из ${TRACK_COUNT}…`;
  }
  if (phase && PHASE_LABELS[phase]) return PHASE_LABELS[phase];
  return PHASE_LABELS.generating;
}

function fmtRemaining(sec: number): string {
  if (sec <= 0) return "почти готово…";
  if (sec > 90) return `осталось ~${Math.ceil(sec / 60)} мин`;
  return "меньше минуты…";
}

function estimateFromTime(elapsedSec: number): number {
  const tau = ESTIMATE_SEC / 2.8;
  return Math.min(FALLBACK_CAP, (1 - Math.exp(-elapsedSec / tau)) * 100);
}

function estimateEta(progress: number, elapsedSec: number): string {
  if (progress >= 95) return "почти готово…";
  if (progress > 10 && elapsedSec > 5) {
    const totalEst = elapsedSec / (progress / 100);
    return fmtRemaining(totalEst - elapsedSec);
  }
  return fmtRemaining(ESTIMATE_SEC - elapsedSec);
}

/* Студийный эквалайзер — «живые» бары, ощущение работающей студии. */
function StudioEqualizer({ active }: { active: boolean }) {
  return (
    <div style={{ display: "flex", alignItems: "flex-end", gap: "4px", height: "26px" }}>
      {Array.from({ length: 9 }, (_, i) => (
        <span
          key={i}
          className={active ? "wave-animate" : undefined}
          style={{
            width: "4px",
            height: `${40 + ((i * 47) % 60)}%`,
            borderRadius: "3px",
            transformOrigin: "bottom",
            background: `linear-gradient(${ACCENT}, ${ACCENT2})`,
            boxShadow: active ? "0 0 8px rgba(0,229,192,0.35)" : "none",
            opacity: active ? 1 : 0.35,
            animationDelay: `${i * 0.09}s`,
            animationDuration: `${0.7 + (i % 4) * 0.12}s`,
          }}
        />
      ))}
    </div>
  );
}

/* Степпер этапов конвейера. */
function StageStepper({ current }: { current: number }) {
  return (
    <div style={{ display: "flex", alignItems: "flex-start", gap: "2px", marginBottom: "16px" }}>
      {STAGES.map((s, i) => {
        const done = i < current;
        const active = i === current;
        const color = done ? ACCENT : active ? "#fff" : TEXT3;
        return (
          <div key={s.label} style={{ flex: 1, display: "flex", flexDirection: "column", alignItems: "center", gap: "5px", minWidth: 0 }}>
            <div
              className={active ? "progress-pulse" : undefined}
              style={{
                width: "30px", height: "30px", borderRadius: "50%",
                display: "flex", alignItems: "center", justifyContent: "center", fontSize: "14px",
                background: done
                  ? `linear-gradient(135deg, ${ACCENT}, ${ACCENT2})`
                  : active
                    ? "rgba(0,229,192,0.12)"
                    : "rgba(255,255,255,0.04)",
                border: `1px solid ${done ? "transparent" : active ? "rgba(0,229,192,0.4)" : "rgba(255,255,255,0.08)"}`,
                transition: "all 0.3s",
              }}
            >
              {done ? (
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#04130f" strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M20 6 9 17l-5-5" />
                </svg>
              ) : (
                <span style={{ opacity: active ? 1 : 0.5 }}>{s.icon}</span>
              )}
            </div>
            <span style={{ fontSize: "9.5px", fontWeight: active ? 700 : 500, color, letterSpacing: "0.02em", textAlign: "center" }}>
              {s.label}
            </span>
          </div>
        );
      })}
    </div>
  );
}

/**
 * Прогресс генерации в фирменном стиле: студийный эквалайзер, степпер этапов и
 * «живые» подсказки. Бар опирается на generation_progress с сервера, но ползёт к
 * высокому потолку, не замирая на плато Suno.
 */
export function GenerationProgress({
  status,
  paidAt,
  generationPhase,
  generationProgress = 0,
  tracksReady = 0,
}: {
  status: GenerationStatus;
  paidAt?: string;
  generationPhase?: string;
  generationProgress?: number;
  tracksReady?: number;
}) {
  const [now, setNow] = useState(() => Date.now());
  const [displayPct, setDisplayPct] = useState(0);
  const [flavorTick, setFlavorTick] = useState(0);

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  // Ротация «живых» подсказок.
  useEffect(() => {
    const t = setInterval(() => setFlavorTick((n) => n + 1), 2800);
    return () => clearInterval(t);
  }, []);

  const fallbackStart = useRef(Date.now());
  const startMs = paidAt ? new Date(paidAt).getTime() : fallbackStart.current;
  const elapsedSec = Math.max(0, (now - startMs) / 1000);

  const completed = status === "completed";
  const serverPct = completed ? 100 : Math.min(99, Math.max(0, generationProgress));
  const timeFallback = estimateFromTime(elapsedSec);
  const targetPct = completed
    ? 100
    : serverPct > 0
      ? serverPct
      : status === "queued"
        ? Math.max(3, Math.min(FALLBACK_CAP, timeFallback))
        : Math.min(FALLBACK_CAP, timeFallback);

  // Серверный прогресс подтягивает отображение мгновенно (скачок вверх).
  useEffect(() => {
    setDisplayPct((prev) => {
      if (completed) return 100;
      if (targetPct > prev) return targetPct;
      return prev;
    });
  }, [targetPct, completed]);

  // Непрерывный «крип»: бар всегда медленно ползёт к CREEP_CAP, поэтому процесс
  // не выглядит застывшим, даже когда сервер долго держит одно значение.
  useEffect(() => {
    if (completed) return;
    const id = setInterval(() => {
      setDisplayPct((prev) => (prev >= CREEP_CAP ? prev : Math.min(CREEP_CAP, prev + 0.25)));
    }, 1000);
    return () => clearInterval(id);
  }, [completed]);

  const pct = completed ? 100 : displayPct;
  const message = completed ? PHASE_LABELS.completed : phaseMessage(generationPhase, tracksReady);
  const eta = completed ? "Готово" : estimateEta(serverPct > 0 ? serverPct : pct, elapsedSec);

  const stage = completed ? 4 : (generationPhase ? PHASE_STAGE[generationPhase] ?? 2 : 0);
  const flavorPool = (!completed && generationPhase && FLAVOR[generationPhase]) || [];
  const flavor = flavorPool.length > 0 ? flavorPool[flavorTick % flavorPool.length] : "";

  return (
    <div
      style={{
        position: "relative",
        marginTop: "4px",
        borderRadius: "18px",
        overflow: "hidden",
        background: "linear-gradient(160deg, #101513 0%, #0c0c0c 60%)",
        border: `1px solid ${completed ? "rgba(0,229,192,0.3)" : "rgba(255,255,255,0.08)"}`,
        padding: "18px 20px",
      }}
    >
      {/* Ambient glow */}
      <div
        style={{
          position: "absolute", inset: 0, pointerEvents: "none",
          background: "radial-gradient(ellipse 70% 50% at 50% -10%, rgba(0,229,192,0.09) 0%, transparent 70%)",
        }}
      />
      <div style={{ position: "relative" }}>
        {/* header: эквалайзер + проценты */}
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "16px" }}>
          <StudioEqualizer active={!completed} />
          <span style={{ fontSize: "22px", fontWeight: 800, color: ACCENT, letterSpacing: "-0.02em", fontVariantNumeric: "tabular-nums" }}>
            {Math.round(pct)}%
          </span>
        </div>

        {/* степпер этапов */}
        <StageStepper current={stage} />

        {/* основное сообщение фазы */}
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: "12px", marginBottom: "10px" }}>
          <span
            key={message}
            className="fade-in"
            style={{ fontSize: "14px", color: "#fff", fontWeight: 600, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
          >
            {message}
          </span>
          <span style={{ flexShrink: 0, fontSize: "11px", color: TEXT3 }}>{eta}</span>
        </div>

        {/* прогресс-бар */}
        <div style={{ position: "relative", width: "100%", height: 8, borderRadius: 999, background: "rgba(255,255,255,0.07)", overflow: "hidden" }}>
          <div
            className={completed ? "" : "bar-flow"}
            style={{
              height: "100%", width: `${pct}%`, borderRadius: 999,
              background: completed ? ACCENT : undefined,
              transition: "width 0.9s cubic-bezier(0.22,1,0.36,1)",
            }}
          />
        </div>

        {/* живая подсказка «что в студии» */}
        <div style={{ marginTop: "10px", minHeight: "16px", textAlign: "center", fontSize: "12px", color: TEXT2 }}>
          {completed ? (
            "Спасибо за ожидание!"
          ) : flavor ? (
            <span key={flavor} className="fade-in">{flavor}…</span>
          ) : (
            "обычно 4 версии готовы за несколько минут"
          )}
        </div>
      </div>
    </div>
  );
}
