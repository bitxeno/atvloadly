import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import test from "node:test";

const localeDirectory = new URL("../../../locales/", import.meta.url);

function loadLocale(name) {
  return JSON.parse(
    readFileSync(new URL(`${name}.json`, localeDirectory), "utf8"),
  );
}

function localeNames() {
  return readdirSync(localeDirectory, { withFileTypes: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".json"))
    .map((entry) => entry.name.slice(0, -".json".length))
    .sort();
}

function flattenLocale(value, path = "", leaves = new Map()) {
  assert.equal(Array.isArray(value), false, `${path || "locale"} must not be an array`);

  if (typeof value === "string") {
    leaves.set(path, value);
    return leaves;
  }

  assert.equal(typeof value, "object", `${path || "locale"} must be an object or string`);
  assert.notEqual(value, null, `${path || "locale"} must not be null`);

  for (const [key, child] of Object.entries(value)) {
    flattenLocale(child, path ? `${path}.${key}` : key, leaves);
  }
  return leaves;
}

function placeholders(message) {
  return [...message.matchAll(/{{\s*\.?\s*([A-Za-z_][\w]*)\s*}}/g)]
    .map((match) => match[1])
    .sort();
}

test("every bundled locale matches the English key and placeholder contract", () => {
  const names = localeNames();
  assert.ok(names.includes("en"), "locales/en.json is required as the canonical locale");

  const english = flattenLocale(loadLocale("en"));
  const englishKeys = [...english.keys()].sort();

  for (const localeName of names.filter((name) => name !== "en")) {
    const locale = flattenLocale(loadLocale(localeName));
    const localeKeys = [...locale.keys()].sort();
    const missingKeys = englishKeys.filter((key) => !locale.has(key));
    const extraKeys = localeKeys.filter((key) => !english.has(key));

    assert.deepEqual(missingKeys, [], `${localeName} is missing locale key(s): ${missingKeys.join(", ")}`);
    assert.deepEqual(
      extraKeys,
      [],
      `${localeName} has obsolete or unsupported locale key(s): ${extraKeys.join(", ")}`,
    );

    for (const [key, englishMessage] of english) {
      assert.deepEqual(
        placeholders(locale.get(key)),
        placeholders(englishMessage),
        `${localeName}.${key} has incompatible placeholders`,
      );
    }
  }
});
