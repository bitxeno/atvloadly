import assert from "node:assert/strict";
import test from "node:test";

import {
  currentBundleURL,
  isStaleAssetLoadError,
  recoverFromStaleAssetLoadError,
} from "../src/utils/stale-asset-recovery.mjs";

function storage() {
  const values = new Map();
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
  };
}

test("recognizes stale dynamic import and preload errors", () => {
  assert.equal(
    isStaleAssetLoadError(new Error("Failed to fetch dynamically imported module")),
    true,
  );
  assert.equal(isStaleAssetLoadError("Unable to preload CSS for /assets/app.css"), true);
  assert.equal(isStaleAssetLoadError(new Error("Installation failed")), false);
});

test("uses the current module script as the deployment-specific recovery token", () => {
  const document = {
    querySelector: () => ({ src: "http://example.test/assets/index.current.js" }),
  };

  assert.equal(
    currentBundleURL(document, { href: "http://example.test/#/laboratory" }),
    "http://example.test/assets/index.current.js",
  );
  assert.equal(
    currentBundleURL({ querySelector: () => null }, { href: "http://example.test/#/laboratory" }),
    "http://example.test/#/laboratory",
  );
});

test("reloads once for each frontend bundle after a stale asset failure", () => {
  const sessionStorage = storage();
  let reloads = 0;
  const staleImport = new Error("Importing a module script failed.");

  assert.equal(
    recoverFromStaleAssetLoadError(
      staleImport,
      "http://example.test/assets/index.old.js",
      sessionStorage,
      () => reloads++,
    ),
    true,
  );
  assert.equal(reloads, 1);
  assert.equal(
    recoverFromStaleAssetLoadError(
      staleImport,
      "http://example.test/assets/index.old.js",
      sessionStorage,
      () => reloads++,
    ),
    false,
  );
  assert.equal(reloads, 1);
  assert.equal(
    recoverFromStaleAssetLoadError(
      staleImport,
      "http://example.test/assets/index.new.js",
      sessionStorage,
      () => reloads++,
    ),
    true,
  );
  assert.equal(reloads, 2);
  assert.equal(
    recoverFromStaleAssetLoadError(staleImport, undefined, sessionStorage, () => reloads++),
    false,
  );
  assert.equal(reloads, 2);
});
