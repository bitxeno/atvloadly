import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
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


const serbianCyrillicToLatin = {
  а: "a", б: "b", в: "v", г: "g", д: "d", ђ: "đ", е: "e",
  ж: "ž", з: "z", и: "i", ј: "j", к: "k", л: "l", љ: "lj",
  м: "m", н: "n", њ: "nj", о: "o", п: "p", р: "r", с: "s",
  т: "t", ћ: "ć", у: "u", ф: "f", х: "h", ц: "c", ч: "č",
  џ: "dž", ш: "š",
};

function transliterateSerbian(message) {
  return [...message].map((letter) => {
    const latin = serbianCyrillicToLatin[letter.toLowerCase()];
    if (!latin) return letter;
    return letter === letter.toUpperCase()
      ? latin[0].toUpperCase() + latin.slice(1)
      : latin;
  }).join("");
}

test("Serbian Cyrillic and Latin translations remain equivalent", () => {
  const cyrillic = flattenLocale(loadLocale("sr"));
  const latin = flattenLocale(loadLocale("sr-Latn"));

  for (const [key, message] of cyrillic) {
    if (key === "language") continue; // The locale identifiers differ by design.
    assert.equal(
      transliterateSerbian(message),
      latin.get(key),
      `Serbian script variants diverge at ${key}`,
    );
  }
});

test("README pages link all supported languages and both logo themes", () => {
  const readmes = [
    ["README.md", "English"],
    ["docs/readme/README_sv.md", "Svenska"],
    ["docs/readme/README_cn.md", "中文"],
    ["docs/readme/README_sr.md", "Српски (ћирилица)"],
    ["docs/readme/README_sr_Latn.md", "Srpski (latinica)"],
  ];
  const english = readFileSync(new URL("../../../README.md", import.meta.url), "utf8");
  const headingCount = (document) => [...document.matchAll(/^#{2,3} /gm)].length;

  for (const [file, label] of readmes) {
    const document = readFileSync(new URL(`../../../${file}`, import.meta.url), "utf8");
    const navigation = [...document.matchAll(/<div align="center">([\s\S]*?)<\/div>/g)].at(-1)?.[1];
    assert.ok(navigation, `${file} is missing the language menu`);
    assert.ok(navigation.includes(label), `${file} has no own language label`);
    for (const [target, name] of readmes) {
      if (target !== file) {
        assert.ok(
          navigation.includes(`[${name}](`),
          `${file} does not link to ${target}`,
        );
      }
    }
    for (const [, destination] of document.matchAll(/\[[^\]]+\]\((\.[^\s)]+)\)/g)) {
      assert.ok(
        existsSync(new URL(destination, new URL(`../../../${file}`, import.meta.url))),
        `${file} has a broken relative link to ${destination}`,
      );
    }
    assert.ok(document.includes('srcset="../preview/logo_dark.svg"') || document.includes('srcset="./docs/preview/logo_dark.svg"'), `${file}: dark logo missing`);
    assert.ok(document.includes('src="../preview/logo.svg"') || document.includes('src="./docs/preview/logo.svg"'), `${file}: light logo missing`);
    if (file.startsWith("docs/readme/README_sr")) {
      assert.equal(headingCount(document), headingCount(english), `${file}: incomplete sections`);
    }
  }
});

test("Swedish distinguishes signing renewal from version updates", () => {
  const sv = loadLocale("sv");
  assert.equal(sv.home.table.button.refresh, "Förnya");
  assert.equal(sv.home.table.button.update, "Uppdatera");
  assert.equal(sv.settings.refresh.title, "Automatisk appförnyelse");
  // Other UI refresh actions do not renew signing and keep their original label.
  assert.equal(sv.install.screenshot.button.refresh, "Uppdatera");
});
