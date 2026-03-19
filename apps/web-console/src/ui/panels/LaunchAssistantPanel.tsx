import React from "react";
import type { ThemeTokens } from "../themeTypes";
import InfoCard from "../shared/InfoCard";
import { ActionBar } from "../shared/ActionBar";
import { SectionHeader } from "../shared/SectionHeader";
import { InlineHint } from "../shared/InlineHint";

export type LaunchAssistantStep = 1 | 2 | 3 | 4;

export type LaunchAssistantDomain = {
  id: string;
  hostname: string;
};

export type LaunchAssistantProgress = {
  domainReady: boolean;
  sslReady: boolean;
  wordpressReady: boolean;
  completedSteps: number;
  totalSteps: number;
  nextStep: LaunchAssistantStep;
  ctaLabel: string;
};

export type LaunchAssistantSummary = {
  summary: string;
  nextStep: string;
  ready: boolean;
};

type LaunchAssistantPanelProps = {
  colors: ThemeTokens;
  onboardingSummary: LaunchAssistantSummary;
  launchAssistantProgress: LaunchAssistantProgress;
  launchAssistantStep: LaunchAssistantStep;
  setLaunchAssistantStep: (step: LaunchAssistantStep) => void;
  launchAssistantDomain: string;
  setLaunchAssistantDomain: (value: string) => void;
  selectedLaunchDomain: LaunchAssistantDomain | null;
  showOnboardingWizard: boolean;
  onGoToAddDomain: () => void;
  onGoToConfigureSSL: () => void;
  onGoToInstallWordPress: () => void;
  onOpenSite: () => void;
  onCheckDomainHealth: () => void;
  onCopyDNSChecklist: () => void;
  onOpenWpAdmin: () => void;
  onOpenFiles: () => void;
};

function buttonPrimary(colors: ThemeTokens): React.CSSProperties {
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

function buttonGhost(colors: ThemeTokens): React.CSSProperties {
  return {
    border: `1px solid ${colors.border}`,
    background: colors.panelAlt,
    color: colors.text,
    padding: "10px 12px",
    borderRadius: 10,
    cursor: "pointer",
    fontWeight: 600,
  };
}

function stepGridStyle(): React.CSSProperties {
  return {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fit, minmax(170px, 1fr))",
    gap: 10,
  };
}

function statCardStyle(colors: ThemeTokens): React.CSSProperties {
  return {
    background: colors.panelAlt,
    border: `1px solid ${colors.borderSoft}`,
    borderRadius: 12,
    padding: 12,
    boxShadow: colors.inset,
    minHeight: 92,
  };
}

function Metric(props: {
  colors: ThemeTokens;
  label: string;
  value: string;
  sub?: string;
}) {
  return (
    <div style={statCardStyle(props.colors)}>
      <div style={{ fontSize: 12, color: props.colors.textMuted }}>{props.label}</div>
      <div style={{ fontSize: 22, fontWeight: 800, marginTop: 4 }}>{props.value}</div>
      <div style={{ fontSize: 12, color: props.colors.textMuted, marginTop: 4 }}>
        {props.sub ?? ""}
      </div>
    </div>
  );
}

