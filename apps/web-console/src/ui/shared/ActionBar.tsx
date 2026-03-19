import React from "react";
import { ThemeTokens } from "../themeTypes";

type ActionBarProps = {
  colors: ThemeTokens;
  children: React.ReactNode;
  align?: "start" | "center" | "end";
  justify?: "start" | "between" | "end";
  wrap?: boolean;
  subtle?: boolean;
  style?: React.CSSProperties;
};

function mapAlign(value: NonNullable<ActionBarProps["align"]>): React.CSSProperties["alignItems"] {
  if (value === "center") return "center";
  if (value === "end") return "flex-end";
  return "flex-start";
}

function mapJustify(
  value: NonNullable<ActionBarProps["justify"]>,
): React.CSSProperties["justifyContent"] {
  if (value === "between") return "space-between";
  if (value === "end") return "flex-end";
  return "flex-start";
}

export function ActionBar({
  colors,
  children,
  align = "center",
  justify = "start",
  wrap = true,
  subtle = false,
  style,
}: ActionBarProps) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: mapAlign(align),
        justifyContent: mapJustify(justify),
        gap: 8,
        flexWrap: wrap ? "wrap" : "nowrap",
        padding: "10px 12px",
        borderRadius: 12,
        border: `1px solid ${colors.borderSoft}`,
        background: subtle ? colors.panelAlt : colors.panel,
        boxShadow: subtle ? colors.inset : `${colors.inset}, ${colors.shadow}`,
        ...style,
      }}
    >
      {children}
    </div>
  );
}

export default ActionBar;
