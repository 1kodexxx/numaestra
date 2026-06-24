import { BUSINESS } from "@shared/config/business";
import { BrandMark } from "@shared/ui";
import { Link } from "react-router-dom";

const TEXT2 = "rgba(255,255,255,0.5)";
const TEXT3 = "rgba(255,255,255,0.32)";
const BORDER = "rgba(255,255,255,0.07)";
const ACCENT = "#00e5c0";

function FootLink({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link
      to={to}
      style={{
        display: "block",
        fontSize: "13px",
        color: TEXT2,
        textDecoration: "none",
        padding: "5px 0",
        transition: "color 0.15s",
        width: "fit-content",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.color = "#fff";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.color = TEXT2;
      }}
    >
      {children}
    </Link>
  );
}

function ColTitle({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        fontSize: "11px",
        fontWeight: 700,
        color: TEXT3,
        letterSpacing: "0.08em",
        textTransform: "uppercase",
        marginBottom: "10px",
      }}
    >
      {children}
    </div>
  );
}

export function Footer() {
  const year = new Date().getFullYear();
  return (
    <footer style={{ borderTop: `1px solid ${BORDER}`, marginTop: "40px" }}>
      <div style={{ maxWidth: 1100, margin: "0 auto", padding: "44px 24px 0" }}>
        <div
          style={{
            display: "flex",
            flexWrap: "wrap",
            gap: "40px",
            justifyContent: "space-between",
          }}
        >
          {/* Brand */}
          <div style={{ maxWidth: 320 }}>
            <div
              className="brand-link"
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: "10px",
                marginBottom: "12px",
              }}
            >
              <BrandMark size={26} />
              <span
                style={{
                  fontSize: "17px",
                  fontWeight: 800,
                  letterSpacing: "-0.02em",
                  color: "#fff",
                }}
              >
                Numaestra
              </span>
            </div>
            <p style={{ fontSize: "13px", color: TEXT2, lineHeight: 1.6 }}>
              AI-студия персональных песен на заказ. Опишите повод — получите 4
              готовые версии трека уже через 10 минут.
            </p>
            <div
              style={{
                display: "flex",
                flexWrap: "wrap",
                gap: "6px",
                marginTop: "14px",
              }}
            >
              {["4 версии", "за 10 минут", "без подписок"].map((t) => (
                <span
                  key={t}
                  style={{
                    fontSize: "11px",
                    fontWeight: 600,
                    color: ACCENT,
                    background: "rgba(0,229,192,0.08)",
                    border: "1px solid rgba(0,229,192,0.2)",
                    borderRadius: "20px",
                    padding: "3px 10px",
                  }}
                >
                  {t}
                </span>
              ))}
            </div>
          </div>

          {/* Links */}
          <div style={{ display: "flex", gap: "48px", flexWrap: "wrap" }}>
            <div>
              <ColTitle>Сервис</ColTitle>
              <FootLink to="/">Каталог категорий</FootLink>
              <FootLink to="/reviews">Отзывы</FootLink>
              <FootLink to="/status">Мой заказ</FootLink>
            </div>
            <div>
              <ColTitle>Документы</ColTitle>
              <FootLink to="/legal/offer">Публичная оферта</FootLink>
              <FootLink to="/legal/privacy">
                Политика конфиденциальности
              </FootLink>
              <FootLink to="/legal/refund">Политика возврата</FootLink>
              <FootLink to="/legal/copyright">Права на треки</FootLink>
            </div>
            <div>
              <ColTitle>Контакты</ColTitle>
              <FootLink to="/legal/contacts">Реквизиты</FootLink>
              <a
                href={`mailto:${BUSINESS.email}`}
                style={{
                  display: "block",
                  fontSize: "13px",
                  color: TEXT2,
                  textDecoration: "none",
                  padding: "5px 0",
                  width: "fit-content",
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = "#fff";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = TEXT2;
                }}
              >
                {BUSINESS.email}
              </a>
            </div>
          </div>
        </div>

        {/* Bottom bar */}
        <div
          style={{
            display: "flex",
            flexWrap: "wrap",
            gap: "10px",
            justifyContent: "space-between",
            alignItems: "center",
            borderTop: `1px solid ${BORDER}`,
            marginTop: "36px",
            padding: "18px 0 24px",
          }}
        >
          <span style={{ fontSize: "12px", color: TEXT3 }}>
            © {year} Numaestra · Персональные песни на заказ
          </span>
        </div>
      </div>
    </footer>
  );
}
