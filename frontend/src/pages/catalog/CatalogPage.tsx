import type { Category } from "@entities/category";
import { exampleApi } from "@entities/example";
import { genreApi } from "@entities/genre";
import { promoApi } from "@entities/admin-promo";
import { useCreateOrder } from "@features/create-order";
import { useCatalog } from "@features/load-catalog";
import type { ExampleSong } from "@shared/data/examples";
import { EXAMPLE_SONGS } from "@shared/data/examples";
import { categoryCover } from "@shared/lib/categoryCover";
import {
  getDemoTrackSync,
  hashStr,
  prewarmDemoTrack,
} from "@shared/lib/demoAudio";
import { useSeo } from "@shared/lib/seo";
import type { GenreOption } from "@shared/lib/sunoPrompt";
import { usePublicConfig } from "@shared/lib/usePublicConfig";
import { theme } from "@shared/lib/theme";
import { Button, TextField, useRipple, LyricsGuardNote } from "@shared/ui";
import { ContactModal } from "@widgets/contact-modal";
import { FloatingPlayer } from "@widgets/floating-player";
import { Footer } from "@widgets/footer";
import { ReviewsSnippet } from "@widgets/reviews-snippet";
import { stockImage } from "@widgets/side-panel";
import { useFocusTrap } from "@shared/lib/useFocusTrap";
import { useBreakpoint } from "@shared/lib/useBreakpoint";
import { GOALS, reachGoal } from "@shared/lib/analytics";
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { clearCatalogDraft, loadCatalogDraft, saveCatalogDraft } from "./catalogDraft";
import {
  EMPTY_FORM,
  GENRES_FALLBACK,
  MOODS,
  TEMPOS,
  VOCALS,
  composeBrief,
  type PromptForm,
} from "./types";

/* ─── tokens ─── */
const ACCENT = theme.accent;
const BORDER = theme.border;
const TEXT2 = theme.text2;
const TEXT3 = theme.text3;
const SURFACE = theme.dark;

const MAX_GENRES = 2;
const MAX_MOODS = 3;

/* ─── icon map: matches Russian and English keywords ─── */
const ICON_RULES: [string, string][] = [
  ["свадьб", "💍"],
  ["wedding", "💍"],
  ["день рожден", "🎂"],
  ["birthday", "🎂"],
  ["корпоратив", "🏢"],
  ["corporate", "🏢"],
  ["юбилей", "🥂"],
  ["anniversary", "🥂"],
  ["любов", "❤️"],
  ["love", "❤️"],
  ["детск", "🎈"],
  ["kids", "🎈"],
  ["выпускн", "🎓"],
  ["graduation", "🎓"],
  ["новый год", "🎆"],
  ["newyear", "🎆"],
  ["путешеств", "✈️"],
  ["travel", "✈️"],
  ["спорт", "🏆"],
  ["sport", "🏆"],
  ["дружб", "🤝"],
  ["friendship", "🤝"],
  ["романтик", "🌹"],
  ["romantic", "🌹"],
  ["мама", "🌸"],
  ["маме", "🌸"],
  ["папа", "👔"],
  ["папе", "👔"],
  ["новорожден", "👶"],
  ["baby", "👶"],
  ["развод", "💔"],
  ["breakup", "💔"],
  ["мотивац", "🔥"],
  ["roast", "🔥"],
  ["валентин", "💝"],
  ["valentine", "💝"],
  ["8 март", "🌷"],
  ["march8", "🌷"],
  ["женск", "🌷"],
  ["повышен", "📈"],
  ["promotion", "📈"],
  ["карьер", "📈"],
];

const FALLBACK_ICONS = [
  "🎵",
  "🎸",
  "🎹",
  "🎺",
  "🥁",
  "🎷",
  "🎻",
  "🪗",
  "🎤",
  "🪘",
  "🎙️",
  "🎚️",
];

function getIcon(cat: Category, idx: number): string {
  const haystack = `${cat.id} ${cat.title}`.toLowerCase();
  for (const [key, icon] of ICON_RULES) {
    if (haystack.includes(key)) return icon;
  }
  return FALLBACK_ICONS[idx % FALLBACK_ICONS.length];
}

/* ─── animated equalizer (decorative) ─── */
function Equalizer() {
  const bars = [14, 22, 32, 24, 38, 28, 18, 30, 20, 12];
  return (
    <div
      aria-hidden
      style={{
        display: "flex",
        alignItems: "flex-end",
        justifyContent: "center",
        gap: "4px",
        height: "40px",
        marginBottom: "20px",
      }}
    >
      {bars.map((h, i) => (
        <span
          key={i}
          className="wave-animate"
          style={{
            width: "4px",
            height: `${h}px`,
            borderRadius: "2px",
            background:
              "linear-gradient(180deg, #00e5c0, rgba(0,191,165,0.35))",
            transformOrigin: "bottom",
            animationDelay: `${i * 0.09}s`,
            animationDuration: `${1 + (i % 3) * 0.25}s`,
          }}
        />
      ))}
    </div>
  );
}

