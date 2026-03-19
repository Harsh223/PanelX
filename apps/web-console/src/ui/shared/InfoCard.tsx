import React from "react";
import { ThemeTokens } from "../themeTypes";

export type InfoCardTone = "default" | "success" | "warning" | "danger";

export type InfoCardProps = {
  colors: ThemeTokens;
  title: string;
  subtitle?: string;
  icon?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  footer?: React.ReactNode;
  tone?: InfoCardTone;
  compact?: boolean;
};

function toneBorder(colors: ThemeTokens, tone: InfoCardTone): string {
  if (tone === "success") return "rgba(34,197,94,0.35)";
  if (tone === "danger") return "rgba(239,68,68,0.45)";
  if (tone === "warning") return "rgba(245,158,11,0.45)";
  return colors.borderSoft;
}

function toneTitleColor(colors: ThemeTokens, tone: InfoCardTone): string {
  if (tone === "success") return colors.success;
  if (tone === "danger") return colors.danger;
  return colors.text;
}

export default function InfoCard({
  colors,
  title,
  subtitle,
  icon,
  actions,
  children,
  footer,
  tone = "default",
  compact = false,
}: InfoCardProps) {
  return (
    <section
      style={{
        border: `1px solid ${toneBorder(colors, tone)}`,
        borderRadius: 12,
        background: colors.panelAlt,
        boxShadow: colors.inset,
        padding: compact ? 10 : 12,
        display: "grid",
        gap: compact ? 8 : 10,
      }}
    >
      <header
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          gap: 10,
        }}
      >
        <div style={{ display: "flex", alignItems: "flex-start", gap: 8 }}>
          {icon ? <span style={{ lineHeight: 1.2 }}>{icon}</span> : null}
          <div>
            <div
              style={{
                fontSize: compact ? 15 : 16,
                fontWeight: 800,
                color: toneTitleColor(colors, tone),
                lineHeight: 1.25,
              }}
            >
              {title}
            </div>
            {subtitle ? (
              <div
                style={{
                  marginTop: 4,
                  color: colors.textMuted,
                  fontSize: 13,
                  lineHeight: 1.45,
                }}
              >
                {subtitle}
              </div>
            ) : null}
          </div>
        </div>

        {actions ? <div style={{ display: "flex", gap: 8 }}>{actions}</div> : null}
      </header>

      {children ? <div>{children}</div> : null}

      {footer ? (
        <footer
          style={{
            borderTop: `1px solid ${colors.borderSoft}`,
            paddingTop: 10,
          }}
        >
          {footer}
        </footer>
      ) : null}
    </section>
  );
}
