import React from "react";
import type { Installation, SitesPanelProps } from "../panelContracts";
import { SectionHeader } from "../shared/SectionHeader";
import { ActionBar } from "../shared/ActionBar";
import InfoCard from "../shared/InfoCard";
import { InlineHint } from "../shared/InlineHint";

function tableHeader(borderSoft: string, textMuted: string): React.CSSProperties {
  return {
    textAlign: "left",
    padding: "11px 10px",
    borderBottom: `1px solid ${borderSoft}`,
    color: textMuted,
    fontSize: 12,
    textTransform: "uppercase",
    letterSpacing: 0.4,
    whiteSpace: "nowrap",
  };
}

function tableCell(borderSoft: string): React.CSSProperties {
  return {
    padding: "11px 10px",
    borderBottom: `1px solid ${borderSoft}`,
    verticalAlign: "top",
    fontSize: 14,
  };
}

function credentialValueStyle(): React.CSSProperties {
  return {
    fontSize: 14,
    fontWeight: 700,
    marginTop: 6,
    wordBreak: "break-all",
  };
}

function AccessDetailCard(props: {
  label: string;
  value: string;
  colors: SitesPanelProps["colors"];
}) {
  return (
    <div
      style={{
        background: props.colors.panelAlt,
        border: `1px solid ${props.colors.borderSoft}`,
        borderRadius: 12,
        padding: 12,
        boxShadow: props.colors.inset,
        minHeight: 80,
      }}
    >
      <div style={{ fontSize: 12, color: props.colors.textMuted }}>{props.label}</div>
      <div style={credentialValueStyle()}>{props.value || "-"}</div>
    </div>
  );
}

function CompactInstallCard(props: {
  item: Installation;
  colors: SitesPanelProps["colors"];
  ghostButton: SitesPanelProps["ghostButton"];
  setSelectedInstall: SitesPanelProps["setSelectedInstall"];
  setFileDomain: SitesPanelProps["setFileDomain"];
  setFilePath: SitesPanelProps["setFilePath"];
  setView: SitesPanelProps["setView"];
}) {
  const { item, colors, ghostButton } = props;

  return (
    <InfoCard
      colors={colors}
      title={item.domain}
      subtitle={`${item.installPath} • ${item.status}`}
      icon="🧩"
      compact
      footer={
        <ActionBar colors={colors} subtle>
          <button
            style={ghostButton(colors)}
            onClick={() => props.setSelectedInstall(item)}
          >
            View Access
          </button>
          <button
            style={ghostButton(colors)}
            onClick={() => {
              props.setFileDomain(item.domain);
              props.setFilePath("");
              props.setView("files");
            }}
          >
            Files
          </button>
        </ActionBar>
      }
    >
      <div style={{ display: "grid", gap: 4, fontSize: 13 }}>
        <div style={{ color: colors.textMuted }}>
          <strong style={{ color: colors.text }}>WP Admin User:</strong> {item.adminUser || "-"}
        </div>
        <div style={{ color: colors.textMuted }}>
          <strong style={{ color: colors.text }}>DB Name:</strong> {item.dbName || "-"}
        </div>
        <div style={{ color: colors.textMuted }}>
          <strong style={{ color: colors.text }}>Created:</strong>{" "}
          {new Date(item.createdAt).toLocaleString()}
        </div>
      </div>
    </InfoCard>
  );
}

