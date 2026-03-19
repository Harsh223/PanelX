import type { CSSProperties } from "react";
import type { ThemeTokens } from "../themeTypes";

export const sectionTitle: CSSProperties = {
  marginTop: 0,
  marginBottom: 10,
};

export function panelStyle(colors: ThemeTokens): CSSProperties {
  return {
    background: colors.panel,
    border: `1px solid ${colors.border}`,
    borderRadius: 16,
    padding: 16,
    boxShadow: `${colors.inset}, ${colors.shadow}`,
  };
}

export function metricCard(colors: ThemeTokens): CSSProperties {
  return {
    background: colors.panelAlt,
    border: `1px solid ${colors.borderSoft}`,
    borderRadius: 12,
    padding: 12,
    boxShadow: colors.inset,
    minHeight: 92,
  };
}

export function serviceCard(
  colors: ThemeTokens,
  active: boolean,
): CSSProperties {
  return {
    background: colors.panelAlt,
    border: `1px solid ${
      active ? "rgba(34,197,94,0.35)" : "rgba(239,68,68,0.35)"
    }`,
    borderRadius: 10,
    padding: 10,
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
  };
}

export function inputStyle(colors: ThemeTokens): CSSProperties {
  return {
    width: "100%",
    border: `1px solid ${colors.border}`,
    borderRadius: 10,
    background: colors.panelAlt,
    color: colors.text,
    padding: "10px 12px",
    outline: "none",
    fontSize: 14,
    lineHeight: 1.35,
  };
}

export function primaryButton(colors: ThemeTokens): CSSProperties {
  return {
    border: `1px solid ${colors.primary}`,
    background: colors.primary,
    color: "#ffffff",
    padding: "10px 14px",
    borderRadius: 10,
    cursor: "pointer",
    fontWeight: 700,
    letterSpacing: 0.2,
  };
}

export function ghostButton(colors: ThemeTokens): CSSProperties {
  return {
    border: `1px solid ${colors.border}`,
    background: colors.panelAlt,
    color: colors.text,
    padding: "10px 12px",
    borderRadius: 10,
    cursor: "pointer",
    fontWeight: 500,
  };
}

export function dangerButton(colors: ThemeTokens): CSSProperties {
  return {
    border: `1px solid ${colors.danger}`,
    background: colors.danger,
    color: "#ffffff",
    padding: "10px 12px",
    borderRadius: 10,
    cursor: "pointer",
    fontWeight: 600,
  };
}

export function tableHeader(colors: ThemeTokens): CSSProperties {
  return {
    textAlign: "left",
    padding: "11px 10px",
    borderBottom: `1px solid ${colors.borderSoft}`,
    color: colors.textMuted,
    fontSize: 12,
    textTransform: "uppercase",
    letterSpacing: 0.4,
    whiteSpace: "nowrap",
  };
}

export function tableCell(colors: ThemeTokens): CSSProperties {
  return {
    padding: "11px 10px",
    borderBottom: `1px solid ${colors.borderSoft}`,
    verticalAlign: "top",
    fontSize: 14,
  };
}

export function filePane(colors: ThemeTokens): CSSProperties {
  return {
    border: `1px solid ${colors.border}`,
    borderRadius: 12,
    padding: 12,
    background: colors.panelAlt,
    boxShadow: colors.inset,
    minHeight: 360,
  };
}

export function gridCards(columns: number): CSSProperties {
  const minWidth =
    columns >= 4 ? 170 : columns === 3 ? 220 : columns === 2 ? 280 : 320;

  return {
    display: "grid",
    gridTemplateColumns: `repeat(auto-fit, minmax(${minWidth}px, 1fr))`,
    gap: 10,
  };
}

export function logoBadge(colors: ThemeTokens): CSSProperties {
  return {
    width: 36,
    height: 36,
    borderRadius: 10,
    display: "grid",
    placeItems: "center",
    fontWeight: 800,
    color: "#fff",
    background: colors.primary,
    boxShadow:
      "inset 0 1px 0 rgba(255,255,255,0.35), 0 6px 18px rgba(0,0,0,0.18)",
    letterSpacing: 0.2,
  };
}

export function mobileQuickNavButtonStyle(
  colors: ThemeTokens,
  active: boolean,
): CSSProperties {
  return {
    border: `1px solid ${active ? colors.primary : colors.border}`,
    background: active ? colors.primarySoft : colors.panelAlt,
    color: colors.text,
    padding: "9px 6px",
    borderRadius: 10,
    cursor: "pointer",
    fontWeight: active ? 700 : 600,
    fontSize: 12,
    minHeight: 36,
  };
}

export const styleSystem = {
  sectionTitle,
  panelStyle,
  metricCard,
  serviceCard,
  inputStyle,
  primaryButton,
  ghostButton,
  dangerButton,
  tableHeader,
  tableCell,
  filePane,
  gridCards,
  logoBadge,
  mobileQuickNavButtonStyle,
} as const;
