export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";

  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const idx = Math.min(
    units.length - 1,
    Math.floor(Math.log(bytes) / Math.log(1024)),
  );
  const value = bytes / Math.pow(1024, idx);

  return `${value.toFixed(value >= 100 || idx === 0 ? 0 : 1)} ${units[idx]}`;
}

export function timeAgo(value?: string): string {
  if (!value) return "unknown";

  const ts = Date.parse(value);
  if (Number.isNaN(ts)) return value;

  const diffSeconds = Math.max(0, Math.floor((Date.now() - ts) / 1000));

  if (diffSeconds < 60) return `${diffSeconds}s ago`;
  if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)}m ago`;
  if (diffSeconds < 86400) return `${Math.floor(diffSeconds / 3600)}h ago`;
  return `${Math.floor(diffSeconds / 86400)}d ago`;
}

export function formatDateTime(value?: string): string {
  if (!value) return "-";

  const ts = Date.parse(value);
  if (Number.isNaN(ts)) return value;

  return new Date(ts).toLocaleString();
}

export function clampNumber(
  value: number,
  min: number,
  max: number,
  fallback: number,
): number {
  if (!Number.isFinite(value)) return fallback;
  return Math.min(max, Math.max(min, value));
}

const format = {
  formatBytes,
  timeAgo,
  formatDateTime,
  clampNumber,
};

export default format;
