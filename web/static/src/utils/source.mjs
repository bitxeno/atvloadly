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

// installLinkFilter returns the filter to save before the source dialog
// installs selection on an app linked to source, or null when the link needs
// no change. A new filter for the same source is only saved by the update
// once it installs, so a failed switch to another variant (maybe another app)
// keeps tracking the installed one.
export function installLinkFilter(source, selection, autoUpdate) {
  const sameSource =
    !!source.kind && selection.kind === source.kind && selection.url.toLowerCase() === source.url.toLowerCase();
  if (!sameSource) {
    return selection.filter;
  }
  if (selection.prerelease !== source.prerelease || autoUpdate !== source.auto_update) {
    return source.filter;
  }
  return null;
}

// updateFailed reports whether installing the latest build of source failed.
// Such a build is not installed automatically again.
export function updateFailed(source) {
  return !!source?.failed_build_id && source.failed_build_id === source.latest_build_id;
}
