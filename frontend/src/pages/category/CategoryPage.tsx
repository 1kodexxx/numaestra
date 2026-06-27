import type { Question, WizardData } from "@entities/category";
import { categoryApi } from "@entities/category";
import { useCreateOrder } from "@features/create-order";
import { useCatalog } from "@features/load-catalog";
import { EXAMPLE_SONGS } from "@shared/data/examples";
import { categoryCover } from "@shared/lib/categoryCover";
import { useSeo } from "@shared/lib/seo";
import { composeCategoryBrief } from "@shared/lib/sunoPrompt";
import { usePublicConfig } from "@shared/lib/usePublicConfig";
import { theme } from "@shared/lib/theme";
import { Button, TextField } from "@shared/ui";
import { ContactModal } from "@widgets/contact-modal";
import {
  PanelHeader,
  PlayOverlay,
  RankCorner,
  SideItem,
  stockImage,
  Thumb,
} from "@widgets/side-panel";
import { useEffect, useMemo, useState } from "react";
import { getReferralCode } from "@shared/lib/referral";
import { promoApi } from "@entities/admin-promo";
import { useNavigate, useParams } from "react-router-dom";
import { useBreakpoint } from "@shared/lib/useBreakpoint";
import { GOALS, reachGoal } from "@shared/lib/analytics";
import { breadcrumbJsonLd, injectJsonLd } from "@shared/lib/jsonLd";

const ACCENT = theme.accent;
const BORDER = theme.border;

// Жанры: пользователь может выбирать пресеты И добавлять свои, но суммарно не
// более трёх. mapping_key совпадает с ключом GENRE в Suno-тегах на бэкенде.
const GENRE_KEY = "GENRE";
const MAX_GENRES = 3;
const TEXT2 = theme.text2;
const TEXT3 = theme.text3;
const PANEL_W = 240;
const SURFACE = theme.dark;

/* ─── chip ─── */
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

/* ─── section label ─── */
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
        <span
          style={{
            fontSize: "13px",
            color: "rgba(255,255,255,0.7)",
            fontWeight: 600,
          }}
        >
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

/* ─── one wizard question → подходящий контрол ─── */
function QuestionField({
  q,
  textValue,
  tagValues,
  onText,
  onToggleTag,
  allowCustom,
  maxSelect,
  onCustomValue,
}: {
  q: Question;
  textValue: string;
  tagValues: string[];
  onText: (v: string) => void;
  onToggleTag: (v: string) => void;
  allowCustom?: boolean;
  maxSelect?: number;
  onCustomValue?: (v: string) => void;
}) {
  const [customInput, setCustomInput] = useState("");

  if (q.ui_type === "text" || q.ui_type === "textarea") {
    return (
      <TextField
        label={q.question_text}
        required={q.is_required}
        value={textValue}
        onChange={onText}
        multiline={q.ui_type === "textarea"}
        rows={3}
        placeholder={q.config?.placeholder}
        surfaceColor={SURFACE}
      />
    );
  }

  const multi = q.ui_type === "tags";
  // Свои значения = выбранные, которых нет среди пресетов (добавлены пользователем).
  const optionValues = q.options.map((o) => o.value);
  const customValues = multi ? tagValues.filter((v) => !optionValues.includes(v)) : [];
  const atLimit = maxSelect != null && tagValues.length >= maxSelect;

  const hint = q.config?.hint
    ?? (multi
      ? allowCustom
        ? `до ${maxSelect ?? MAX_GENRES} — выбирайте из списка или добавьте свой`
        : `можно выбрать несколько${maxSelect ? `, до ${maxSelect}` : ""}`
      : undefined);

  function addCustom() {
    const v = customInput.trim();
    if (!v) return;
    const exists = tagValues.some((x) => x.toLowerCase() === v.toLowerCase());
    if (!exists) {
      if (onCustomValue) onCustomValue(v);
      else onToggleTag(v);
    }
    setCustomInput("");
  }

  return (
    <Section
      label={q.question_text}
      required={q.is_required}
      hint={hint}
    >
      {q.options.map((opt) => {
        const sel = multi
          ? tagValues.includes(opt.value)
          : textValue === opt.value;
        return (
          <Chip
            key={opt.value}
            label={opt.label}
            selected={sel}
            onClick={() =>
              multi
                ? onToggleTag(opt.value)
                : onText(textValue === opt.value ? "" : opt.value)
            }
          />
        );
      })}
      {/* Свои жанры — клик по чипу убирает его */}
      {customValues.map((v) => (
        <Chip key={`custom:${v}`} label={`${v} ✕`} selected onClick={() => onToggleTag(v)} />
      ))}
      {allowCustom && (
        <div style={{ display: "flex", gap: "6px", width: "100%", marginTop: "4px" }}>
          <input
            value={customInput}
            onChange={(e) => setCustomInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addCustom(); } }}
            disabled={atLimit}
            placeholder={atLimit ? `Достигнут лимит — ${maxSelect ?? MAX_GENRES} жанра` : "Свой жанр…"}
            maxLength={40}
            style={{
              flex: 1, minWidth: 0, padding: "9px 14px", borderRadius: "20px",
              background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.1)",
              color: "#fff", fontSize: "13px", fontFamily: "inherit", outline: "none",
              opacity: atLimit ? 0.5 : 1,
            }}
          />
          <Button size="sm" onClick={addCustom} disabled={atLimit || !customInput.trim()}>
            Добавить
          </Button>
        </div>
      )}
    </Section>
  );
}