/* ─── hero ─── */
function Hero({
  compact,
  priceLabel,
}: {
  compact: boolean;
  priceLabel: string;
}) {
  return (
    <div
      style={{
        position: "relative",
        textAlign: "center",
        maxWidth: "600px",
        margin: "0 auto",
      }}
    >
      {/* floating ambient orbs */}
      <div
        aria-hidden
        style={{
          position: "absolute",
          inset: "-40px 0 0",
          pointerEvents: "none",
          overflow: "visible",
        }}
      >
        <div
          className="float-y"
          style={{
            position: "absolute",
            top: "-10px",
            left: "8%",
            width: 160,
            height: 160,
            borderRadius: "50%",
            background:
              "radial-gradient(circle, rgba(0,229,192,0.18), transparent 70%)",
            filter: "blur(20px)",
          }}
        />
        <div
          className="float-y"
          style={{
            position: "absolute",
            top: "20px",
            right: "6%",
            width: 130,
            height: 130,
            borderRadius: "50%",
            background:
              "radial-gradient(circle, rgba(45,226,182,0.14), transparent 70%)",
            filter: "blur(18px)",
            animationDelay: "2.5s",
          }}
        />
      </div>

      <div style={{ position: "relative" }}>
        {!compact && <Equalizer />}

        <div
          className="hero-enter"
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: "7px",
            background: "rgba(0,229,192,0.08)",
            border: "1px solid rgba(0,229,192,0.18)",
            borderRadius: "20px",
            padding: "5px 13px 5px 11px",
            marginBottom: compact ? "14px" : "18px",
          }}
        >
          <span style={{ fontSize: "12px" }}>🎙️</span>
          <span
            style={{
              fontSize: "11px",
              fontWeight: 700,
              color: ACCENT,
              letterSpacing: "0.06em",
              textTransform: "uppercase",
            }}
          >
            Подарок, который невозможно купить в магазине
          </span>
        </div>

        <h1
          className="hero-enter-d1"
          style={{
            fontSize: "clamp(26px, 5.4vw, 44px)",
            fontWeight: 800,
            letterSpacing: "-0.038em",
            lineHeight: 1.08,
            marginBottom: "14px",
          }}
        >
          Песня, написанная <span className="gradient-text">лично для вас</span>
        </h1>

        <p
          className="hero-enter-d2"
          style={{
            fontSize: "clamp(14px, 1.6vw, 16px)",
            color: TEXT2,
            lineHeight: 1.6,
            maxWidth: "480px",
            margin: "0 auto",
          }}
        >
          Подарок на свадьбу, юбилей или признание в любви, который тронет до
          слёз. Послушайте демо бесплатно — платите, только если понравилось.
        </p>

        {/* trust pills — первый акцентный про бесплатное демо */}
        <div
          className="hero-enter-d3"
          style={{
            display: "flex",
            flexWrap: "wrap",
            justifyContent: "center",
            gap: "8px",
            marginTop: compact ? "16px" : "22px",
          }}
        >
          {[
            { t: "🎧 Демо бесплатно", accent: true },
            { t: "4 версии на выбор", accent: false },
            { t: "Готово за 10 минут", accent: false },
            { t: `${priceLabel} · без подписок`, accent: false },
          ].map(({ t, accent }) => (
            <div
              key={t}
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: "6px",
                background: accent ? "rgba(0,229,192,0.12)" : "rgba(255,255,255,0.03)",
                border: `1px solid ${accent ? "rgba(0,229,192,0.4)" : BORDER}`,
                borderRadius: "20px",
                padding: "6px 13px",
                fontSize: "12px",
                fontWeight: accent ? 700 : 500,
                color: accent ? ACCENT : "rgba(255,255,255,0.6)",
              }}
            >
              {!accent && (
                <svg
                  width="12"
                  height="12"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke={ACCENT}
                  strokeWidth="3"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M20 6 9 17l-5-5" />
                </svg>
              )}
              {t}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

/* ─── search bar ─── */
function SearchBar({ onOpen }: { onOpen: () => void }) {
  return (
    <button
      onClick={onOpen}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = "rgba(0,229,192,0.35)";
        e.currentTarget.style.background = "rgba(255,255,255,0.045)";
        e.currentTarget.style.boxShadow = "0 0 0 4px rgba(0,229,192,0.05)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = "rgba(255,255,255,0.08)";
        e.currentTarget.style.background = "rgba(255,255,255,0.03)";
        e.currentTarget.style.boxShadow = "none";
      }}
      onFocus={(e) => {
        if (!e.currentTarget.matches(":focus-visible")) return;
        e.currentTarget.style.borderColor = "rgba(0,229,192,0.5)";
        e.currentTarget.style.boxShadow = "0 0 0 3px rgba(0,229,192,0.2)";
        e.currentTarget.style.outline = "none";
      }}
      onBlur={(e) => {
        e.currentTarget.style.borderColor = "rgba(255,255,255,0.08)";
        e.currentTarget.style.boxShadow = "none";
      }}
      style={{
        display: "flex",
        alignItems: "center",
        gap: "12px",
        width: "100%",
        maxWidth: "580px",
        background: "rgba(255,255,255,0.03)",
        border: "1px solid rgba(255,255,255,0.08)",
        borderRadius: "14px",
        padding: "15px 20px",
        cursor: "pointer",
        transition: "all 0.2s",
        textAlign: "left",
        outline: "none",
      }}
    >
      <svg
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="rgba(255,255,255,0.25)"
        strokeWidth="2.5"
        strokeLinecap="round"
      >
        <circle cx="11" cy="11" r="8" />
        <path d="m21 21-4.35-4.35" />
      </svg>
      <span
        style={{
          fontSize: "14px",
          color: "rgba(255,255,255,0.25)",
          fontWeight: 400,
          letterSpacing: "0.01em",
        }}
      >
        Опишите вашу песню или выберите категорию...
      </span>
    </button>
  );
}

/* ─── chip (single-select) ─── */
function Chip({
  label,
  selected,
  onClick,
}: {
  label: string;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className="chip-press"
      style={{
        padding: "9px 16px",
        borderRadius: "20px",
        fontFamily: "inherit",
        background: selected
          ? "rgba(0,229,192,0.14)"
          : "rgba(255,255,255,0.04)",
        border: `1px solid ${selected ? "rgba(0,229,192,0.5)" : "rgba(255,255,255,0.1)"}`,
        color: selected ? ACCENT : "rgba(255,255,255,0.6)",
        fontSize: "13px",
        fontWeight: selected ? 600 : 400,
        cursor: "pointer",
        transition: "all 0.15s",
      }}
      onMouseEnter={(e) => {
        if (!selected) {
          e.currentTarget.style.borderColor = "rgba(255,255,255,0.2)";
          e.currentTarget.style.color = "#fff";
        }
      }}
      onMouseLeave={(e) => {
        if (!selected) {
          e.currentTarget.style.borderColor = "rgba(255,255,255,0.1)";
          e.currentTarget.style.color = "rgba(255,255,255,0.6)";
        }
      }}
    >
      {label}
    </button>
  );
}

/* ─── section label with optional hint ─── */
function Section({
  label,
  hint,
  required,
  children,
}: {
  label: string;
  hint?: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div
        style={{
          display: "flex",
          alignItems: "baseline",
          gap: "8px",
          marginBottom: "10px",
        }}
      >
        <span style={{ fontSize: "13px", color: "rgba(255,255,255,0.7)", fontWeight: 600 }}>
          {label}
          {required && (
            <span style={{ color: "#f87171", marginLeft: "3px" }}>*</span>
          )}
        </span>
        {hint && <span style={{ fontSize: "11px", color: TEXT3 }}>{hint}</span>}
      </div>
      <div style={{ display: "flex", flexWrap: "wrap", gap: "8px" }}>
        {children}
      </div>
    </div>
  );
}

