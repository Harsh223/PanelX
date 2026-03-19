import React from "react";
import type { ThemeTokens } from "../themeTypes";

export type InlineHintTone = "info" | "success" | "warning" | "danger";

type InlineHintProps = {
  colors: ThemeTokens;
  children: React.ReactNode;
  tone?: InlineHintTone;
  icon?: React.ReactNode;
  compact?: boolean;
  style?: React.CSSProperties;
};

function toneStyles(
  colors: ThemeTokens,
  tone: InlineHintTone,
): {
  border: string;
  background: string;
  color: string;
} {
  if (tone === "success") {
    return {
      border: "rgba(34,197,94,0.35)",
      background: colors.panelAlt,
      color: colors.success,
    };
  }

  if (tone === "warning") {
    return {
      border: "rgba(245,158,11,0.45)",
      background: colors.panelAlt,
      color: colors.text,
    };
  }

  if (tone === "danger") {
    return {
      border: "rgba(239,68,68,0.45)",
      background: colors.panelAlt,
      color: colors.danger,
    };
  }

  return {
    border: colors.borderSoft,
    background: colors.panelAlt,
    color: colors.textMuted,
  };
}

export function InlineHint({
  colors,
  children,
  tone = "info",
  icon,
  compact = false,
  style,
}: InlineHintProps) {
  const palette = toneStyles(colors, tone);

  return (
    <div
      role="note"
      style={{
        display: "flex",
        alignItems: "flex-start",
        gap: 8,
        border: `1px solid ${palette.border}`,
        background: palette.background,
        color: palette.color,
        borderRadius: 10,
        padding: compact ? "6px 8px" : "8px 10px",
        fontSize: compact ? 12 : 13,
        lineHeight: 1.5,
        boxShadow: colors.inset,
        ...style,
      }}
    >
      <span style={{ lineHeight: 1.2 }}>{icon ?? "ℹ️"}</span>
      <span>{children}</span>
    </div>
  );
}

export default InlineHint;
