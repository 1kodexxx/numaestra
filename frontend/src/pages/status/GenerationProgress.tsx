import type { GenerationStatus } from "@entities/order";
import { useEffect, useRef, useState } from "react";

const ACCENT = "#00e5c0";
const TEXT2 = "rgba(255,255,255,0.5)";
const TEXT3 = "rgba(255,255,255,0.28)";

const TRACK_COUNT = 4;
const ESTIMATE_SEC = 10 * 60;
const FALLBACK_CAP = 94;

const PHASE_LABELS: Record<string, string> = {
  queued: "Заказ принят — становимся в очередь…",
  preparing: "Готовим заказ к генерации…",
  lyrics: "✍️ Пишем текст песни под ваш повод…",
  submitting: "Отправляем задание в AI-студию…",
  generating: "🎼 Создаём музыку…",
  uploading: "☁️ Сохраняем готовые версии…",
  completed: "Песня готова 🎉",
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

/**
 * Прогресс генерации: опирается на generation_progress с сервера (обновляется
 * воркером по реальным этапам: очередь → текст → Suno → загрузка). Между
 * опросами статуса плавно подтягивает отображение, не обгоняя сервер.
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

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  const fallbackStart = useRef(Date.now());
  const startMs = paidAt ? new Date(paidAt).getTime() : fallbackStart.current;
  const elapsedSec = Math.max(0, (now - startMs) / 1000);

  const completed = status === "completed";
  const serverPct = completed
    ? 100
    : Math.min(99, Math.max(0, generationProgress));
  const timeFallback = estimateFromTime(elapsedSec);
  const targetPct = completed
    ? 100
    : serverPct > 0
      ? serverPct
      : status === "queued"
        ? Math.max(3, Math.min(FALLBACK_CAP, timeFallback))
        : Math.min(FALLBACK_CAP, timeFallback);

  useEffect(() => {
    setDisplayPct((prev) => {
      if (completed) return 100;
      if (targetPct > prev) return targetPct;
      return prev;
    });
  }, [targetPct, completed]);

  useEffect(() => {
    if (completed) return;
    const id = setInterval(() => {
      setDisplayPct((prev) => {
        const creepCap =
          serverPct > 0
            ? Math.min(99, serverPct + 3)
            : Math.min(FALLBACK_CAP, targetPct + 2);
        if (prev >= creepCap) return prev;
        return Math.min(creepCap, prev + 0.25);
      });
    }, 1000);
    return () => clearInterval(id);
  }, [completed, serverPct, targetPct]);

  const pct = completed ? 100 : displayPct;
  const message = completed
    ? PHASE_LABELS.completed
    : phaseMessage(generationPhase, tracksReady);
  const eta = completed
    ? "Готово"
    : estimateEta(serverPct > 0 ? serverPct : pct, elapsedSec);
  const detail =
    tracksReady > 0 && !completed
      ? `${tracksReady} из ${TRACK_COUNT} версий`
      : "обычно 4 версии готовы за несколько минут";

  return (
    <div style={{ marginTop: "4px" }}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: "12px",
          marginBottom: "10px",
        }}
      >
        <span
          key={message}
          className="fade-in"
          style={{
            fontSize: "13px",
            color: TEXT2,
            fontWeight: 500,
            minWidth: 0,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {message}
        </span>
        <span
          style={{
            flexShrink: 0,
            fontSize: "12px",
            color: ACCENT,
            fontWeight: 700,
            fontVariantNumeric: "tabular-nums",
          }}
        >
          {Math.round(pct)}%
        </span>
      </div>

      <div
        style={{
          position: "relative",
          width: "100%",
          height: 8,
          borderRadius: 999,
          background: "rgba(255,255,255,0.07)",
          overflow: "hidden",
        }}
      >
        <div
          className={completed ? "" : "bar-flow"}
          style={{
            height: "100%",
            width: `${pct}%`,
            borderRadius: 999,
            background: completed ? ACCENT : undefined,
            transition: "width 0.9s cubic-bezier(0.22,1,0.36,1)",
          }}
        />
      </div>

      <div
        style={{
          marginTop: "8px",
          fontSize: "11px",
          color: TEXT3,
          textAlign: "center",
        }}
      >
        {completed ? "Спасибо за ожидание!" : `${eta} · ${detail}`}
      </div>
    </div>
  );
}