export function LaunchAssistantPanel({
  colors,
  onboardingSummary,
  launchAssistantProgress,
  launchAssistantStep,
  setLaunchAssistantStep,
  launchAssistantDomain,
  setLaunchAssistantDomain,
  selectedLaunchDomain,
  showOnboardingWizard,
  onGoToAddDomain,
  onGoToConfigureSSL,
  onGoToInstallWordPress,
  onOpenSite,
  onCheckDomainHealth,
  onCopyDNSChecklist,
  onOpenWpAdmin,
  onOpenFiles,
}: LaunchAssistantPanelProps) {
  const hasDomain = !!selectedLaunchDomain;

  return (
    <section
      style={{
        background: colors.panel,
        border: `1px solid ${colors.border}`,
        borderRadius: 16,
        padding: 16,
        boxShadow: `${colors.inset}, ${colors.shadow}`,
        display: "grid",
        gap: 12,
      }}
    >
      <SectionHeader
        colors={colors}
        title="Launch Assistant"
        subtitle="Follow this guided workflow to launch a secure WordPress site without VPS experience."
      />

      <InfoCard
        colors={colors}
        title={`Progress: ${launchAssistantProgress.completedSteps}/${launchAssistantProgress.totalSteps} completed`}
        subtitle={onboardingSummary.summary}
        tone={launchAssistantProgress.nextStep === 4 ? "success" : "default"}
      >
        <div
          style={{
            marginTop: 4,
            fontSize: 12,
            color:
              launchAssistantProgress.nextStep === 4
                ? colors.success
                : colors.textMuted,
            fontWeight: 700,
          }}
        >
          {launchAssistantProgress.nextStep === 4
            ? "Everything is ready. You can now manage your live site."
            : `Next action: ${launchAssistantProgress.ctaLabel}`}
        </div>
      </InfoCard>

      <div style={stepGridStyle()}>
        <Metric
          colors={colors}
          label="Step 1"
          value={launchAssistantProgress.domainReady ? "Done" : "Now"}
          sub="Connect your domain"
        />
        <Metric
          colors={colors}
          label="Step 2"
          value={
            launchAssistantProgress.sslReady
              ? "Done"
              : launchAssistantProgress.domainReady
                ? "Now"
                : "Locked"
          }
          sub="Enable HTTPS SSL"
        />
        <Metric
          colors={colors}
          label="Step 3"
          value={
            launchAssistantProgress.wordpressReady
              ? "Done"
              : launchAssistantProgress.sslReady
                ? "Now"
                : "Locked"
          }
          sub="Install WordPress"
        />
        <Metric
          colors={colors}
          label="Step 4"
          value={launchAssistantProgress.nextStep === 4 ? "Ready" : "Waiting"}
          sub="Open site + admin"
        />
      </div>

      <InfoCard
        colors={colors}
        title={`Active Step: ${launchAssistantStep}`}
        subtitle={
          hasDomain
            ? `Selected: ${selectedLaunchDomain.hostname}`
            : "No domain selected yet. Add one first to continue."
        }
      >
        <label style={{ display: "grid", gap: 6, marginBottom: 10 }}>
          <span style={{ fontSize: 14, fontWeight: 600 }}>Working Domain</span>
          <input
            style={{
              width: "100%",
              border: `1px solid ${colors.border}`,
              borderRadius: 10,
              background: colors.panelAlt,
              color: colors.text,
              padding: "10px 12px",
              outline: "none",
              fontSize: 14,
              lineHeight: 1.35,
            }}
            value={launchAssistantDomain}
            onChange={(e) => setLaunchAssistantDomain(e.target.value)}
            placeholder={selectedLaunchDomain?.hostname ?? "example.com"}
          />
        </label>

        <ActionBar colors={colors} subtle>
          <button
            style={buttonPrimary(colors)}
            onClick={() => {
              setLaunchAssistantStep(1);
              onGoToAddDomain();
            }}
          >
            Step 1: Add Domain
          </button>

          <button
            style={buttonGhost(colors)}
            disabled={!hasDomain}
            onClick={() => {
              setLaunchAssistantStep(2);
              onGoToConfigureSSL();
            }}
          >
            Step 2: Configure SSL
          </button>

          <button
            style={buttonGhost(colors)}
            disabled={!hasDomain}
            onClick={() => {
              setLaunchAssistantStep(3);
              onGoToInstallWordPress();
            }}
          >
            Step 3: Install WordPress
          </button>

          <button
            style={buttonGhost(colors)}
            disabled={!hasDomain}
            onClick={() => onOpenSite()}
          >
            Step 4: Open Site
          </button>
        </ActionBar>

        {hasDomain && (
          <ActionBar colors={colors} subtle style={{ marginTop: 8 }}>
            <button style={buttonGhost(colors)} onClick={onCheckDomainHealth}>
              Check DNS + Health
            </button>
            <button style={buttonGhost(colors)} onClick={onCopyDNSChecklist}>
              Copy DNS Checklist
            </button>
            <button style={buttonGhost(colors)} onClick={onOpenWpAdmin}>
              Open WP Admin
            </button>
            <button style={buttonGhost(colors)} onClick={onOpenFiles}>
              Open Files
            </button>
          </ActionBar>
        )}
      </InfoCard>

      <InlineHint colors={colors} tone={showOnboardingWizard ? "warning" : "info"}>
        Assistant status: {launchAssistantProgress.ctaLabel}{" "}
        {showOnboardingWizard ? "(guided mode)" : "(normal mode)"}
      </InlineHint>
    </section>
  );
}

export default LaunchAssistantPanel;
