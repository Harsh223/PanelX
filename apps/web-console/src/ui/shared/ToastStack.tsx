import React from "react";
import type { ThemeTokens } from "../themeTypes";

export type ToastKind = "success" | "error" | "info";

export type ToastItem = {
  id: number;
  text: string;
  kind?: ToastKind;
};

type ToastStackProps = {
  colors: ThemeTokens;
  toasts: ToastItem[];
  onDismiss: (id: ToastItem["id"]) => void;
  isCompact?: boolean;
  topOffset?: number;
  rightOffset?: number;
  width?: string | number;
};

function toneColor(colors: ThemeTokens, kind: ToastKind): string {
  if (kind === "error") return colors.danger;
  if (kind === "success") return colors.success;
  return colors.text;
}

function toneBorder(colors: ThemeTokens, kind: ToastKind): string {
  if (kind === "error") return "rgba(239,68,68,0.45)";
  if (kind === "success") return "rgba(34,197,94,0.45)";
  return colors.borderSoft;
}

function dismissButtonStyle(colors: ThemeTokens): React.CSSProperties {
  return {
    border: `1px solid ${colors.border}`,
    background: colors.panelAlt,
    color: colors.text,
    padding: "6px 10px",
    borderRadius: 8,
    cursor: "pointer",
    fontWeight: 600,
    fontSize: 12,
    lineHeight: 1.2,
  };
}

export function ToastStack({
  colors,
  toasts,
  onDismiss,
  isCompact = false,
  topOffset,
  rightOffset = 14,
  width = "min(92vw, 360px)",
}: ToastStackProps) {
  if (toasts.length === 0) return null;

  const top = topOffset ?? (isCompact ? 128 : 88);

  return (
    <div
      style={{
        position: "fixed",
        top,
        right: rightOffset,
        zIndex: 40,
        display: "grid",
        gap: 8,
        width,
      }}
      aria-live="polite"
      aria-atomic={false}
    >
      {toasts.map((toast) => {
        const kind = toast.kind ?? "info";

        return (
          <div
            key={toast.id}
            role="status"
            style={{
              border: `1px solid ${toneBorder(colors, kind)}`,
              borderRadius: 12,
              background: colors.panel,
              boxShadow: `${colors.inset}, ${colors.shadow}`,
              padding: "10px 12px",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 10,
              fontSize: 13,
            }}
          >
            <span
              style={{
                color: toneColor(colors, kind),
                fontWeight: 700,
                lineHeight: 1.35,
              }}
            >
              {toast.text}
            </span>

            <button
              type="button"
              style={dismissButtonStyle(colors)}
              onClick={() => onDismiss(toast.id)}
              aria-label="Dismiss notification"
            >
              Dismiss
            </button>
          </div>
        );
      })}
    </div>
  );
}

export default ToastStack;