/* ─── multi-select tags with optional custom value ─── */
function MultiTagSection({
  label,
  required,
  hint,
  presets,
  selected,
  onToggle,
  allowCustom,
  maxSelect,
  customPlaceholder,
  limitLabel,
}: {
  label: string;
  required?: boolean;
  hint?: string;
  presets: string[];
  selected: string[];
  onToggle: (val: string) => void;
  allowCustom?: boolean;
  maxSelect?: number;
  customPlaceholder?: string;
  limitLabel?: string;
}) {
  const [customInput, setCustomInput] = useState("");
  const customValues = selected.filter((v) => !presets.includes(v));
  const atLimit = maxSelect != null && selected.length >= maxSelect;

  function addCustom() {
    const v = customInput.trim();
    if (!v) return;
    const exists = selected.some((x) => x.toLowerCase() === v.toLowerCase());
    if (!exists) onToggle(v);
    setCustomInput("");
  }

  const resolvedHint =
    hint ??
    (allowCustom && maxSelect
      ? `до ${maxSelect} — выбирайте из списка или добавьте свой`
      : "можно несколько");

  return (
    <Section label={label} required={required} hint={resolvedHint}>
      {presets.map((p) => (
        <Chip
          key={p}
          label={p}
          selected={selected.includes(p)}
          onClick={() => onToggle(p)}
        />
      ))}
      {customValues.map((v) => (
        <Chip
          key={`custom:${v}`}
          label={`${v} ✕`}
          selected
          onClick={() => onToggle(v)}
        />
      ))}
      {allowCustom && (
        <div
          style={{
            display: "flex",
            gap: "6px",
            width: "100%",
            marginTop: "4px",
          }}
        >
          <input
            value={customInput}
            onChange={(e) => setCustomInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                addCustom();
              }
            }}
            disabled={atLimit}
            placeholder={
              atLimit
                ? `Достигнут лимит — ${maxSelect ?? 0} ${limitLabel ?? ""}`
                : customPlaceholder
            }
            maxLength={40}
            style={{
              flex: 1,
              minWidth: 0,
              padding: "9px 14px",
              borderRadius: "20px",
              background: "rgba(255,255,255,0.04)",
              border: "1px solid rgba(255,255,255,0.1)",
              color: "#fff",
              fontSize: "13px",
              fontFamily: "inherit",
              outline: "none",
              opacity: atLimit ? 0.5 : 1,
            }}
          />
          <Button
            size="sm"
            onClick={addCustom}
            disabled={atLimit || !customInput.trim()}
          >
            Добавить
          </Button>
        </div>
      )}
    </Section>
  );
}

function GenreCustomInput({
  atLimit,
  maxSelect,
  onAdd,
}: {
  atLimit: boolean;
  maxSelect: number;
  onAdd: (v: string) => void;
}) {
  const [customInput, setCustomInput] = useState("");

  function addCustom() {
    const v = customInput.trim();
    if (!v) return;
    onAdd(v);
    setCustomInput("");
  }

  return (
    <div
      style={{
        display: "flex",
        gap: "6px",
        width: "100%",
        marginTop: "4px",
      }}
    >
      <input
        value={customInput}
        onChange={(e) => setCustomInput(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            addCustom();
          }
        }}
        disabled={atLimit}
        placeholder={
          atLimit
            ? `Достигнут лимит — ${maxSelect} жанра`
            : "Свой жанр…"
        }
        maxLength={40}
        style={{
          flex: 1,
          minWidth: 0,
          padding: "9px 14px",
          borderRadius: "20px",
          background: "rgba(255,255,255,0.04)",
          border: "1px solid rgba(255,255,255,0.1)",
          color: "#fff",
          fontSize: "13px",
          fontFamily: "inherit",
          outline: "none",
          opacity: atLimit ? 0.5 : 1,
        }}
      />
      <Button
        size="sm"
        onClick={addCustom}
        disabled={atLimit || !customInput.trim()}
      >
        Добавить
      </Button>
    </div>
  );
}

