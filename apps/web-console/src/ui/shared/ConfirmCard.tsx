import type { CSSProperties, ReactNode } from "react";
import type { ThemeTokens } from "../themeTypes";

type ConfirmCardTone = "danger" | "primary";

type ConfirmCardProps = {
  colors: ThemeTokens;
  title: string;
  message: string;
  targetLabel?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: ConfirmCardTone;
  busy?: boolean;
  disabled?: boolean;
  children?: ReactNode;
  onConfirm: () => void;
  onCancel: () => void;
};

function confirmButtonStyle(
  colors: ThemeTokens,
  tone: ConfirmCardTone,
  disabled: boolean,
): CSSProperties {
  const isDanger = tone === "danger";
  const borderColor = isDanger ? colors.danger : colors.primary;
  const bgColor = isDanger ? colors.danger : colors.primary;

  return {
    border: `1px solid ${borderColor}`,
    background: bgColor,
    color: "#ffffff",
    padding: "10px 12px",
    borderRadius: 10,
    cursor: disabled ? "not-allowed" : "pointer",
    fontWeight: 700,
    opacity: disabled ? 0.65 : 1,
  };
}

function cancelButtonStyle(
  colors: ThemeTokens,
  disabled: boolean,
): CSSProperties {
  return {
    border: `1px solid ${colors.border}`,
    background: colors.panel,
    color: colors.text,
    padding: "10px 12px",
    borderRadius: 10,
    cursor: disabled ? "not-allowed" : "pointer",
    fontWeight: 600,
    opacity: disabled ? 0.75 : 1,
  };
}

export function ConfirmCard({
  colors,
  title,
  message,
  targetLabel,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  tone = "danger",
  busy = false,
  disabled = false,
  children,
  onConfirm,
  onCancel,
}: ConfirmCardProps) {
  const isDisabled = busy || disabled;

  return (
    <div
      style={{
        marginTop: 12,
        border: `1px solid ${tone === "danger" ? colors.danger : colors.primary}`,
        borderRadius: 12,
        padding: 12,
        background: colors.panelAlt,
        boxShadow: colors.inset,
        display: "grid",
        gap: 10,
      }}
      role="alertdialog"
      aria-busy={busy}
      aria-live="polite"
    >
      <div style={{ fontWeight: 800 }}>{title}</div>

      <div style={{ color: colors.textMuted, fontSize: 13, lineHeight: 1.5 }}>
        {message}
      </div>

      {targetLabel ? (
        <code
          style={{
            display: "block",
            wordBreak: "break-all",
            border: `1px solid ${colors.borderSoft}`,
            borderRadius: 10,
            padding: "8px 10px",
            background: colors.panel,
            color: colors.text,
            fontSize: 12,
          }}
        >
          {targetLabel}
        </code>
      ) : null}

      {children ? <div>{children}</div> : null}

      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <button
          style={confirmButtonStyle(colors, tone, isDisabled)}
          onClick={onConfirm}
          disabled={isDisabled}
        >
          {busy ? "Working..." : confirmLabel}
        </button>
        <button
          style={cancelButtonStyle(colors, isDisabled)}
          onClick={onCancel}
          disabled={isDisabled}
        >
          {cancelLabel}
        </button>
      </div>
    </div>
  );
}

export default ConfirmCard;
