import assert from "node:assert/strict";
import test from "node:test";
import { createServer } from "vite";

test("the selected UI language survives a change to the generic i18next cache", async (t) => {
  const values = new Map();
  globalThis.window = {
    location: new URL("http://localhost/#/home"),
    localStorage: {
      getItem: (key) => values.get(key) ?? null,
      setItem: (key, value) => values.set(key, value),
      removeItem: (key) => values.delete(key),
    },
  };
  t.after(() => { delete globalThis.window; });

  const vite = await createServer({ server: { middlewareMode: true, hmr: false } });
  t.after(() => vite.close());
  const { default: i18n } = await vite.ssrLoadModule("/src/i18n.js");

  for (const lang of ["zh-CN", "sv"]) {
    await i18n.changeLanguage(lang);
    window.localStorage.setItem("i18nextLng", "zh");
    // Run the same detection that i18next uses on a fresh page load.
    await i18n.changeLanguage();
    assert.equal(i18n.resolvedLanguage, lang);
  }

  window.location.search = "?lng=zh";
  await i18n.changeLanguage();
  assert.equal(i18n.resolvedLanguage, "zh-CN");
});