/* ─── prompt constructor ─── */
function PromptBuilder({
  form,
  update,
  genres,
  onBack,
  onSubmit,
  canSubmit,
  promoCode,
  promoStatus,
  promoError,
  promoLoading,
  onPromoChange,
  onApplyPromo,
  onClearPromo,
}: {
  form: PromptForm;
  update: <K extends keyof PromptForm>(key: K, value: PromptForm[K]) => void;
  genres: GenreOption[];
  onBack: () => void;
  onSubmit: () => void;
  canSubmit: boolean;
  promoCode: string;
  promoStatus: { discount_kopecks: number; label: string } | null;
  promoError: string | null;
  promoLoading: boolean;
  onPromoChange: (v: string) => void;
  onApplyPromo: () => void;
  onClearPromo: () => void;
}) {
  const toggleMulti = (
    key: "moods",
    val: string,
    max?: number,
  ) => {
    const arr = form[key];
    if (arr.includes(val)) {
      update(
        key,
        arr.filter((x) => x !== val),
      );
    } else if (max == null || arr.length < max) {
      update(key, [...arr, val]);
    }
  };
  const preview = composeBrief(form, genres);
  const labelToSuno = new Map(genres.map((g) => [g.label, g.sunoValue]));
  const genrePresets = genres.map((g) => g.label);
  const genreCount = form.genres.length + form.customGenres.length;

  function togglePresetGenre(label: string) {
    const suno = labelToSuno.get(label);
    if (!suno) return;
    if (form.genres.includes(suno)) {
      update(
        "genres",
        form.genres.filter((x) => x !== suno),
      );
    } else if (genreCount < MAX_GENRES) {
      update("genres", [...form.genres, suno]);
    }
  }

  function toggleCustomGenre(val: string) {
    if (form.customGenres.includes(val)) {
      update(
        "customGenres",
        form.customGenres.filter((x) => x !== val),
      );
    } else if (genreCount < MAX_GENRES) {
      update("customGenres", [...form.customGenres, val]);
    }
  }

  // Прогресс по 5 обязательным полям.
  const progressDone =
    (form.occasion.trim() ? 1 : 0) +
    ((form.genres.length > 0 || form.customGenres.length > 0) ? 1 : 0) +
    (form.moods.length > 0 ? 1 : 0) +
    (form.vocal.trim() ? 1 : 0) +
    (form.details.trim() ? 1 : 0);
  const progressTotal = 5;

  return (
    <div style={{ width: "100%", maxWidth: "640px" }} className="fade-in">
      {/* Кнопка назад — единообразна с CategoryPage */}
      <button
        onClick={onBack}
        style={{
          background: "none",
          border: "none",
          color: TEXT2,
          fontSize: "13px",
          cursor: "pointer",
          padding: 0,
          marginBottom: "18px",
          display: "flex",
          alignItems: "center",
          gap: "6px",
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.color = "#fff";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.color = TEXT2;
        }}
      >
        ← Назад к категориям
      </button>

      {/* Шапка — icon-box + бейдж + заголовок, как в CategoryPage */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "14px",
          marginBottom: "8px",
        }}
      >
        <span
          style={{
            flexShrink: 0,
            width: 48,
            height: 48,
            borderRadius: "14px",
            background: "linear-gradient(135deg, #00e5c0, #00bfa5)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: "24px",
            boxShadow: "0 6px 18px rgba(0,229,192,0.28)",
          }}
        >
          🎛️
        </span>
        <div>
          <div
            style={{
              fontSize: "11px",
              fontWeight: 700,
              color: ACCENT,
              letterSpacing: "0.06em",
              textTransform: "uppercase",
            }}
          >
            Конструктор промпта для ИИ
          </div>
          <h1
            style={{
              fontSize: "22px",
              fontWeight: 800,
              letterSpacing: "-0.03em",
              lineHeight: 1.15,
            }}
          >
            Соберите свою песню
          </h1>
        </div>
      </div>

      <p
        style={{
          fontSize: "14px",
          color: TEXT2,
          lineHeight: 1.55,
          marginBottom: "20px",
        }}
      >
        Выберите стиль, опишите детали и при желании впишите свой текст — мы
        превратим это в готовый трек.
      </p>

      {/* Прогресс обязательных полей — как в CategoryPage */}
      <div style={{ marginBottom: "20px" }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            fontSize: "12px",
            color: TEXT2,
            marginBottom: "6px",
          }}
        >
          <span>Заполнено обязательных полей</span>
          <span>
            {progressDone} / {progressTotal}
          </span>
        </div>
        <div
          style={{
            height: "4px",
            borderRadius: "2px",
            background: "rgba(255,255,255,0.08)",
            overflow: "hidden",
          }}
        >
          <div
            style={{
              height: "100%",
              width: `${(progressDone / progressTotal) * 100}%`,
              background: `linear-gradient(90deg, ${ACCENT}, #00bfa5)`,
              transition: "width 0.25s ease",
            }}
          />
        </div>
      </div>

      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "24px",
          textAlign: "left",
        }}
      >
        <TextField
          label="Для кого и по какому поводу"
          required
          value={form.occasion}
          onChange={(v) => update("occasion", v)}
          placeholder="Например: жене на годовщину свадьбы"
          surfaceColor={SURFACE}
        />

        <Section label="Жанр" required hint={`1–2 жанра — так звук чище`}>
          {genrePresets.map((label) => {
            const suno = labelToSuno.get(label);
            const selected = suno ? form.genres.includes(suno) : false;
            return (
              <Chip
                key={label}
                label={label}
                selected={selected}
                onClick={() => togglePresetGenre(label)}
              />
            );
          })}
          {form.customGenres.map((v) => (
            <Chip
              key={`custom:${v}`}
              label={`${v} ✕`}
              selected
              onClick={() => toggleCustomGenre(v)}
            />
          ))}
          <GenreCustomInput
            atLimit={genreCount >= MAX_GENRES}
            maxSelect={MAX_GENRES}
            onAdd={(v) => {
              if (
                genreCount < MAX_GENRES &&
                !form.customGenres.some((x) => x.toLowerCase() === v.toLowerCase())
              ) {
                update("customGenres", [...form.customGenres, v]);
              }
            }}
          />
        </Section>

        <MultiTagSection
          label="Настроение"
          required
          presets={[...MOODS]}
          selected={form.moods}
          onToggle={(v) => toggleMulti("moods", v, MAX_MOODS)}
          allowCustom
          maxSelect={MAX_MOODS}
          customPlaceholder="Своё настроение…"
          limitLabel="настроения"
        />

        <Section label="Темп">
          {TEMPOS.map((t) => (
            <Chip
              key={t}
              label={t}
              selected={form.tempo === t}
              onClick={() => update("tempo", form.tempo === t ? "" : t)}
            />
          ))}
        </Section>

        <Section label="Вокал" required>
          {VOCALS.map((v) => (
            <Chip
              key={v}
              label={v}
              selected={form.vocal === v}
              onClick={() => update("vocal", form.vocal === v ? "" : v)}
            />
          ))}
        </Section>

        <TextField
          label="Детали"
          required
          value={form.details}
          onChange={(v) => update("details", v)}
          multiline
          rows={4}
          placeholder="Имена, ваша история, важные слова и моменты, которые обязательно упомянуть..."
          surfaceColor={SURFACE}
          supportingText="Чем больше деталей — тем точнее получится песня."
        />

        <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
          <TextField
            label="Свой текст песни (по желанию)"
            value={form.customText}
            onChange={(v) => update("customText", v)}
            multiline
            rows={4}
            placeholder="Впишите строки или припев, которые должны прозвучать дословно..."
            surfaceColor={SURFACE}
          />
          <LyricsGuardNote text={`${form.customText} ${form.details}`} />
        </div>

        {/* Live preview — идентичен CategoryPage */}
        <div
          style={{
            background: "rgba(0,229,192,0.04)",
            border: "1px solid rgba(0,229,192,0.18)",
            borderRadius: "14px",
            padding: "16px 18px",
          }}
        >
          <div
            style={{
              fontSize: "10px",
              fontWeight: 700,
              color: ACCENT,
              letterSpacing: "0.07em",
              textTransform: "uppercase",
              marginBottom: "10px",
            }}
          >
            Готовый промпт для ИИ
          </div>
          <pre
            style={{
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
              fontFamily: "inherit",
              fontSize: "13px",
              lineHeight: 1.6,
              color: "rgba(255,255,255,0.82)",
              margin: 0,
            }}
          >
            {preview}
          </pre>
        </div>
      </div>

      {/* Нижняя панель — промокод + кнопка заказа */}
      <div
        style={{
          marginTop: "24px",
          paddingTop: "20px",
          borderTop: `1px solid ${BORDER}`,
          display: "flex",
          flexDirection: "column",
          gap: "10px",
        }}
      >
        {/* Промокод */}
        <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
          <div style={{ flex: 1 }}>
            <TextField
              label="Промокод (необязательно)"
              value={promoCode}
              onChange={(v) => onPromoChange(v.toUpperCase())}
              disabled={!!promoStatus}
            />
          </div>
          {promoStatus ? (
            <Button size="sm" onClick={onClearPromo}>✕</Button>
          ) : (
            <Button size="sm" onClick={onApplyPromo} disabled={!promoCode.trim() || promoLoading}>
              {promoLoading ? "…" : "Применить"}
            </Button>
          )}
        </div>
        {promoStatus && (
          <div style={{ fontSize: "12px", color: ACCENT, textAlign: "center" }}>
            ✓ Промокод применён: {promoStatus.label} — сэкономите{" "}
            {Math.round(promoStatus.discount_kopecks / 100)} ₽
          </div>
        )}
        {promoError && (
          <div style={{ fontSize: "12px", color: "#f87171", textAlign: "center" }}>
            ✕ {promoError}
          </div>
        )}

        {!canSubmit && (
          <div
            style={{
              fontSize: "12px",
              color: TEXT3,
              textAlign: "center",
            }}
          >
            Заполните обязательные поля, чтобы продолжить
          </div>
        )}
        <Button size="lg" fullWidth disabled={!canSubmit} onClick={onSubmit}>
          Создать песню — демо бесплатно →
        </Button>
        <div style={{ fontSize: "12px", color: TEXT3, textAlign: "center" }}>
          Оплата только после демо, если понравится
        </div>
      </div>
    </div>
  );
}

