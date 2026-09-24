<template>
  <div class="app-container">
    <header class="navbar border-b pt-0 pb-0">
      <div class="flex-1">
        <a href="/" class="btn btn-ghost rounded-btn atv-brand">
          <AppIcon class="w-7 h-7" />
          <span class="normal-case text-xl">atvloadly</span>
        </a>
      </div>
      <div class="flex-none">
        <nav class="navbar w-full"> 
          <div class="dropdown" :class="{ 'dropdown-hover': !isMobile, 'dropdown-open': isMobile && openMobileMenu === 'preferences' }" @click="closeMobileMenu">
            <label tabindex="0" class="btn btn-ghost rounded-btn px-2 md:px-4"
              @click.stop="toggleMobileMenu('preferences')"
              @keydown.enter.prevent="toggleMobileMenu('preferences')"
              @keydown.space.prevent="toggleMobileMenu('preferences')"
              :aria-expanded="isMobile ? openMobileMenu === 'preferences' : undefined"
              :aria-label="$t('nav.preferences')">
              <span class="w-5">
                  <OptionIcon />
                </span>
              <span class="hidden sm:inline">{{ $t("nav.preferences") }}</span></label
            >
            <ul
              tabindex="0"
              class="dropdown-content z-[1] menu menu-sm p-2 shadow bg-base-200 rounded-box w-36 gap-1"
            >
              <li>
                <router-link :to="{ name: 'account' }">
                  <span class="w-5">
                  <AccountIcon />
                </span>
                  {{ $t("nav.account") }}
                  </router-link>
              </li>
              <li>
                <router-link :to="{ name: 'tools' }">
                    <span class="w-5">
                      <ToolsIcon />
                    </span>
                    {{ $t("nav.tools") }}
                </router-link>
              </li>
              <li>
                <router-link :to="{ name: 'laboratory' }">
                    <span class="w-5">
                      <FlaskIcon />
                    </span>
                    {{ $t("nav.laboratory") }}
                </router-link>
              </li>
              <li>
                <router-link :to="{ name: 'settings' }">
                  <span class="w-5">
                  <SettingsIcon />
                </span>
                  {{ $t("nav.settings") }}
                </router-link>
              </li>
              <li>
                <button @click="showDonateModal = true">
                  <span class="w-5">
                    <DonateThumbIcon />
                  </span>
                  {{ $t("nav.donate") }}
                </button>
              </li>
              <li>
                <button @click="showAboutModal = true">
                  <span class="inline-flex h-5 w-5 items-center justify-center">
                    <HelpIcon class="h-5 w-5" />
                  </span>
                  {{ $t("nav.about") }}
                </button>
              </li>
            </ul>
          </div>

          <div class="dropdown" :class="{ 'dropdown-hover': !isMobile, 'dropdown-open': isMobile && openMobileMenu === 'language' }" @click="closeMobileMenu">
            <label tabindex="0" class="btn btn-ghost rounded-btn px-2 md:px-4"
              @click.stop="toggleMobileMenu('language')"
              @keydown.enter.prevent="toggleMobileMenu('language')"
              @keydown.space.prevent="toggleMobileMenu('language')"
              :aria-expanded="isMobile ? openMobileMenu === 'language' : undefined"
              :aria-label="$t('nav.language')">
              <span class="w-5">
                <WorldIcon />
              </span>
              <span class="hidden sm:inline">{{ $t("nav.language") }}</span></label
            >
            <ul
              tabindex="0"
              class="dropdown-content z-[1] menu menu-sm p-2 shadow bg-base-200 rounded-box w-36 gap-1"
            >
              <li v-for="item in languages" :key="item.key">
                <button
                  :class="[{ active: $i18next.language == item.key }]"
                  v-on:click="changeLanguage(item.key)"
                >
                  {{ item.name }}
                </button>
              </li>
            </ul>
          </div>

