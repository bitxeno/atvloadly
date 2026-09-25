import assert from "node:assert/strict";
import test from "node:test";

import {
  catalogBuilds,
  devicePlatform,
  filterMatches,
  formatBytes,
  guessSourceKind,
  installLinkFilter,
  listedBuilds,
  platformLabel,
  searchBuilds,
  updateFailed,
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

test("saves a new variant filter of the same source only with the update", () => {
  const sideload = "(?i)(^|[^a-z0-9])sideload([^a-z0-9]|$)";
  const sideloadly = "(?i)(^|[^a-z0-9])sideloadly([^a-z0-9]|$)";
  const source = { kind: "github", url: "prehakanson-art/OrivioTVAppleTV", filter: sideload, prerelease: false, auto_update: false };
  const selection = { kind: "github", url: "Prehakanson-art/OrivioTVAppleTV", filter: sideloadly, prerelease: false };

  // Only the variant changed: nothing to save before the update.
  assert.equal(installLinkFilter(source, selection, false), null);
  // Other settings changed: they are saved with the stored filter.
  assert.equal(installLinkFilter(source, selection, true), sideload);
  assert.equal(installLinkFilter(source, { ...selection, prerelease: true }, false), sideload);
  // A new source (or the first one) is saved with the chosen filter.
  assert.equal(installLinkFilter(source, { ...selection, url: "bobsupra/NuvioTVOS" }, false), sideloadly);
  assert.equal(installLinkFilter({ ...source, kind: "altstore" }, selection, false), sideloadly);
  const untracked = { kind: "", url: "", filter: "", prerelease: false, auto_update: false };
  assert.equal(installLinkFilter(untracked, selection, false), sideloadly);
});

test("detects a failed update of the latest build", () => {
  assert.equal(updateFailed({ failed_build_id: "302", latest_build_id: "302" }), true);
  assert.equal(updateFailed({ failed_build_id: "302", latest_build_id: "303" }), false);
  assert.equal(updateFailed({ failed_build_id: "302", latest_build_id: "" }), false);
  assert.equal(updateFailed({ failed_build_id: "", latest_build_id: "" }), false);
  assert.equal(updateFailed(undefined), false);
});

test("maps device classes to platforms like the server", () => {
  assert.equal(devicePlatform("AppleTV"), "tvos");
  assert.equal(devicePlatform("iPhone"), "ios");
  assert.equal(devicePlatform("iPad"), "ios");
  assert.equal(devicePlatform("appletv"), "");
  assert.equal(devicePlatform(""), "");
  assert.equal(devicePlatform(undefined), "");
});

const nuvioURL = "https://raw.githubusercontent.com/bobsupra/NuvioTVOS/main/apps.json";
const otherURL = "https://example.com/apps.json";
const catalog = [
  {
    id: 1,
    url: nuvioURL,
    name: "bobsupra's NuvioTVOS",
    error: "",
    builds: [
      { id: "a1", name: "Nuvio iOS", platform: "ios" },
      { id: "a2", name: "NuvioTVOS", platform: "tvos" },
      { id: "a3", name: "Nuvio Tools", platform: "" },
    ],
  },
  { id: 2, url: "https://down.example.com/apps.json", name: "Down", error: "HTTP 500", builds: [] },
  {
    id: 3,
    url: otherURL,
    name: "Example",
    error: "",
    builds: [
      { id: "b1", name: "Player", platform: "" },
      { id: "b2", name: "Player TV", platform: "tvos" },
      { id: "b3", name: "Player iPhone", platform: "ios" },
    ],
  },
  { id: 4, url: "https://null.example.com/apps.json", name: "Null", error: "timeout", builds: null },
];
const ids = (builds) => builds.map((b) => b.id);

test("flattens the catalog with the source of each build", () => {
  const builds = catalogBuilds(catalog, "AppleTV");
  assert.deepEqual(builds[0], {
    id: "a2",
    name: "NuvioTVOS",
    platform: "tvos",
    source_url: nuvioURL,
    source_name: "bobsupra's NuvioTVOS",
  });
  assert.equal(builds.find((b) => b.id === "b1").source_url, otherURL);
  assert.equal(builds.find((b) => b.id === "b1").source_name, "Example");
  // The catalog itself is not modified.
  assert.equal(catalog[0].builds[1].source_url, undefined);
});

test("sorts catalog builds for the device platform first", () => {
  assert.deepEqual(ids(catalogBuilds(catalog, "AppleTV")), ["a2", "b2", "a3", "b1", "a1", "b3"]);
  assert.deepEqual(ids(catalogBuilds(catalog, "iPhone")), ["a1", "b3", "a3", "b1", "a2", "b2"]);
  // Unknown devices keep the catalog order.
  assert.deepEqual(ids(catalogBuilds(catalog, "")), ["a1", "a2", "a3", "b1", "b2", "b3"]);
  assert.deepEqual(catalogBuilds([], "AppleTV"), []);
});

test("searches builds by every term in any field", () => {
  const builds = [
    {
      id: "n",
      name: "NuvioTVOS",
      bundle_id: "com.pyksel.nuviotvos",
      developer: "bobsupra",
      subtitle: "Nuvio TV for tvOS",
      version: "3.3.7",
      source_name: "bobsupra's NuvioTVOS",
    },
    { id: "p", name: "Provenance", bundle_id: "org.provenance-emu.provenance", developer: "Provenance Emu", version: "3.0.1" },
    { id: "k", name: "Kodi", bundle_id: "tv.kodi.kodi", subtitle: "Media center", version: "21.1", source_name: "Kodi repo" },
  ];

  assert.deepEqual(ids(searchBuilds(builds, "nuvio")), ["n"]);
  assert.deepEqual(ids(searchBuilds(builds, "  NUVIO   ")), ["n"]);
  assert.deepEqual(ids(searchBuilds(builds, "BobSupra")), ["n"]);
  assert.deepEqual(ids(searchBuilds(builds, "pyksel")), ["n"]);
  assert.deepEqual(ids(searchBuilds(builds, "3.0")), ["p"]);
  assert.deepEqual(ids(searchBuilds(builds, "media")), ["k"]);
  assert.deepEqual(ids(searchBuilds(builds, "repo")), ["k"]);
  // Terms may match different fields, but every term must match.
  assert.deepEqual(ids(searchBuilds(builds, "tvos 3.3")), ["n"]);
  assert.deepEqual(ids(searchBuilds(builds, "kodi 3.3")), []);
  // A term does not match across two fields.
  assert.deepEqual(ids(searchBuilds(builds, "kodi21")), []);
  assert.deepEqual(ids(searchBuilds(builds, "tv")), ["n", "k"]);
  assert.deepEqual(searchBuilds(builds, "zzz"), []);
});

test("returns every build for an empty search", () => {
  const builds = [{ id: "b" }, { id: "a" }];
  assert.deepEqual(ids(searchBuilds(builds, "")), ["b", "a"]);
  assert.deepEqual(ids(searchBuilds(builds, "   ")), ["b", "a"]);
  assert.deepEqual(ids(searchBuilds(builds, undefined)), ["b", "a"]);
});

test("lists the pinned build first when it is beyond the cap", () => {
  const builds = ["a", "b", "c", "d"].map((id) => ({ id }));
  const pin = (id) => (b) => b.id === id;
  assert.deepEqual(ids(listedBuilds(builds, 2, pin(""))), ["a", "b"]);
  // A pinned build within the cap keeps its place.
  assert.deepEqual(ids(listedBuilds(builds, 2, pin("b"))), ["a", "b"]);
  assert.deepEqual(ids(listedBuilds(builds, 2, pin("d"))), ["d", "a", "b"]);
  assert.deepEqual(ids(listedBuilds(builds, 4, pin("d"))), ["a", "b", "c", "d"]);
  assert.deepEqual(ids(listedBuilds(builds, 9, pin("z"))), ["a", "b", "c", "d"]);
  assert.deepEqual(listedBuilds([], 2, pin("a")), []);
});
