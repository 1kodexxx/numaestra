import type { Category } from "@entities/category";
import { exampleApi } from "@entities/example";
import { genreApi } from "@entities/genre";
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
import { composeCatalogBrief } from "@shared/lib/sunoPrompt";
import { usePublicConfig } from "@shared/lib/usePublicConfig";
import { Button, TextField, useRipple } from "@shared/ui";
import { ContactModal } from "@widgets/contact-modal";
import { FloatingPlayer } from "@widgets/floating-player";
import { Footer } from "@widgets/footer";
import { stockImage } from "@widgets/side-panel";
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

/* ─── tokens ─── */
const ACCENT = "#00e5c0";
const BORDER = "rgba(255,255,255,0.07)";
const TEXT2 = "rgba(255,255,255,0.48)";
const TEXT3 = "rgba(255,255,255,0.2)";

/* ─── breakpoints ─── */
function useBreakpoint() {
  const [vp, setVp] = useState(() => ({
    w: typeof window !== "undefined" ? window.innerWidth : 1200,
    h: typeof window !== "undefined" ? window.innerHeight : 800,
  }));
  useEffect(() => {
    const fn = () => setVp({ w: window.innerWidth, h: window.innerHeight });
    window.addEventListener("resize", fn);
    return () => {
      window.removeEventListener("resize", fn);
    };
  }, []);
  return {
    isMobile: vp.w < 640,
    isTablet: vp.w >= 640 && vp.w < 1024,
    isDesktop: vp.w >= 1024,
    isShort: vp.h < 720, // низкий вьюпорт (ландшафт телефона, маленький ноутбук)
  };
}

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
            AI-студия персональных песен
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
            maxWidth: "460px",
            margin: "0 auto",
          }}
        >
          Опишите повод — и получите 4 готовые версии трека уже через 10 минут
        </p>

        {/* trust pills */}
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
            "4 версии трека",
            "Готово за 10 минут",
            `${priceLabel} · без подписок`,
          ].map((t) => (
            <div
              key={t}
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: "6px",
                background: "rgba(255,255,255,0.03)",
                border: `1px solid ${BORDER}`,
                borderRadius: "20px",
                padding: "6px 13px",
                fontSize: "12px",
                fontWeight: 500,
                color: "rgba(255,255,255,0.6)",
              }}
            >
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
      style={{
        padding: "9px 16px",
        borderRadius: "20px",
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

const MOODS = [
  "Романтика",
  "Радость",
  "Грусть",
  "Ностальгия",
  "Энергия",
  "Торжественность",
  "Юмор",
  "Спокойствие",
  "Драйв",
];
const GENRES_FALLBACK: GenreOption[] = [
  { label: "Поп", sunoValue: "modern pop" },
  { label: "Баллада", sunoValue: "pop ballad" },
  { label: "Рок", sunoValue: "rock" },
  { label: "Рэп", sunoValue: "rap" },
  { label: "Хип-хоп", sunoValue: "hip hop" },
  { label: "Джаз", sunoValue: "smooth jazz" },
  { label: "R&B", sunoValue: "contemporary rnb" },
  { label: "Электроника", sunoValue: "electronic dance" },
  { label: "Шансон", sunoValue: "russian chanson" },
  { label: "Акустика", sunoValue: "acoustic guitar" },
  { label: "Фолк", sunoValue: "folk" },
  { label: "Кантри", sunoValue: "modern country" },
];
const TEMPOS = ["Медленный", "Средний", "Быстрый"];
const VOCALS = ["Мужской", "Женский", "Дуэт", "Хор", "Без вокала"];

const SURFACE = "#080808";

interface PromptForm {
  occasion: string;
  moods: string[];
  genres: string[];
  tempo: string;
  vocal: string;
  details: string;
  customText: string;
}

const EMPTY_FORM: PromptForm = {
  occasion: "",
  moods: [],
  genres: [],
  tempo: "",
  vocal: "",
  details: "",
  customText: "",
};

/* Собираем Suno-промпт: style tags (англ.) + понятное описание для генерации. */
function composeBrief(f: PromptForm, genreOptions: GenreOption[]): string {
  return composeCatalogBrief(f, genreOptions);
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
        <span style={{ fontSize: "12px", color: TEXT2, fontWeight: 500 }}>
          {label}
          {required && (
            <span style={{ color: "#ef4444", marginLeft: "3px" }}>*</span>
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

/* ─── prompt constructor ─── */
function PromptBuilder({
  form,
  update,
  genres,
  onBack,
  onSubmit,
  canSubmit,
  priceLabel,
}: {
  form: PromptForm;
  update: <K extends keyof PromptForm>(key: K, value: PromptForm[K]) => void;
  genres: GenreOption[];
  onBack: () => void;
  onSubmit: () => void;
  canSubmit: boolean;
  priceLabel: string;
}) {
  const toggleMulti = (key: "moods" | "genres", val: string) => {
    const arr = form[key];
    update(
      key,
      arr.includes(val) ? arr.filter((x) => x !== val) : [...arr, val],
    );
  };
  const preview = composeBrief(form, genres);

  return (
    <div style={{ width: "100%", maxWidth: "640px" }} className="fade-in">
      <button
        onClick={onBack}
        style={{
          background: "none",
          border: "none",
          color: TEXT2,
          fontSize: "13px",
          cursor: "pointer",
          marginBottom: "18px",
          padding: 0,
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.color = "#fff";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.color = TEXT2;
        }}
      >
        ← К категориям
      </button>

      <div
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: "7px",
          marginBottom: "12px",
        }}
      >
        <span style={{ fontSize: "12px" }}>🎛️</span>
        <span
          style={{
            fontSize: "11px",
            fontWeight: 700,
            color: ACCENT,
            letterSpacing: "0.06em",
            textTransform: "uppercase",
          }}
        >
          Конструктор промпта для ИИ
        </span>
      </div>
      <div
        style={{
          fontSize: "22px",
          fontWeight: 800,
          letterSpacing: "-0.02em",
          marginBottom: "4px",
          textAlign: "left",
        }}
      >
        Соберите свою песню
      </div>
      <div
        style={{
          fontSize: "13px",
          color: TEXT2,
          marginBottom: "26px",
          textAlign: "left",
          lineHeight: 1.5,
        }}
      >
        Выберите стиль, опишите детали и при желании впишите свой текст — мы
        превратим это в готовый трек.
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

        <Section label="Жанр" hint="можно несколько">
          {genres.map((g) => (
            <Chip
              key={g.label}
              label={g.label}
              selected={form.genres.includes(g.label)}
              onClick={() => toggleMulti("genres", g.label)}
            />
          ))}
        </Section>

        <Section label="Настроение" hint="можно несколько">
          {MOODS.map((m) => (
            <Chip
              key={m}
              label={m}
              selected={form.moods.includes(m)}
              onClick={() => toggleMulti("moods", m)}
            />
          ))}
        </Section>

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

        <Section label="Вокал">
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

        <TextField
          label="Свой текст песни (по желанию)"
          value={form.customText}
          onChange={(v) => update("customText", v)}
          multiline
          rows={4}
          placeholder="Впишите строки или припев, которые должны прозвучать дословно..."
          surfaceColor={SURFACE}
        />

        {/* Live preview */}
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

      <div style={{ marginTop: "24px" }}>
        {!canSubmit && (
          <div
            style={{
              fontSize: "12px",
              color: TEXT3,
              marginBottom: "10px",
              textAlign: "center",
            }}
          >
            Заполните «Повод» и «Детали», чтобы продолжить
          </div>
        )}
        <Button size="lg" fullWidth disabled={!canSubmit} onClick={onSubmit}>
          Продолжить — {priceLabel} →
        </Button>
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
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
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
  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
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
        <img
          src={
            categoryCover(cat.id, cat.cover_image_url) ||
            stockImage(cat.id, "celebration,party")
          }
          alt={`Обложка категории ${cat.title}`}
          loading="lazy"
          onError={(e) => {
            e.currentTarget.style.opacity = "0";
          }}
          style={{
            width: "100%",
            height: "100%",
            objectFit: "cover",
            display: "block",
          }}
        />
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
  return (
    <button
      onClick={onPlay}
      onMouseEnter={() => setH(true)}
      onMouseLeave={() => setH(false)}
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
        <img
          src={ex.coverUrl || stockImage(ex.id, "concert,music")}
          alt={`Обложка песни «${ex.title}»`}
          loading="lazy"
          onError={(e) => {
            e.currentTarget.style.opacity = "0";
          }}
          style={{
            width: "100%",
            height: "100%",
            objectFit: "cover",
            display: "block",
          }}
        />
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
  const [form, setForm] = useState<PromptForm>(EMPTY_FORM);
  const [genres, setGenres] = useState<GenreOption[]>(GENRES_FALLBACK);
  const [showContact, setShowContact] = useState(false);
  const [playing, setPlaying] = useState<ExampleSong | null>(null);
  const [track, setTrack] = useState<{ url: string; duration: number } | null>(
    null,
  );
  const audioRef = useRef<HTMLAudioElement>(null);

  // Примеры приходят из админки (API). До загрузки и при сбое — захардкоженный
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
    setBriefOpen(true);
  }

  function closeBuilder() {
    setBriefOpen(false);
  }

  const { loading: submitting, error: submitError, submit } = useCreateOrder();

  async function handleCustomOrder(email: string, phone: string) {
    const brief = composeBrief(form, genres);
    await submit({
      email,
      phone,
      brief,
      category_id: "",
      answers: {},
      consent_doc_version: publicConfig.consent_doc_version,
    });
  }

  const gridCols = isMobile
    ? "repeat(2, 1fr)"
    : "repeat(auto-fill, minmax(150px, 1fr))";
  const popular = categories.slice(0, 10);

  // Конструктор промпта — непрозрачный полноэкранный оверлей поверх всего.
  const constructorOverlay = briefOpen && (
    <div
      className="fade-in"
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
        padding: isMobile ? "20px 16px 40px" : "32px 24px",
      }}
    >
      <div style={{ width: "100%", maxWidth: 640, margin: "auto" }}>
        <PromptBuilder
          form={form}
          update={updateForm}
          genres={genres}
          onBack={closeBuilder}
          onSubmit={() => setShowContact(true)}
          canSubmit={
            form.occasion.trim().length > 0 && form.details.trim().length > 0
          }
          priceLabel={publicConfig.price_label}
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
          {!loading && !error && popular.length > 0 && (
            <div style={{ marginTop: "24px" }}>
              <HSection
                icon="🔥"
                title="Популярное"
                sub="выбор пользователей"
                minCol={150}
              >
                {popular.map((cat, i) => (
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
                : categories.map((cat, i) => (
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
        </div>

        <Footer />
        {/* отступ снизу, чтобы плавающий плеер не перекрывал футер */}
        {playing && !briefOpen && <div style={{ height: 76 }} />}
      </div>
    </>
  );
}