<div class="dropdown" :class="{ 'dropdown-hover': !isMobile, 'dropdown-open': isMobile && openMobileMenu === 'theme' }" @click="closeMobileMenu">
            <label tabindex="0" class="btn btn-ghost rounded-btn px-2 md:px-4"
              :aria-label="$t('nav.theme')"
              @click.stop="toggleMobileMenu('theme')"
              @keydown.enter.prevent="toggleMobileMenu('theme')"
              @keydown.space.prevent="toggleMobileMenu('theme')"
              :aria-expanded="isMobile ? openMobileMenu === 'theme' : undefined">
              <span class="atv-theme-glyph" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M12 3 A9 9 0 0 0 12 21 Z" fill="currentColor"/><circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="2.4"/></svg></span>
              <span class="hidden sm:inline">{{ $t("nav.theme") }}</span>
            </label>
            <ul tabindex="0"
              class="dropdown-content z-[1] menu menu-sm p-2 shadow bg-base-200 rounded-box w-44 gap-1">
              <li v-for="mode in ['system', 'light', 'dark']" :key="mode">
                <button type="button"
                  :class="{ active: themePreference === mode }"
                  :aria-pressed="themePreference === mode"
                  @click="changeTheme(mode)">
                  {{ $t('nav.theme_' + mode) }}
                </button>
              </li>
            </ul>
          </div>

          <div>
            <a href="https://github.com/bitxeno/atvloadly" target="_blank">
              <label tabindex="0" class="btn btn-ghost rounded-btn">
                <GithubIcon class="w-5 h-5" /></label>
            </a>
          </div>
        </nav>
      </div>
    </header>

    <div class="main-container">
      <router-view />
    </div>

    <!-- Donate Modal -->
    <div v-if="showDonateModal" class="modal modal-open modal-middle" @click.self="showDonateModal = false">
      <div class="modal-box relative">
        <label @click="showDonateModal = false" class="btn btn-sm btn-circle absolute right-2 top-2">✕</label>
        <h3 class="font-bold text-lg flex items-center gap-2">
          <LikeIcon class="w-6 h-6 text-red-500" />
          {{ $t("donate.title") }}
        </h3>
        <p class="py-4 text-base-content/70">
          {{ $t("donate.desc") }}
        </p>
        <div class="flex flex-col gap-3 w-full mt-2">
           <a href="https://github.com/sponsors/bitxeno" target="_blank" class="btn border-none w-full normal-case text-lg gap-2 text-white hover:opacity-90" style="background-color: #ea4aaa;">
             <GithubIcon class="w-6 h-6" />
             {{ $t("donate.github_sponsor") }}
           </a>
           <a href="https://ko-fi.com/bitxeno" target="_blank" class="btn border-none w-full normal-case text-lg gap-2 text-white hover:opacity-90" style="background-color: #29abe0;">
             <KofiIcon class="w-6 h-6" />
             {{ $t("donate.kofi") }}
           </a>
           <a href="https://ifdian.net/a/bitxeno" target="_blank" class="btn border-none w-full normal-case text-lg gap-2 text-white hover:opacity-90" style="background-color: #946ce6;">
             <AfdianIcon class="w-6 h-6" />
             {{ $t("donate.afdian") }}
           </a>
        </div>
      </div>
    </div>

    <div v-if="showAboutModal" class="modal modal-open modal-middle" @click.self="showAboutModal = false">
      <div class="modal-box relative overflow-hidden border border-base-300 bg-gradient-to-br from-base-100 via-base-100 to-info/10 shadow-2xl">
        <label @click="showAboutModal = false" class="btn btn-sm btn-circle absolute right-2 top-2">✕</label>
        <div class="absolute inset-0 pointer-events-none opacity-60">
          <div class="absolute -top-12 -left-10 h-40 w-40 rounded-full bg-info/20 blur-3xl"></div>
          <div class="absolute -bottom-10 -right-6 h-36 w-36 rounded-full bg-secondary/20 blur-3xl"></div>
        </div>
        <div class="relative flex flex-col items-center text-center gap-5 py-6">
          <div class="flex h-20 w-20 items-center justify-center rounded-3xl bg-base-100/80 shadow-lg ring-1 ring-base-300 backdrop-blur">
            <AppIcon class="w-12 h-12" />
          </div>
          <div class="space-y-2">
            <h3 class="font-bold text-3xl tracking-tight">atvloadly</h3>
          </div>
          <div class="grid w-full max-w-md grid-cols-2 gap-3 pt-2">
            <div class="rounded-2xl border border-base-300 bg-base-100/80 px-4 py-3 shadow-sm backdrop-blur">
              <div class="text-xs uppercase tracking-wide text-base-content/50">{{ $t("about.version") }}</div>
              <div class="mt-1 text-lg font-semibold">{{ appVersion }}</div>
            </div>
            <div class="rounded-2xl border border-base-300 bg-base-100/80 px-4 py-3 shadow-sm backdrop-blur">
              <div class="text-xs uppercase tracking-wide text-base-content/50">{{ $t("about.build_date") }}</div>
              <div class="mt-1 text-lg font-semibold">{{ buildDate }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
  
  <script>
import api from "@/api/api";
export default {
  name: "App",
  data() {
    return {
      languages: [],
      isMobile: window.matchMedia("(max-width: 767px)").matches,
      mobileMediaQuery: null,
      openMobileMenu: null,
      themePreference: "light",
      colorSchemeQuery: null,
      showDonateModal: false,
      showAboutModal: false,
      appVersion: "unknown",
      buildDate: "unknown",
    };
  },
  created() {
    const saved = window.localStorage.getItem("atvloadly-theme");
    this.themePreference = ["system", "light", "dark"].includes(saved) ? saved : "light";
    api.syncLang({lang: this.$i18next.language})
    api.getVersion().then((res) => {
      this.appVersion = res.data?.version || "unknown";
      this.buildDate = this.formatBuildDate(res.data?.build_date);
    });
    let keys = Object.keys(this.$i18next.options.resources);
    for (const key of keys) {
      this.languages.push({
        key: key,
        name: this.$i18next.options.resources[key].name,
      });
    }
  },
  mounted() {
    this.mobileMediaQuery = window.matchMedia("(max-width: 767px)");
    this.mobileMediaQuery.addEventListener("change", this.onMobileBreakpointChange);
    document.addEventListener("click", this.onDocumentClick);
    document.addEventListener("keydown", this.onDocumentKeydown);
    this.colorSchemeQuery = window.matchMedia("(prefers-color-scheme: dark)");
    this.colorSchemeQuery.addEventListener("change", this.applyTheme);
    this.applyTheme();
  },
  beforeUnmount() {
    this.colorSchemeQuery?.removeEventListener("change", this.applyTheme);
    this.mobileMediaQuery?.removeEventListener("change", this.onMobileBreakpointChange);
    document.removeEventListener("click", this.onDocumentClick);
    document.removeEventListener("keydown", this.onDocumentKeydown);
  },
  methods: {
    onMobileBreakpointChange(event) {
      this.isMobile = event.matches;
      this.openMobileMenu = null;
    },
    toggleMobileMenu(menu) {
      if (!this.isMobile) return;
      if (this.openMobileMenu === menu) {
        this.closeMobileMenu();
      } else {
        this.openMobileMenu = menu;
      }
    },
    closeMobileMenu() {
      if (!this.isMobile) return;
      this.openMobileMenu = null;
      const focused = document.activeElement;
      if (focused?.closest?.(".app-container > .navbar .dropdown")) {
        focused.blur();
      }
    },
    onDocumentClick(event) {
      if (this.isMobile &&
          !event.target?.closest?.(".app-container > .navbar .dropdown")) {
        this.closeMobileMenu();
      }
    },
    onDocumentKeydown(event) {
      if (this.isMobile && event.key === "Escape") {
        this.closeMobileMenu();
      }
    },
    applyTheme() {
      const dark = this.themePreference === "dark" ||
        (this.themePreference === "system" && this.colorSchemeQuery?.matches);
      document.documentElement.setAttribute("data-theme", dark ? "dark" : "winter");
    },
    changeTheme(mode) {
      if (!["system", "light", "dark"].includes(mode)) return;
      this.themePreference = mode;
      window.localStorage.setItem("atvloadly-theme", mode);
      this.applyTheme();
    },
    formatBuildDate(buildDate) {
      if (!buildDate) {
        return "unknown";
      }

      const text = String(buildDate);
      const matched = text.match(/^(\d{4})[-/](\d{1,2})[-/](\d{1,2})/);
      if (matched) {
        return `${matched[1]}-${matched[2].padStart(2, "0")}-${matched[3].padStart(2, "0")}`;
      }

      const monthOnly = text.match(/^(\d{4})[-/](\d{2})/);
      if (monthOnly) {
        return `${monthOnly[1]}-${monthOnly[2]}`;
      }

      const parsed = new Date(buildDate);
      if (Number.isNaN(parsed.getTime())) {
        return buildDate;
      }

      const year = parsed.getFullYear();
      const month = String(parsed.getMonth() + 1).padStart(2, "0");
      const day = String(parsed.getDate()).padStart(2, "0");
      return `${year}-${month}-${day}`;
    },
    changeLanguage(lang) {
      this.$i18next.changeLanguage(lang);
      api.syncLang({lang: lang})
    },
  },
};
</script>

<script setup>
import AppIcon from "@/assets/icons/app.svg";
import SettingsIcon from "@/assets/icons/settings.svg";
import WorldIcon from "@/assets/icons/world.svg";
import GithubIcon from "@/assets/icons/github.svg";
import AccountIcon from "@/assets/icons/person.svg";
import OptionIcon from "@/assets/icons/slider.svg";
import LikeIcon from "@/assets/icons/like.svg";
import DonateThumbIcon from "@/assets/icons/donate-thumb.svg";
import AfdianIcon from "@/assets/icons/afdian.svg";
import KofiIcon from "@/assets/icons/kofi.svg";
import ToolsIcon from "@/assets/icons/tools.svg";
import FlaskIcon from "@/assets/icons/flask.svg";
import HelpIcon from "@/assets/icons/help.svg";
</script>

<style lang="postcss" scoped>
.app-container {
  width: 100%;
  min-height: 100vh;
  background-color: var(--atv-canvas);
  background-image: var(--atv-page-background);
  background-size: cover;
  background-attachment: fixed;
}
.main-container {
  @apply px-6 lg:px-16 py-8 gap-y-16;
}

.atv-theme-glyph { display: inline-flex; width: 20px; height: 20px; }
.atv-theme-glyph svg { display: block; width: 100%; height: 100%; }
</style>
