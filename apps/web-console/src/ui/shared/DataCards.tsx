import React from "react";
import type { ThemeTokens } from "../themeTypes";
import { metricCard } from "./styleSystem";

type MetricProps = {
  colors: ThemeTokens;
  label: string;
  value: string;
  sub?: string;
};

export function Metric({ colors, label, value, sub }: MetricProps) {
  return (
    <div style={metricCard(colors)}>
      <div style={{ fontSize: 12, color: colors.textMuted }}>{label}</div>
      <div style={{ fontSize: 22, fontWeight: 800, marginTop: 4 }}>{value}</div>
      <div style={{ fontSize: 12, color: colors.textMuted, marginTop: 4 }}>
        {sub ?? ""}
      </div>
    </div>
  );
}

type CredentialItemProps = {
  label: string;
  value: string;
  colors: ThemeTokens;
};

export function CredentialItem({ label, value, colors }: CredentialItemProps) {
  return (
    <div style={metricCard(colors)}>
      <div style={{ fontSize: 12, color: colors.textMuted }}>{label}</div>
      <div
        style={{
          fontSize: 14,
          fontWeight: 700,
          marginTop: 6,
          wordBreak: "break-all",
        }}
      >
        {value || "-"}
      </div>
    </div>
  );
}

export default Metric;