/* ─── category card ─── */
function CategoryCard({
  cat,
  index,
  onClick,
  priceLabel,
}: {
  cat: Category;
  index: number;
  onClick: () => void;
  priceLabel: string;
}) {
  const [h, setH] = useState(false);
  const icon = getIcon(cat, index);
  const { onPointerDown, rippleEl } = useRipple("rgba(0,229,192,0.5)");

  return (
    <button
      onClick={onClick}
      onPointerDown={onPointerDown}
      onMouseEnter={() => {
        if (window.matchMedia("(hover: hover)").matches) setH(true);
      }}
      onMouseLeave={() => setH(false)}
      className={`interactive-card chip-press catalog-tile${h ? " is-hovered" : ""}`}
      aria-label={`Категория: ${cat.title}`}
      style={{
        width: "100%",
        aspectRatio: "1 / 1",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: "12px",
        padding: "18px 14px",
        background: h ? "rgba(0,229,192,0.06)" : "rgba(255,255,255,0.02)",
        border: `1.5px solid ${h ? "rgba(0,229,192,0.5)" : "rgba(255,255,255,0.1)"}`,
        borderRadius: "22px",
        cursor: "pointer",
        transform: h ? "translateY(-4px)" : "translateY(0)",
        boxShadow: h ? "0 16px 36px rgba(0,229,192,0.12)" : "none",
        transition: "all 0.24s cubic-bezier(0.34, 1.4, 0.64, 1)",
        overflow: "hidden",
        position: "relative",
      }}
    >
      {/* icon badge */}
      <div
        style={{
          width: "52px",
          height: "52px",
          borderRadius: "50%",
          background: h ? "rgba(0,229,192,0.14)" : "rgba(255,255,255,0.05)",
          border: `1px solid ${h ? "rgba(0,229,192,0.4)" : "rgba(255,255,255,0.12)"}`,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          transition: "all 0.24s",
          position: "relative",
          transform: h ? "scale(1.08)" : "scale(1)",
        }}
      >
        <span
          style={{
            fontSize: "26px",
            lineHeight: 1,
            filter: h ? "none" : "grayscale(0.15)",
          }}
        >
          {icon}
        </span>
      </div>

      <div
        style={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          gap: "7px",
          position: "relative",
        }}
      >
        <span
          style={{
            fontSize: "13px",
            fontWeight: 700,
            color: h ? "#00e5c0" : "rgba(255,255,255,0.85)",
            lineHeight: 1.25,
            letterSpacing: "-0.01em",
            textAlign: "center",
            transition: "color 0.2s",
          }}
        >
          {cat.title}
        </span>

        <span
          style={{
            fontSize: "11px",
            fontWeight: 700,
            color: h ? ACCENT : TEXT2,
            background: h ? "rgba(0,229,192,0.12)" : "rgba(255,255,255,0.05)",
            border: `1px solid ${h ? "rgba(0,229,192,0.32)" : "rgba(255,255,255,0.08)"}`,
            borderRadius: "999px",
            padding: "2px 9px",
            letterSpacing: "0.01em",
            transition: "all 0.2s",
          }}
        >
          {priceLabel}
        </span>
      </div>
      {rippleEl}
    </button>
  );
}

/* ─── skeleton card ─── */
function SkeletonCard() {
  return (
    <div
      className="skeleton"
      style={{ aspectRatio: "1 / 1", borderRadius: "22px" }}
    />
  );
}

/* ─── section: переносящаяся сетка (ничего не обрезается по ширине) ─── */
function HSection({
  icon,
  title,
  sub,
  minCol = 160,
  children,
}: {
  icon: string;
  title: string;
  sub?: string;
  minCol?: number;
  children: React.ReactNode;
}) {
  return (
    <section style={{ marginTop: "8px" }}>
      <div
        style={{
          display: "flex",
          alignItems: "baseline",
          gap: "10px",
          marginBottom: "14px",
        }}
      >
        <span style={{ fontSize: "18px", lineHeight: 1 }}>{icon}</span>
        <h2
          style={{
            fontSize: "18px",
            fontWeight: 800,
            letterSpacing: "-0.02em",
            color: "#fff",
          }}
        >
          {title}
        </h2>
        {sub && <span style={{ fontSize: "12px", color: TEXT3 }}>{sub}</span>}
      </div>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: `repeat(auto-fill, minmax(${minCol}px, 1fr))`,
          gap: "12px",
        }}
      >
        {children}
      </div>
    </section>
  );
}

