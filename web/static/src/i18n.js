import i18next from "i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import en from "@locales/en.json";
import zh_cn from "@locales/zh_cn.json";

i18next.use(LanguageDetector).init({
  //   debug: true,
  fallbackLng: "en",
  resources: {
    en: {
      name: "English",
      translation: en,
    },
    "zh-CN": {
      name: "中文",
      translation: zh_cn,
    },
  },
});

export default i18next;
