import React from "react";
import type { ThemeTokens } from "../themeTypes";
import { SectionHeader } from "../shared/SectionHeader";
import ActionBar from "../shared/ActionBar";
import InfoCard from "../shared/InfoCard";
import InlineHint from "../shared/InlineHint";

type HelpPanelProps = {
  colors: ThemeTokens;
  onOpenLaunchAssistant: () => void;
  onOpenFiles: () => void;
  onBackHome: () => void;
};

function panelStyle(colors: ThemeTokens): React.CSSProperties {
  return {
    background: colors.panel,
    border: `1px solid ${colors.border}`,
    borderRadius: 16,
    padding: 16,
    boxShadow: `${colors.inset}, ${colors.shadow}`,
    display: "grid",
    gap: 12,
  };
}

function cardsGridStyle(): React.CSSProperties {
  return {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))",
    gap: 10,
  };
}

function primaryButton(colors: ThemeTokens): React.CSSProperties {
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

function ghostButton(colors: ThemeTokens): React.CSSProperties {
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

export default function HelpPanel({
  colors,
  onOpenLaunchAssistant,
  onOpenFiles,
  onBackHome,
}: HelpPanelProps) {
  return (
    <section style={panelStyle(colors)}>
      <SectionHeader
        colors={colors}
        title="Beginner Runbook"
        subtitle="If you are new to VPS hosting, follow this exact flow. Complete each step before moving on."
      />

      <div style={cardsGridStyle()}>
        <InfoCard
          colors={colors}
          title="Step 1 — Add Domain"
          subtitle="Open Launch Site, add your domain, then copy the DNS checklist and point your A record to this VPS IP."
          icon="🌍"
          compact
        />
        <InfoCard
          colors={colors}
          title="Step 2 — Enable SSL"
          subtitle="Issue Let's Encrypt SSL once DNS is correct. Your site should open on https://."
          icon="🔒"
          compact
        />
        <InfoCard
          colors={colors}
          title="Step 3 — Install WordPress"
          subtitle="Use one-click WordPress install. Keep optional credentials blank to auto-generate secure values."
          icon="🚀"
          compact
        />
        <InfoCard
          colors={colors}
          title="Step 4 — Manage Safely"
          subtitle="Use Files for small edits, Sites to review installs, and only use advanced tools when needed."
          icon="🛡️"
          compact
        />
      </div>

      <InfoCard
        colors={colors}
        title="Quick Troubleshooting"
        icon="🧰"
        tone="default"
      >
        <ul
          style={{
            margin: 0,
            paddingLeft: 18,
            color: colors.textMuted,
            lineHeight: 1.6,
          }}
        >
          <li>Site not opening: run domain health, then confirm DNS points to this server.</li>
          <li>SSL fails: verify DNS is propagated and email address is valid.</li>
          <li>WordPress install fails: re-check domain, admin email, and server service status.</li>
          <li>File editor empty: set domain and path (for example /public_html) then click Browse.</li>
        </ul>
      </InfoCard>

      <InlineHint colors={colors} tone="info">
        Follow the Launch Assistant sequence from top to bottom. Skipping DNS or SSL checks usually causes non-trivial deployment failures later.
      </InlineHint>

      <ActionBar colors={colors} subtle>
        <button style={primaryButton(colors)} onClick={onOpenLaunchAssistant}>
          Open Launch Assistant
        </button>
        <button style={ghostButton(colors)} onClick={onOpenFiles}>
          Open File Manager
        </button>
        <button style={ghostButton(colors)} onClick={onBackHome}>
          Back to Home
        </button>
      </ActionBar>
    </section>
  );
}
