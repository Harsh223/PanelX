import React from "react";
import type { FilesPanelProps, FileEntry } from "../panelContracts";
import { SectionHeader } from "../shared/SectionHeader";
import { ActionBar } from "../shared/ActionBar";
import { InlineHint } from "../shared/InlineHint";
import { ConfirmCard } from "../shared/ConfirmCard";

function filePane(colors: FilesPanelProps["colors"]): React.CSSProperties {
  return {
    border: `1px solid ${colors.border}`,
    borderRadius: 12,
    padding: 12,
    background: colors.panelAlt,
    boxShadow: colors.inset,
    minHeight: 360,
  };
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const idx = Math.min(
    units.length - 1,
    Math.floor(Math.log(bytes) / Math.log(1024)),
  );
  const value = bytes / Math.pow(1024, idx);
  return `${value.toFixed(value >= 100 || idx === 0 ? 0 : 1)} ${units[idx]}`;
}

function EntryRow(props: {
  entry: FileEntry;
  colors: FilesPanelProps["colors"];
  ghostButton: FilesPanelProps["ghostButton"];
  dangerButton: FilesPanelProps["dangerButton"];
  onOpen: (path: string, isDir: boolean) => void;
  onDelete: (path: string) => void;
}) {
  const { entry, colors, ghostButton, dangerButton, onOpen, onDelete } = props;

  return (
    <div style={{ display: "flex", gap: 8, marginBottom: 6 }}>
      <button
        style={{
          ...ghostButton(colors),
          width: "100%",
          textAlign: "left",
        }}
        onClick={() => onOpen(entry.path, entry.isDir)}
        title={entry.path}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 8,
            width: "100%",
          }}
        >
          <span>
            {entry.isDir ? "📁" : "📄"} {entry.name}
          </span>
          <span
            style={{
              fontSize: 12,
              color: colors.textMuted,
              whiteSpace: "nowrap",
            }}
          >
            {entry.isDir ? "Folder" : formatBytes(entry.size)}
          </span>
        </div>
      </button>

      {!entry.isDir && (
        <button style={dangerButton(colors)} onClick={() => onDelete(entry.path)}>
          Delete
        </button>
      )}
    </div>
  );
}

export function FilesPanel({
  colors,
  isCompact,
  panelStyle,
  inputStyle,
  primaryButton,
  ghostButton,
  dangerButton,
  fileDomain,
  filePath,
  entries,
  selectedPath,
  fileContent,
  pendingDeletePath,
  fileQuickPaths,
  fileBreadcrumbs,
  setFileDomain,
  setFilePath,
  setFileContent,
  setPendingDeletePath,
  listFiles,
  openFile,
  saveFile,
  deletePath,
}: FilesPanelProps) {
  return (
    <section style={panelStyle(colors)}>
      <SectionHeader
        colors={colors}
        title="File Manager"
        subtitle="Beginner mode: pick a domain, browse folders, select a file, edit safely, then save."
      />

      <InlineHint colors={colors} tone="warning">
        Avoid editing unknown PHP core files unless required. Start with theme or plugin files first.
      </InlineHint>

      <ActionBar colors={colors} subtle>
        <input
          style={{ ...inputStyle(colors), minWidth: 220 }}
          value={fileDomain}
          onChange={(e) => setFileDomain(e.target.value)}
          placeholder="domain or IP"
        />
        <input
          style={{ ...inputStyle(colors), minWidth: 260 }}
          value={filePath}
          onChange={(e) => setFilePath(e.target.value)}
          placeholder="path"
        />
        <button style={primaryButton(colors)} onClick={() => void listFiles()}>
          Browse
        </button>
      </ActionBar>

      <ActionBar colors={colors} subtle>
        {fileQuickPaths.map((path) => (
          <button
            key={path}
            style={ghostButton(colors)}
            onClick={() => void listFiles(path)}
          >
            {path}
          </button>
        ))}
      </ActionBar>

      <div
        style={{
          border: `1px solid ${colors.borderSoft}`,
          borderRadius: 10,
          padding: "8px 10px",
          background: colors.panelAlt,
          marginBottom: 10,
          display: "flex",
          gap: 6,
          flexWrap: "wrap",
          alignItems: "center",
          fontSize: 13,
        }}
      >
        <span style={{ color: colors.textMuted, fontWeight: 600 }}>Path:</span>
        {fileBreadcrumbs.map((crumb, index) => (
          <React.Fragment key={crumb.path}>
            <button
              style={ghostButton(colors)}
              onClick={() => void listFiles(crumb.path)}
            >
              {crumb.label}
            </button>
            {index < fileBreadcrumbs.length - 1 && (
              <span style={{ color: colors.textMuted }}>/</span>
            )}
          </React.Fragment>
        ))}
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: isCompact
            ? "1fr"
            : "minmax(260px, 1fr) minmax(420px, 2fr)",
          gap: 12,
        }}
      >
        <div style={filePane(colors)}>
          {entries.length === 0 ? (
            <InlineHint colors={colors} tone="info" icon="🧭">
              <strong>No files shown yet.</strong> Enter a domain, set a path
              (for example <code>/public_html</code>), then click Browse.
            </InlineHint>
          ) : (
            entries.map((entry) => (
              <EntryRow
                key={entry.path}
                entry={entry}
                colors={colors}
                ghostButton={ghostButton}
                dangerButton={dangerButton}
                onOpen={(path, isDir) => {
                  if (isDir) void listFiles(path);
                  else void openFile(path);
                }}
                onDelete={(path) => setPendingDeletePath(path)}
              />
            ))
          )}
        </div>

        <div style={filePane(colors)}>
          <div style={{ marginBottom: 6, color: colors.textMuted }}>
            Editing: <code>{selectedPath || "(none selected)"}</code>
          </div>

          {!selectedPath && (
            <InlineHint colors={colors} tone="info" icon="📄" style={{ marginBottom: 8 }}>
              Select a file from the left list to load it here.
            </InlineHint>
          )}

          <textarea
            value={fileContent}
            onChange={(e) => setFileContent(e.target.value)}
            style={{
              width: "100%",
              minHeight: 320,
              borderRadius: 10,
              border: `1px solid ${colors.border}`,
              background: colors.panel,
              color: colors.text,
              padding: 10,
              fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
            }}
            placeholder="File content will appear here after you open a file."
          />

          <ActionBar colors={colors} subtle style={{ marginTop: 8 }}>
            <button
              style={primaryButton(colors)}
              onClick={() => void saveFile()}
              disabled={!selectedPath}
            >
              Save File
            </button>
          </ActionBar>
        </div>
      </div>

      {pendingDeletePath && (
        <ConfirmCard
          colors={colors}
          title="Confirm file delete"
          message="You are about to permanently delete this file."
          targetLabel={pendingDeletePath}
          confirmLabel="Yes, Delete"
          cancelLabel="Cancel"
          tone="danger"
          onConfirm={() => void deletePath(pendingDeletePath)}
          onCancel={() => setPendingDeletePath("")}
        />
      )}
    </section>
  );
}

export default FilesPanel;