export function CategoryPage() {
  const publicConfig = usePublicConfig();
  const { id = "" } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { isMobile, isDesktop } = useBreakpoint();
  const { categories, loading: catalogLoading } = useCatalog();
  const [wizard, setWizard] = useState<WizardData | null>(null);
  const [wizardLoading, setWizardLoading] = useState(true);
  const [wizardError, setWizardError] = useState<string | null>(null);
  const [quizError, setQuizError] = useState<string | null>(null);
  const [answers, setAnswers] = useState<Record<string, string>>({}); // text / select / radio
  const [tagSel, setTagSel] = useState<Record<string, string[]>>({}); // tags (multi)
  const [customText, setCustomText] = useState("");
  const [showContact, setShowContact] = useState(false);
  const [promoCode, setPromoCode] = useState("");
  const [promoStatus, setPromoStatus] = useState<{ discount_kopecks: number; label: string } | null>(null);
  const [promoLoading, setPromoLoading] = useState(false);
  const [promoError, setPromoError] = useState<string | null>(null);
  const { loading: submitting, error: submitError, submit } = useCreateOrder();

  const category = categories.find((c) => c.id === id);
  const categoryMissing = !catalogLoading && categories.length > 0 && !category;

  useSeo({
    title: category
      ? `${category.title} — заказать песню`
      : "Конструктор песни",
    description:
      category?.description ||
      "Соберите свою песню: повод, настроение, жанр и детали. 4 готовые версии за 10 минут.",
  });

  useEffect(() => {
    if (!category) return;
    injectJsonLd(
      "ld-breadcrumb",
      breadcrumbJsonLd([
        { name: "Главная", path: "/" },
        { name: category.title, path: `/category/${id}` },
      ]),
    );
    return () => document.getElementById("ld-breadcrumb")?.remove();
  }, [category, id]);

  useEffect(() => {
    setWizard(null);
    setAnswers({});
    setTagSel({});
    setCustomText("");
    setWizardError(null);
    setQuizError(null);
    setWizardLoading(true);
    categoryApi
      .wizard(id)
      .then((w) => {
        setWizard(w);
        setWizardError(null);
      })
      .catch((err: Error) => {
        setWizard(null);
        setWizardError(err.message || "Не удалось загрузить форму");
      })
      .finally(() => setWizardLoading(false));
  }, [id]);

  // Итоговая карта ответов: одиночные + мультивыбор (склеенный запятой).
  const mergedAnswers = useMemo(() => {
    const m: Record<string, string> = { ...answers };
    for (const [k, arr] of Object.entries(tagSel))
      if (arr.length) m[k] = arr.join(", ");
    return m;
  }, [answers, tagSel]);

  const preview = useMemo(() => {
    if (!wizard?.base_prompt_template) return "";
    return composeCategoryBrief(
      wizard.title || category?.title || "",
      wizard.base_prompt_template,
      mergedAnswers,
      customText,
    );
  }, [wizard, mergedAnswers, category?.title, customText]);

  function toggleTag(key: string, val: string, maxSelect?: number) {
    setTagSel((prev) => {
      const arr = prev[key] ?? [];
      if (arr.includes(val)) {
        return { ...prev, [key]: arr.filter((x) => x !== val) };
      }
      if (maxSelect && arr.length >= maxSelect) return prev;
      return { ...prev, [key]: [...arr, val] };
    });
  }

  function validateQuiz(): string | null {
    if (!wizard) return "Форма категории ещё загружается";
    for (const q of wizard.questions) {
      if (!q.is_required) continue;
      const val = mergedAnswers[q.mapping_key];
      if (!val?.trim()) return `Заполните обязательное поле: ${q.question_text}`;
      if (q.ui_type === "tags") {
        const min = q.config?.min_select;
        if (min != null && min > 0) {
          const count = tagSel[q.mapping_key]?.length ?? 0;
          if (count < min) {
            return `Выберите минимум ${min} в «${q.question_text}»`;
          }
        }
      }
    }
    return null;
  }

  const quizProgress = useMemo(() => {
    if (!wizard) return null;
    const required = wizard.questions.filter((q) => q.is_required);
    const done = required.filter((q) => mergedAnswers[q.mapping_key]?.trim());
    return { done: done.length, total: required.length };
  }, [wizard, mergedAnswers]);

  function openContact() {
    const err = validateQuiz();
    if (err) {
      setQuizError(err);
      return;
    }
    setQuizError(null);
    reachGoal(GOALS.CONTACT_OPEN, { source: "category", category_id: id });
    setShowContact(true);
  }

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
      setPromoError(null);
    } catch {
      // Сохраняем введённый код, но показываем явную обратную связь.
      setPromoStatus(null);
      setPromoError("Промокод недействителен или истёк");
    } finally {
      setPromoLoading(false);
    }
  }

  async function handleOrder(email: string, phone: string) {
    const orderAnswers = { ...mergedAnswers };
    if (customText.trim()) {
      orderAnswers.CUSTOM_LYRICS = customText.trim();
    }
    await submit({
      email,
      phone,
      brief: preview,
      category_id: id,
      answers: orderAnswers,
      consent_doc_version: publicConfig.consent_doc_version,
      promo_code: promoStatus ? promoCode.trim().toUpperCase() : undefined,
      referral_code: getReferralCode() || undefined,
    });
  }

  const topCats = categories.slice(0, 8);
  const icon = "🎵";

  if (categoryMissing) {
    return (
      <div style={{ maxWidth: 480, margin: "0 auto", padding: "64px 24px", textAlign: "center" }}>
        <div style={{ fontSize: "48px", marginBottom: "16px" }}>🔍</div>
        <h1 style={{ fontSize: "22px", fontWeight: 800, marginBottom: "8px" }}>Категория не найдена</h1>
        <p style={{ fontSize: "14px", color: TEXT2, marginBottom: "24px" }}>
          Возможно, ссылка устарела. Выберите другую категорию на главной.
        </p>
        <Button size="lg" onClick={() => navigate("/")}>На главную →</Button>
      </div>
    );
  }

  /* ── контент конструктора (общий для мобайла и десктопа) ── */
  const formBody = (
    <>
      <button
        onClick={() => navigate("/")}
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

      {/* header */}
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
          {icon}
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
              fontSize: isMobile ? "22px" : "26px",
              fontWeight: 800,
              letterSpacing: "-0.03em",
              lineHeight: 1.15,
            }}
          >
            {category?.title ?? "Категория"}
          </h1>
        </div>
      </div>
      {category?.description && (
        <p
          style={{
            fontSize: "14px",
            color: TEXT2,
            lineHeight: 1.55,
            marginBottom: "26px",
          }}
        >
          {category.description}
        </p>
      )}

      {quizProgress && quizProgress.total > 0 && (
        <div style={{ marginBottom: "20px" }}>
          <div style={{ display: "flex", justifyContent: "space-between", fontSize: "12px", color: TEXT2, marginBottom: "6px" }}>
            <span>Заполнено обязательных полей</span>
            <span>{quizProgress.done} / {quizProgress.total}</span>
          </div>
          <div style={{ height: "4px", borderRadius: "2px", background: "rgba(255,255,255,0.08)", overflow: "hidden" }}>
            <div
              style={{
                height: "100%",
                width: `${(quizProgress.done / quizProgress.total) * 100}%`,
                background: `linear-gradient(90deg, ${theme.accent}, ${theme.accent2})`,
                transition: "width 0.25s ease",
              }}
            />
          </div>
        </div>
      )}

      {/* questions */}
      <div style={{ display: "flex", flexDirection: "column", gap: "24px" }}>
        {wizardError && (
          <div style={{
            padding: "14px 16px", borderRadius: "12px",
            background: "rgba(239,68,68,0.08)", border: "1px solid rgba(239,68,68,0.2)",
            fontSize: "13px", color: "#f87171",
          }}>
            {wizardError}
            <button
              type="button"
              onClick={() => {
                setWizardLoading(true);
                setWizardError(null);
                categoryApi.wizard(id).then(setWizard).catch((e: Error) => setWizardError(e.message)).finally(() => setWizardLoading(false));
              }}
              style={{ display: "block", marginTop: "8px", background: "none", border: "none", color: ACCENT, cursor: "pointer", fontSize: "13px", padding: 0 }}
            >
              Повторить загрузку
            </button>
          </div>
        )}
        {wizard
          ? wizard.questions.map((q) => (
              <QuestionField
                key={q.id}
                q={q}
                textValue={answers[q.mapping_key] ?? ""}
                tagValues={tagSel[q.mapping_key] ?? []}
                onText={(v) =>
                  setAnswers((prev) => ({ ...prev, [q.mapping_key]: v }))
                }
                allowCustom={q.mapping_key === GENRE_KEY}
                maxSelect={q.mapping_key === GENRE_KEY ? MAX_GENRES : q.config?.max_select}
                onToggleTag={(v) => toggleTag(q.mapping_key, v, q.mapping_key === GENRE_KEY ? MAX_GENRES : q.config?.max_select)}
                onCustomValue={
                  q.mapping_key === GENRE_KEY
                    ? (v) => {
                        setAnswers((prev) => {
                          const extra = prev.EXTRA?.trim() ?? "";
                          const line = `Preferred genre style: ${v}`;
                          return {
                            ...prev,
                            EXTRA: extra ? `${extra}\n${line}` : line,
                          };
                        });
                      }
                    : undefined
                }
              />
            ))
          : wizardLoading
            ? Array.from({ length: 6 }, (_, i) => (
              <div
                key={i}
                className="skeleton"
                style={{
                  height: "52px",
                  borderRadius: "12px",
                  opacity: 1 - i * 0.12,
                }}
              />
            ))
            : null}

        {/* universal extras */}
        {wizard && (
        <>
        <TextField
          label="Свой текст песни (по желанию)"
          value={customText}
          onChange={setCustomText}
          multiline
          rows={3}
          placeholder="Строки или припев, которые должны прозвучать дословно..."
          surfaceColor={SURFACE}
        />

        {/* live preview */}
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
        </>
        )}
      </div>
    </>
  );

  const promoDiscountLabel = promoStatus
    ? `${promoStatus.label} — сэкономите ${Math.round(promoStatus.discount_kopecks / 100)} ₽`
    : null;

  const orderBar = (pad: string) => (
    <div
      className="safe-bottom"
      style={{
        padding: pad,
        borderTop: `1px solid ${BORDER}`,
        background: SURFACE,
      }}
    >
      {/* Поле промокода */}
      <div style={{ display: "flex", gap: "8px", marginBottom: "10px", alignItems: "center" }}>
        <div style={{ flex: 1 }}>
          <TextField
            label="Промокод (необязательно)"
            value={promoCode}
            onChange={v => { setPromoCode(v.toUpperCase()); setPromoStatus(null); setPromoError(null); }}
            disabled={!!promoStatus}
          />
        </div>
        {promoStatus ? (
          <Button size="sm" onClick={() => { setPromoCode(""); setPromoStatus(null); setPromoError(null); }}>✕</Button>
        ) : (
          <Button size="sm" onClick={applyPromoCode} disabled={!promoCode.trim() || promoLoading}>
            {promoLoading ? "…" : "Применить"}
          </Button>
        )}
      </div>
      {promoDiscountLabel && (
        <div style={{ fontSize: "12px", color: ACCENT, marginBottom: "8px", textAlign: "center" }}>
          ✓ Промокод применён: {promoDiscountLabel}
        </div>
      )}
      {promoError && (
        <div style={{ fontSize: "12px", color: "#f87171", marginBottom: "8px", textAlign: "center" }}>
          ✕ {promoError}
        </div>
      )}
      {quizError && (
        <div style={{ fontSize: "12px", color: "#f87171", marginBottom: "10px", textAlign: "center" }}>
          {quizError}
        </div>
      )}
      <Button size="lg" fullWidth disabled={!wizard || wizardLoading} onClick={openContact}>
        Создать песню — демо бесплатно →
      </Button>
      <div style={{ fontSize: "12px", color: TEXT3, textAlign: "center", marginTop: "8px" }}>
        Оплата только после демо, если понравится
      </div>
    </div>
  );

  const modal = showContact && (
    <ContactModal
      loading={submitting}
      error={submitError}
      priceLabel={publicConfig.price_label}
      onClose={() => setShowContact(false)}
      onSubmit={handleOrder}
    />
  );

  /* ── mobile / tablet ── */
  if (!isDesktop) {
    const p = isMobile ? "16px" : "24px";
    return (
      <div className="category-mobile-shell">
        {modal}
        <div
          style={
            {
              flex: 1,
              overflowY: "auto",
              minHeight: 0,
              padding: `20px ${p} 28px`,
              WebkitOverflowScrolling: "touch",
            } as React.CSSProperties
          }
        >
          {formBody}
        </div>
        {orderBar(`12px ${p} 18px`)}
      </div>
    );
  }

  /* ── desktop: examples | constructor | categories ── */
  return (
    <div
      style={{
        display: "flex",
        height: "calc(100dvh - 60px)",
        overflow: "hidden",
      }}
    >
      {modal}

      <aside
        style={{
          width: PANEL_W,
          flexShrink: 0,
          borderRight: `1px solid ${BORDER}`,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            flex: 1,
            overflowY: "auto",
            minHeight: 0,
            padding: "18px 10px 12px",
          }}
        >
          <PanelHeader icon="🎧" title="Примеры" sub="Послушайте, как звучит" />
          {EXAMPLE_SONGS.map((ex, i) => (
            <SideItem
              key={ex.id}
              index={i}
              title={ex.title}
              sub={ex.category}
              onClick={() => navigate(`/examples/${ex.id}`)}
              leading={(hovered) => (
                <Thumb
                  src={ex.coverUrl || stockImage(ex.id, "concert,music")}
                  alt={`Обложка песни «${ex.title}»`}
                  active={hovered}
                >
                  <PlayOverlay active={hovered} />
                </Thumb>
              )}
            />
          ))}
        </div>
      </aside>

      <main
        style={{
          flex: 1,
          minWidth: 0,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            flex: 1,
            overflowY: "auto",
            minHeight: 0,
            padding: "28px 36px",
          }}
        >
          <div style={{ maxWidth: 680, margin: "0 auto" }}>{formBody}</div>
        </div>
        <div style={{ maxWidth: 680, margin: "0 auto", width: "100%" }}>
          {orderBar("16px 36px 20px")}
        </div>
      </main>

      <aside
        style={{
          width: PANEL_W,
          flexShrink: 0,
          borderLeft: `1px solid ${BORDER}`,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            flex: 1,
            overflowY: "auto",
            minHeight: 0,
            padding: "18px 10px 12px",
          }}
        >
          <PanelHeader icon="🔥" title="Популярное" sub="Другие категории" />
          {topCats.map((cat, i) => (
            <SideItem
              key={cat.id}
              index={i}
              title={cat.title}
              onClick={() => navigate(`/category/${cat.id}`)}
              leading={(hovered) => (
                <Thumb
                  src={
                    categoryCover(cat.id, cat.cover_image_url) ||
                    stockImage(cat.id, "celebration,party")
                  }
                  alt={`Обложка категории ${cat.title}`}
                  active={hovered}
                >
                  <RankCorner n={i + 1} />
                </Thumb>
              )}
            />
          ))}
        </div>
      </aside>
    </div>
  );
}
