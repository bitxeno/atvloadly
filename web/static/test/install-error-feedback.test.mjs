import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  installFailureMessage,
  takePendingCertificateReset,
} from "../src/utils/install-error-feedback.mjs";

function loadLocale(name) {
  return JSON.parse(
    readFileSync(new URL(`../../../locales/${name}.json`, import.meta.url), "utf8"),
  );
}

function translate(locale) {
  return (key) => key.split(".").reduce((value, part) => value?.[part], locale);
}

test("explains an unavailable App Group in every supported locale", () => {
  const unavailableGroup = [
    "Developer API error 35:",
    "An Application Group with Identifier 'group.example.app.TEAMID' is not available.",
  ].join("\n");

  for (const localeName of ["en", "sv", "zh_cn"]) {
    const message = translate(loadLocale(localeName));
    assert.equal(
      installFailureMessage(unavailableGroup, message),
      message("install.toast.app_group_unavailable"),
    );
  }
});

test("offers only pending refreshes that stopped at the certificate limit", () => {
  const apps = [
    { ID: 1, refreshed_error: 0 },
    { ID: 2, refreshed_error: 5 },
    { ID: 3, refreshed_error: 1 },
    { ID: 4, refreshed_error: 5 },
  ];
  const pending = { 1: true, 2: true, 3: true, 4: true, 9: true };

  const first = takePendingCertificateReset(apps, pending);
  assert.equal(first.ID, 2);
  assert.deepEqual(pending, { 4: true, 9: true });

  const second = takePendingCertificateReset(apps, pending);
  assert.equal(second.ID, 4);
  assert.deepEqual(pending, { 9: true });

  assert.equal(takePendingCertificateReset(apps, pending), null);
  assert.deepEqual(pending, { 9: true });
});

test("retains the generic failure message for unrelated errors", () => {
  const message = translate(loadLocale("en"));

  assert.equal(
    installFailureMessage("Installation failed with exit status 1", message),
    message("install.toast.install_failed"),
  );
});
