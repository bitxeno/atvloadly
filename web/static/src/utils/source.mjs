// Helpers for apps tracked against a GitHub repository or an AltStore source.

const ownerRepoPattern = /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9._-]+\/?$/;
const githubHosts = new Set(["github.com", "www.github.com"]);

// guessSourceKind returns "github" for a GitHub repository (owner/repo or a
// github.com URL), "altstore" for any other http(s) URL, and "" otherwise.
// Direct IPA links are not sources and return "".
export function guessSourceKind(url) {
  const value = String(url ?? "").trim();
  if (ownerRepoPattern.test(value)) {
    return "github";
  }

  let parsed;
  try {
    parsed = new URL(/^[a-z][a-z0-9+.-]*:\/\//i.test(value) ? value : `https://${value}`);
  } catch {
    return "";
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return "";
  }

  const path = parsed.pathname.toLowerCase();
  if (path.endsWith(".ipa") || path.endsWith(".tipa")) {
    return "";
  }
  if (githubHosts.has(parsed.hostname.toLowerCase()) && !path.endsWith(".json")) {
    return parsed.pathname.split("/").filter(Boolean).length >= 2 ? "github" : "";
  }
  // Without a scheme only owner/repo and github.com locations are recognized.
  return /^https?:\/\//i.test(value) ? "altstore" : "";
}

const byteUnits = ["KB", "MB", "GB"];

// formatBytes renders a size with decimal units, e.g. 52100000 -> "52.1 MB".
export function formatBytes(n) {
  let value = Number(n);
  if (!Number.isFinite(value) || value <= 0) {
    return "";
  }
  if (value < 1000) {
    return `${value} B`;
  }
  let unit = "";
  for (const u of byteUnits) {
    value /= 1000;
    unit = u;
    if (value < 1000) {
      break;
    }
  }
  return `${value >= 100 ? value.toFixed(0) : value.toFixed(1)} ${unit}`;
}

const platformLabels = { tvos: "tvOS", ios: "iOS" };

// platformLabel returns the display name of a build platform hint.
export function platformLabel(platform) {
  return platformLabels[platform] || "";
}

// filterMatches mirrors the server check that a tracking filter selects a
// build: a case-insensitive asset name regexp for GitHub, the bundle
// identifier for AltStore. Filters JavaScript cannot compile do not match.
export function filterMatches(kind, filter, build) {
  if (!filter || !build) {
    return false;
  }
  if (kind === "altstore") {
    return filter === build.bundle_id;
  }
  const insensitive = filter.startsWith("(?i)");
  try {
    return new RegExp(insensitive ? filter.slice(4) : filter, insensitive ? "i" : "").test(build.name);
  } catch {
    return false;
  }
}
