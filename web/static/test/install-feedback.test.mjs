import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { accountStatusLabel } from "../src/utils/install-feedback.mjs";

function loadLocale(name) {
  return JSON.parse(
    readFileSync(new URL(`../../../locales/${name}.json`, import.meta.url), "utf8"),
  );
}

function translate(locale) {
  return (key) => key.split(".").reduce((value, part) => value?.[part], locale);
}

test("localizes valid and invalid account statuses", () => {
  const en = translate(loadLocale("en"));
  const sv = translate(loadLocale("sv"));

  assert.equal(accountStatusLabel("valid", en), "Valid");
  assert.equal(accountStatusLabel("invalid", en), "Invalid");
  assert.equal(accountStatusLabel("valid", sv), "Giltigt");
  assert.equal(accountStatusLabel("invalid", sv), "Ogiltigt");
});

test("preserves unknown account statuses", () => {
  const en = translate(loadLocale("en"));

  assert.equal(accountStatusLabel("  Pending  ", en), "Pending");
  assert.equal(accountStatusLabel(undefined, en), "—");
});
