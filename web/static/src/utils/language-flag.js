import countries from "flag-icons/country.json";

const availableFlags = new Set(countries.map(({ code }) => code));

// Behåll förväntade flaggor för språk utan angiven region.
const defaults = {
  en: "gb",
  sv: "se",
  zh: "cn",
  pt: "pt",
  no: "no",
  nb: "no",
  nn: "no",
};

// Dessa språk har ingen självklar enskild landsflagga.
const withoutDefault = new Set(["ar", "eo", "ia", "ie", "ku", "mul", "pa", "und", "yi", "zxx"]);

export function languageFlagCode(language) {
  if (typeof language !== "string" || !language.trim()) return null;

  try {
    const locale = new Intl.Locale(language.replace(/_/g, "-"));
    const base = locale.language.toLowerCase();

    // En uttrycklig region, t.ex. pt-BR, går före språkets standardflagga.
    if (locale.region) {
      const explicit = locale.region.toLowerCase();
      if (availableFlags.has(explicit)) return explicit;
    }

    if (defaults[base]) return defaults[base];
    if (withoutDefault.has(base)) return null;

    // Språk utan uttrycklig region får CLDR:s vanligaste region.
    const likely = locale.maximize().region?.toLowerCase();
    return likely && availableFlags.has(likely) ? likely : null;
  } catch {
    return null;
  }
}