/* ─── popular category card (horizontal) ─── */
function PopularCard({
  cat,
  rank,
  onClick,
  priceLabel,
}: {
  cat: Category;
  rank: number;
  onClick: () => void;
  priceLabel: string;
}) {
  const [h, setH] = useState(false);
  const [imgFailed, setImgFailed] = useState(false);
  const icon = getIcon(cat, rank);
  return (
    <button
      onClick={onClick}
      onMouseEnter={() => {
        if (window.matchMedia("(hover: hover)").matches) setH(true);
      }}
      onMouseLeave={() => setH(false)}
      className={`interactive-card chip-press catalog-tile catalog-tile--horizontal${h ? " is-hovered" : ""}`}
      style={{
        width: "100%",
        display: "flex",
        flexDirection: "column",
        gap: "10px",
        background: h ? "rgba(0,229,192,0.06)" : "rgba(255,255,255,0.025)",
        border: `1px solid ${h ? "rgba(0,229,192,0.3)" : "rgba(255,255,255,0.07)"}`,
        borderRadius: "16px",
        padding: "10px",
        cursor: "pointer",
        textAlign: "left",
        transform: h ? "translateY(-3px)" : "translateY(0)",
        boxShadow: h ? "0 12px 28px rgba(0,0,0,0.3)" : "none",
        transition: "all 0.2s cubic-bezier(0.34,1.4,0.64,1)",
      }}
    >
      <div
        style={{
          position: "relative",
          width: "100%",
          height: 96,
          borderRadius: "11px",
          overflow: "hidden",
          background:
            "linear-gradient(135deg, rgba(0,229,192,0.18), rgba(0,191,165,0.05))",
        }}
      >
        {imgFailed ? (
          <div style={{
            width: "100%", height: "100%", display: "flex",
            alignItems: "center", justifyContent: "center", fontSize: "36px",
          }}>
            {icon}
          </div>
        ) : (
          <img
            src={
              categoryCover(cat.id, cat.cover_image_url) ||
              stockImage(cat.id, "celebration,party")
            }
            alt={`Обложка категории ${cat.title}`}
            loading="lazy"
            onError={() => setImgFailed(true)}
            style={{
              width: "100%",
              height: "100%",
              objectFit: "cover",
              display: "block",
            }}
          />
        )}
        <span
          style={{
            position: "absolute",
            left: 6,
            bottom: 6,
            minWidth: 20,
            height: 20,
            padding: "0 5px",
            borderRadius: "7px",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: "12px",
            fontWeight: 800,
            color: "#062420",
            background: "linear-gradient(135deg, #00e5c0, #00bfa5)",
            boxShadow: "0 2px 6px rgba(0,0,0,0.45)",
          }}
        >
          {rank}
        </span>
      </div>
      <span
        style={{
          display: "-webkit-box",
          WebkitLineClamp: 2,
          WebkitBoxOrient: "vertical",
          overflow: "hidden",
          fontSize: "13px",
          fontWeight: 700,
          lineHeight: 1.25,
          color: h ? ACCENT : "#fff",
          transition: "color 0.18s",
        }}
      >
        {cat.title}
      </span>

      <span
        style={{
          alignSelf: "flex-start",
          fontSize: "11px",
          fontWeight: 700,
          color: h ? ACCENT : TEXT2,
          background: h ? "rgba(0,229,192,0.12)" : "rgba(255,255,255,0.05)",
          border: `1px solid ${h ? "rgba(0,229,192,0.32)" : "rgba(255,255,255,0.08)"}`,
          borderRadius: "999px",
          padding: "2px 9px",
          letterSpacing: "0.01em",
          transition: "all 0.2s",
        }}
      >
        {priceLabel}
      </span>
    </button>
  );
}

/* ─── example card (horizontal) ─── */
function ExampleCard({ ex, onPlay }: { ex: ExampleSong; onPlay: () => void }) {
  const [h, setH] = useState(false);
  const [imgFailed, setImgFailed] = useState(false);
  return (
    <button
      onClick={onPlay}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
      className="interactive-card chip-press"
      style={{
        width: "100%",
        display: "flex",
        flexDirection: "column",
        gap: "10px",
        background: h ? "rgba(0,229,192,0.06)" : "rgba(255,255,255,0.025)",
        border: `1px solid ${h ? "rgba(0,229,192,0.3)" : "rgba(255,255,255,0.07)"}`,
        borderRadius: "16px",
        padding: "10px",
        cursor: "pointer",
        textAlign: "left",
        transform: h ? "translateY(-3px)" : "translateY(0)",
        boxShadow: h ? "0 12px 28px rgba(0,0,0,0.3)" : "none",
        transition: "all 0.2s cubic-bezier(0.34,1.4,0.64,1)",
      }}
    >
      <div
        style={{
          position: "relative",
          width: "100%",
          height: 110,
          borderRadius: "11px",
          overflow: "hidden",
          background:
            "linear-gradient(135deg, rgba(0,229,192,0.18), rgba(0,191,165,0.05))",
        }}
      >
        {imgFailed ? (
          <div style={{
            width: "100%", height: "100%", display: "flex",
            alignItems: "center", justifyContent: "center", fontSize: "40px",
          }}>
            🎵
          </div>
        ) : (
          <img
            src={ex.coverUrl || stockImage(ex.id, "concert,music")}
            alt={`Обложка песни «${ex.title}»`}
            loading="lazy"
            onError={() => setImgFailed(true)}
            style={{
              width: "100%",
              height: "100%",
              objectFit: "cover",
              display: "block",
            }}
          />
        )}
        <span
          style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            background: h ? "rgba(0,0,0,0.25)" : "rgba(0,0,0,0.4)",
            transition: "background 0.2s",
          }}
        >
          <span
            style={{
              width: 40,
              height: 40,
              borderRadius: "50%",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              background: "linear-gradient(135deg, #00e5c0, #00bfa5)",
              boxShadow: "0 4px 14px rgba(0,229,192,0.5)",
              transform: h ? "scale(1.08)" : "scale(1)",
              transition: "transform 0.2s",
            }}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="#062420">
              <path d="M8 5v14l11-7z" />
            </svg>
          </span>
        </span>
      </div>
      <div>
        <div
          style={{
            fontSize: "13px",
            fontWeight: 700,
            color: "#fff",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {ex.title}
        </div>
        <div style={{ fontSize: "11px", color: TEXT2, marginTop: "2px" }}>
          {ex.category} · {ex.mood}
        </div>
      </div>
    </button>
  );
}

