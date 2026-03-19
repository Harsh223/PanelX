export function normalizeInput(value: string): string {
  return value.trim();
}

export function normalizeHostname(value: string): string {
  return normalizeInput(value).toLowerCase();
}

export function isValidEmail(value: string): boolean {
  const email = normalizeInput(value);
  if (!email) return false;

  // Practical validation for UI forms (not full RFC 5322 parsing).
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

export function isValidIPv4(value: string): boolean {
  const input = normalizeHostname(value);
  if (!input) return false;

  const ipv4 =
    /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

  return ipv4.test(input);
}

export function isValidHostname(value: string): boolean {
  const host = normalizeHostname(value);
  if (!host || host.includes(" ")) return false;
  if (host.length > 253) return false;
  if (host.endsWith(".")) return false;

  const labels = host.split(".");
  if (labels.length < 2) return false;

  // TLD should be alphabetic and reasonably bounded.
  const tld = labels[labels.length - 1];
  if (!/^[a-z]{2,63}$/.test(tld)) return false;

  for (const label of labels) {
    if (!label || label.length > 63) return false;
    if (!/^[a-z0-9-]+$/.test(label)) return false;
    if (label.startsWith("-") || label.endsWith("-")) return false;
  }

  return true;
}

export function isLikelyDomainOrIP(value: string): boolean {
  const input = normalizeHostname(value);
  if (!input || input.includes(" ")) return false;

  return isValidIPv4(input) || isValidHostname(input);
}

export function validateDomainOrIP(value: string): {
  valid: boolean;
  reason?: string;
} {
  const input = normalizeInput(value);

  if (!input) {
    return { valid: false, reason: "Domain or IP is required." };
  }

  if (!isLikelyDomainOrIP(input)) {
    return {
      valid: false,
      reason: "Enter a valid domain (example.com) or IPv4 address (203.0.113.10).",
    };
  }

  return { valid: true };
}