export default function SitesPanel({
  colors,
  isCompact,
  panelStyle,
  gridCards,
  ghostButton,
  primaryButton,
  installations,
  selectedInstall,
  setSelectedInstall,
  setFileDomain,
  setFilePath,
  setView,
  loadInstallations,
}: SitesPanelProps) {
  return (
    <section style={panelStyle(colors)}>
      <SectionHeader
        colors={colors}
        title="Sites & WordPress Installs"
        subtitle="Review each website, open credentials, and jump to files quickly."
        actions={
          <button
            style={ghostButton(colors)}
            onClick={() => void loadInstallations()}
          >
            Reload
          </button>
        }
      />

      {installations.length === 0 ? (
        <InlineHint colors={colors} tone="warning" icon="🧭">
          No sites yet. Use Launch Assistant to add a domain, issue SSL, and complete your first WordPress install.
        </InlineHint>
      ) : isCompact ? (
        <div style={gridCards(2)}>
          {installations.map((item) => (
            <CompactInstallCard
              key={item.id}
              item={item}
              colors={colors}
              ghostButton={ghostButton}
              setSelectedInstall={setSelectedInstall}
              setFileDomain={setFileDomain}
              setFilePath={setFilePath}
              setView={setView}
            />
          ))}
        </div>
      ) : (
        <div
          style={{
            overflowX: "auto",
            border: `1px solid ${colors.borderSoft}`,
            borderRadius: 12,
            background: colors.panelAlt,
          }}
        >
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <thead>
              <tr>
                {[
                  "Domain",
                  "Path",
                  "WP Admin User",
                  "Database Name",
                  "Status",
                  "Created",
                  "Actions",
                ].map((h) => (
                  <th key={h} style={tableHeader(colors.borderSoft, colors.textMuted)}>
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {installations.map((item) => (
                <tr key={item.id}>
                  <td style={tableCell(colors.borderSoft)}>{item.domain}</td>
                  <td style={tableCell(colors.borderSoft)}>{item.installPath}</td>
                  <td style={tableCell(colors.borderSoft)}>{item.adminUser}</td>
                  <td style={tableCell(colors.borderSoft)}>{item.dbName}</td>
                  <td style={tableCell(colors.borderSoft)}>{item.status}</td>
                  <td style={tableCell(colors.borderSoft)}>
                    {new Date(item.createdAt).toLocaleString()}
                  </td>
                  <td style={tableCell(colors.borderSoft)}>
                    <button
                      style={ghostButton(colors)}
                      onClick={() => setSelectedInstall(item)}
                    >
                      View Access
                    </button>
                    <button
                      style={{ ...ghostButton(colors), marginLeft: 6 }}
                      onClick={() => {
                        setFileDomain(item.domain);
                        setFilePath("");
                        setView("files");
                      }}
                    >
                      Files
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selectedInstall && (
        <InfoCard
          colors={colors}
          title={`Site Access Details: ${selectedInstall.domain}`}
          subtitle="Store credentials safely. Rotate defaults after first login for stronger security."
          icon="🔐"
          actions={
            <button
              style={ghostButton(colors)}
              onClick={() => setSelectedInstall(null)}
            >
              Close
            </button>
          }
        >
          <p style={{ marginTop: 0, marginBottom: 12, color: colors.textMuted }}>
            URL:{" "}
            <a href={selectedInstall.url} target="_blank" rel="noreferrer">
              {selectedInstall.url}
            </a>{" "}
            | Admin:{" "}
            <a href={selectedInstall.adminUrl} target="_blank" rel="noreferrer">
              {selectedInstall.adminUrl}
            </a>
          </p>

          <div style={gridCards(3)}>
            <AccessDetailCard
              label="Admin User"
              value={selectedInstall.adminUser}
              colors={colors}
            />
            <AccessDetailCard
              label="Admin Password"
              value={selectedInstall.adminPassword}
              colors={colors}
            />
            <AccessDetailCard
              label="Admin Email"
              value={selectedInstall.adminEmail}
              colors={colors}
            />
            <AccessDetailCard
              label="DB Name"
              value={selectedInstall.dbName}
              colors={colors}
            />
            <AccessDetailCard
              label="DB User"
              value={selectedInstall.dbUser}
              colors={colors}
            />
            <AccessDetailCard
              label="DB Password"
              value={selectedInstall.dbPassword}
              colors={colors}
            />
          </div>

          <ActionBar colors={colors} subtle style={{ marginTop: 12 }}>
            <button
              style={primaryButton(colors)}
              onClick={() => window.open(selectedInstall.adminUrl, "_blank", "noopener,noreferrer")}
            >
              Open WP Admin
            </button>
            <button
              style={ghostButton(colors)}
              onClick={() => {
                setFileDomain(selectedInstall.domain);
                setFilePath("/public_html");
                setView("files");
              }}
            >
              Open Files
            </button>
          </ActionBar>
        </InfoCard>
      )}
    </section>
  );
}