/* ─── main ─── */
export function CatalogPage() {
  const publicConfig = usePublicConfig();
  const { categories, loading, error, reload } = useCatalog();
  const navigate = useNavigate();
  const { isMobile, isShort } = useBreakpoint();
  const [briefOpen, setBriefOpen] = useState(false);
  const builderTrapRef = useFocusTrap(briefOpen);
  const [form, setForm] = useState<PromptForm>(() => loadCatalogDraft() ?? EMPTY_FORM);
  const [genres, setGenres] = useState<GenreOption[]>(GENRES_FALLBACK);
  const [showContact, setShowContact] = useState(false);
  const [promoCode, setPromoCode] = useState("");
  const [promoStatus, setPromoStatus] = useState<{ discount_kopecks: number; label: string } | null>(null);
  const [promoError, setPromoError] = useState<string | null>(null);
  const [promoLoading, setPromoLoading] = useState(false);
  const [playing, setPlaying] = useState<ExampleSong | null>(null);
  const [track, setTrack] = useState<{ url: string; duration: number } | null>(
    null,
  );
  const audioRef = useRef<HTMLAudioElement>(null);

  useEffect(() => {
    saveCatalogDraft(form);
  }, [form]);

  // Примеры приходят из админки
  // список как резерв, чтобы блок «Послушать примеры» не пустовал.
  const [examples, setExamples] = useState<ExampleSong[]>(EXAMPLE_SONGS);
  useEffect(() => {
    exampleApi
      .list()
      .then((items) => {
        if (items.length === 0) return; // оставляем резерв
        setExamples(
          items.map((e) => ({
            id: e.id,
            title: e.title,
            category: e.category,
            description: e.description,
            mood: e.mood,
            audioUrl: e.audio_url,
            coverUrl: e.cover_url,
          })),
        );
      })
      .catch(() => {
        /* сеть недоступна — остаёмся на резервном списке */
      });
  }, []);

  useEffect(() => {
    genreApi
      .list()
      .then((items) => {
        if (items.length === 0) return;
        setGenres(
          items.map((g) => ({ label: g.label, sunoValue: g.suno_value })),
        );
      })
      .catch(() => {
        /* остаёмся на резервном списке */
      });
  }, []);

  useEffect(() => {
    try {
      if (!localStorage.getItem("numaestra_catalog_draft_v1")) return;
      const migrated = loadCatalogDraft(genres);
      if (migrated) setForm(migrated);
    } catch {
      /* ignore */
    }
  }, [genres]);

  // Прогреваем демо-синтез только для примеров без реальной записи.
  useEffect(() => {
    examples.forEach((ex) => {
      if (!ex.audioUrl) prewarmDemoTrack(hashStr(ex.id));
    });
  }, [examples]);

  // Запуск воспроизведения СИНХРОННО в жесте тапа — иначе мобильные браузеры
  // блокируют автоплей (жест истекает после await синтеза).
  // Если у примера есть реальный файл — играем его, иначе синтезируем демо.
  function playExample(ex: ExampleSong) {
    const t = ex.audioUrl
      ? { url: ex.audioUrl, duration: 0 } // длительность подтянется из loadedmetadata
      : getDemoTrackSync(hashStr(ex.id));
    setTrack(t);
    setPlaying(ex);
    const el = audioRef.current;
    if (el && t) {
      el.src = t.url;
      el.currentTime = 0;
      el.play().catch(() => {});
    }
  }

  function stopExample() {
    const el = audioRef.current;
    if (el) {
      el.pause();
      el.removeAttribute("src");
      el.load();
    }
    setPlaying(null);
    setTrack(null);
  }

  useSeo({
    title: "Numaestra — персональная песня на заказ за 10 минут",
    description:
      "Закажите уникальную песню под ваш повод: свадьба, день рождения, корпоратив и ещё 30+ категорий. Опишите идею — получите 4 версии трека за 10 минут. Один платёж, без подписок.",
  });

  function updateForm<K extends keyof PromptForm>(
    key: K,
    value: PromptForm[K],
  ) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  // Конструктор открывается по клику на строку поиска.
  function openBuilder() {
    reachGoal(GOALS.BUILDER_OPEN);
    setBriefOpen(true);
  }

  function openContact() {
    reachGoal(GOALS.CONTACT_OPEN, { source: "catalog" });
    setShowContact(true);
  }

  function closeBuilder() {
    setBriefOpen(false);
  }

  const { loading: submitting, error: submitError, submit } = useCreateOrder();

  async function applyPromoCode() {
    const code = promoCode.trim().toUpperCase();
    if (!code) return;
    setPromoLoading(true);
    setPromoStatus(null);
    setPromoError(null);
    try {
      const res = await promoApi.validate(code, publicConfig.price_kopecks);
      const label =
        res.discount_type === "percent"
          ? `−${res.discount_value}%`
          : `−${res.discount_value} ₽`;
      setPromoStatus({ discount_kopecks: res.discount_kopecks, label });
    } catch {
      setPromoStatus(null);
      setPromoError("Промокод недействителен или истёк");
    } finally {
      setPromoLoading(false);
    }
  }

  async function handleCustomOrder(email: string, phone: string) {
    const brief = composeBrief(form, genres);
    const orderId = await submit({
      email,
      phone,
      brief,
      category_id: "",
      answers: {},
      consent_doc_version: publicConfig.consent_doc_version,
      promo_code: promoStatus ? promoCode.trim().toUpperCase() : undefined,
    });
    // Очищаем черновик только после подтверждённого создания заказа.
    // При сетевой ошибке или ошибке сервера заказ не создан — черновик остаётся.
    if (orderId) {
      clearCatalogDraft();
    }
  }

  const gridCols = isMobile
    ? "repeat(2, 1fr)"
    : "repeat(auto-fill, minmax(150px, 1fr))";
  const popular = categories.slice(0, 10);
  const filteredCategories = categories;
  const filteredPopular = popular;

  // Конструктор промпта — непрозрачный полноэкранный оверлей поверх всего.
  const constructorOverlay = briefOpen && (
    <div
      className="fade-in safe-overlay-bottom"
      style={{
        position: "fixed",
        top: 60,
        left: 0,
        right: 0,
        bottom: 0,
        zIndex: 40,
        background: "#080808",
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "center",
        overflowY: "auto",
        padding: isMobile ? "20px 16px" : "32px 24px",
      }}
    >
      <div ref={builderTrapRef} style={{ width: "100%", maxWidth: 640, margin: "auto" }}>
        <PromptBuilder
          form={form}
          update={updateForm}
          genres={genres}
          onBack={closeBuilder}
          onSubmit={openContact}
          canSubmit={
            form.occasion.trim().length > 0 &&
            form.details.trim().length > 0 &&
            (form.genres.length > 0 || form.customGenres.length > 0) &&
            form.moods.length > 0 &&
            form.vocal.trim().length > 0
          }
          promoCode={promoCode}
          promoStatus={promoStatus}
          promoError={promoError}
          promoLoading={promoLoading}
          onPromoChange={(v) => { setPromoCode(v); setPromoStatus(null); setPromoError(null); }}
          onApplyPromo={applyPromoCode}
          onClearPromo={() => { setPromoCode(""); setPromoStatus(null); setPromoError(null); }}
        />
      </div>
    </div>
  );

  return (
    <>
      {showContact && (
        <ContactModal
          loading={submitting}
          error={submitError}
          priceLabel={publicConfig.price_label}
          priceKopecks={publicConfig.price_kopecks}
          discountKopecks={promoStatus?.discount_kopecks ?? 0}
          discountLabel={promoStatus?.label}
          onClose={() => setShowContact(false)}
          onSubmit={handleCustomOrder}
        />
      )}
      {constructorOverlay}
      {/* Постоянный аудио-элемент: src/play() выставляются синхронно в жесте тапа. */}
      <audio ref={audioRef} preload="auto" />
      {playing && !briefOpen && (
        <FloatingPlayer
          example={playing}
          track={track}
          audioRef={audioRef}
          onClose={stopExample}
        />
      )}

      {/* Одноколоночный premium-лэйаут: единый скролл, центр ≤1000px */}
      <div
        style={
          {
            height: "calc(100dvh - 60px)",
            overflowY: "auto",
            WebkitOverflowScrolling: "touch",
          } as React.CSSProperties
        }
      >
        <div
          style={{
            maxWidth: 1000,
            margin: "0 auto",
            padding: isMobile ? "20px 16px 0" : "36px 32px 0",
          }}
        >
          {/* Hero */}
          <div style={{ marginBottom: isMobile ? "8px" : "4px" }}>
            <Hero
              compact={isMobile || isShort}
              priceLabel={publicConfig.price_label}
            />
          </div>

          {/* Search → конструктор */}
          <div
            style={{
              padding: isMobile ? "16px 0 6px" : "22px 0 10px",
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: "10px",
            }}
          >
            <SearchBar onOpen={openBuilder} />
            <p style={{ fontSize: "12px", color: TEXT3, fontWeight: 500 }}>
              или выберите категорию ниже
            </p>
          </div>

          {error && (
            <div
              style={{
                margin: isMobile ? "16px 0" : "22px 0",
                padding: "16px 20px",
                borderRadius: "16px",
                background: "rgba(239,68,68,0.08)",
                border: "1px solid rgba(239,68,68,0.2)",
                textAlign: "center",
              }}
            >
              <div
                style={{
                  fontSize: "14px",
                  color: "#f87171",
                  marginBottom: "12px",
                }}
              >
                Не удалось загрузить категории: {error}
              </div>
              <Button size="sm" onClick={reload}>
                Повторить
              </Button>
            </div>
          )}

          {!loading && !error && categories.length === 0 && (
            <div
              style={{
                textAlign: "center",
                padding: "32px 16px",
                color: TEXT2,
                fontSize: "14px",
              }}
            >
              Категории временно недоступны. Попробуйте обновить страницу или
              воспользуйтесь конструктором выше.
            </div>
          )}

          {/* Популярное — горизонтальная секция */}
          {!loading && !error && filteredPopular.length > 0 && (
            <div style={{ marginTop: "24px" }}>
              <HSection
                icon="🔥"
                title="Популярное"
                sub="выбор пользователей"
                minCol={150}
              >
                {filteredPopular.map((cat, i) => (
                  <PopularCard
                    key={cat.id}
                    cat={cat}
                    rank={i + 1}
                    priceLabel={publicConfig.price_label}
                    onClick={() => navigate(`/category/${cat.id}`)}
                  />
                ))}
              </HSection>
            </div>
          )}

          {/* Все категории — сетка */}
          <section style={{ marginTop: "32px" }}>
            <div
              style={{
                display: "flex",
                alignItems: "baseline",
                gap: "10px",
                marginBottom: "16px",
              }}
            >
              <span style={{ fontSize: "18px", lineHeight: 1 }}>🎵</span>
              <h2
                style={{
                  fontSize: "18px",
                  fontWeight: 800,
                  letterSpacing: "-0.02em",
                  color: "#fff",
                }}
              >
                Все категории
              </h2>
              {!loading && (
                <span style={{ fontSize: "12px", color: TEXT3 }}>
                  {categories.length}
                </span>
              )}
            </div>
            <div
              style={{
                display: "grid",
                gridTemplateColumns: gridCols,
                gap: isMobile ? "10px" : "14px",
              }}
            >
              {loading
                ? Array.from({ length: 12 }, (_, i) => <SkeletonCard key={i} />)
                : filteredCategories.map((cat, i) => (
                    <div
                      key={cat.id}
                      className="fade-up"
                      style={{
                        animationDelay: `${Math.min(i * 0.025, 0.35)}s`,
                      }}
                    >
                      <CategoryCard
                        cat={cat}
                        index={i}
                        priceLabel={publicConfig.price_label}
                        onClick={() => navigate(`/category/${cat.id}`)}
                      />
                    </div>
                  ))}
            </div>
          </section>

          {/* Примеры — горизонтальная секция с плавающим плеером */}
          {!loading && (
            <div style={{ marginTop: "36px" }}>
              <HSection
                icon="🎧"
                title="Послушать примеры"
                sub="нажмите play"
                minCol={180}
              >
                {examples.map((ex) => (
                  <ExampleCard
                    key={ex.id}
                    ex={ex}
                    onPlay={() => playExample(ex)}
                  />
                ))}
              </HSection>
            </div>
          )}

          <ReviewsSnippet />
        </div>

        <Footer />
        {/* отступ снизу, чтобы плавающий плеер не перекрывал футер */}
        {playing && !briefOpen && <div style={{ height: 76 }} />}
      </div>
    </>
  );
}
