import assert from "node:assert/strict";
import test from "node:test";

import {
  filterMatches,
  formatBytes,
  guessSourceKind,
  platformLabel,
} from "../src/utils/source.mjs";

test("guesses GitHub repositories", () => {
  for (const input of [
    "prehakanson-art/OrivioTVAppleTV",
    " bobsupra/NuvioTVOS/ ",
    "github.com/bobsupra/NuvioTVOS",
    "https://github.com/prehakanson-art/OrivioTVAppleTV",
    "https://www.github.com/owner/repo.git",
    "https://github.com/prehakanson-art/OrivioTVAppleTV/releases/tag/v0.10",
  ]) {
    assert.equal(guessSourceKind(input), "github", input);
  }
});

test("guesses AltStore sources", () => {
  for (const input of [
    "https://raw.githubusercontent.com/bobsupra/NuvioTVOS/main/apps.json",
    "https://github.com/bobsupra/NuvioTVOS/raw/main/apps.json",
    "https://example.com/source",
    "http://192.168.1.2:8080/apps.json",
  ]) {
    assert.equal(guessSourceKind(input), "altstore", input);
  }
});

test("does not guess a source for IPA links and other input", () => {
  for (const input of [
    "",
    undefined,
    "hello",
    "https://github.com/owner",
    "https://github.com/bobsupra/NuvioTVOS/releases/download/tvos-beta-3.3.7/NuvioTV-3.3.7-unsigned-release.ipa",
    "https://example.com/App.tipa",
    "example.com/apps.json",
    "ftp://example.com/apps.json",
  ]) {
    assert.equal(guessSourceKind(input), "", String(input));
  }
});

test("formats sizes with decimal units", () => {
  assert.equal(formatBytes(52100000), "52.1 MB");
  assert.equal(formatBytes(23972092), "24.0 MB");
  assert.equal(formatBytes(150000), "150 KB");
  assert.equal(formatBytes(1500000000), "1.5 GB");
  assert.equal(formatBytes(999), "999 B");
  assert.equal(formatBytes("2048"), "2.0 KB");
  assert.equal(formatBytes(0), "");
  assert.equal(formatBytes(undefined), "");
});

test("labels platform hints", () => {
  assert.equal(platformLabel("tvos"), "tvOS");
  assert.equal(platformLabel("ios"), "iOS");
  assert.equal(platformLabel(""), "");
  assert.equal(platformLabel(undefined), "");
});

test("matches GitHub filters derived by the server", () => {
  const sideload = "(?i)(^|[^a-z0-9])sideload([^a-z0-9]|$)";
  const sideloadly = "(?i)(^|[^a-z0-9])sideloadly([^a-z0-9]|$)";
  const build = (name) => ({ name });

  assert.equal(filterMatches("github", sideload, build("OrivioTV-V9.Sideload.ipa")), true);
  assert.equal(filterMatches("github", sideload, build("OrivioTV-V9.Sideloadly.ipa")), false);
  assert.equal(filterMatches("github", sideloadly, build("OrivioTV-0.7.15-sideloadly-unsigned.ipa")), true);
  assert.equal(filterMatches("github", "(?i)^App\\.ipa$", build("app.ipa")), true);
  assert.equal(filterMatches("github", "^App\\.ipa$", build("app.ipa")), false);
  assert.equal(filterMatches("github", "(", build("app.ipa")), false);
  assert.equal(filterMatches("github", "", build("app.ipa")), false);
});

test("matches AltStore filters by bundle identifier", () => {
  const build = { name: "NuvioTVOS", bundle_id: "com.pyksel.nuviotvos" };

  assert.equal(filterMatches("altstore", "com.pyksel.nuviotvos", build), true);
  assert.equal(filterMatches("altstore", "com.example.other", build), false);
});
