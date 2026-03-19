import React from "react";
import type { ThemeTokens } from "../themeTypes";

type SectionHeaderProps = {
  colors: ThemeTokens;
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
  compact?: boolean;
};

export function SectionHeader({
  colors,
  title,
  subtitle,
  actions,
  compact = false,
}: SectionHeaderProps) {
  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: compact ? "center" : "flex-start",
        gap: 10,
        marginBottom: compact ? 8 : 10,
        flexWrap: "wrap",
      }}
    >
      <div>
        <h3
          style={{
            marginTop: 0,
            marginBottom: subtitle ? 6 : 0,
          }}
        >
          {title}
        </h3>
        {subtitle && (
          <p
            style={{
              marginTop: 0,
              marginBottom: 0,
              color: colors.textMuted,
            }}
          >
            {subtitle}
          </p>
        )}
      </div>
      {actions && (
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>{actions}</div>
      )}
    </div>
  );
}
