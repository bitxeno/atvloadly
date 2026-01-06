<template>
  <div class="app-container">
    <header class="navbar border-b pt-0 pb-0">
      <div class="flex-1">
        <label tabindex="0" class="btn btn-ghost rounded-btn">
         <AppIcon class="w-7 h-7"/>
        <a href="/" class="normal-case text-xl">atvloadly</a>
        </label>
      </div>
      <div class="flex-none">
        <nav class="navbar w-full"> 
          <div class="dropdown dropdown-hover">
            <label tabindex="0" class="btn btn-ghost rounded-btn px-2 md:px-4">
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
                <router-link :to="{ name: 'settings' }">
                  <span class="w-5">
                  <SettingsIcon />
                </span>
                  {{ $t("nav.settings") }}
                </router-link>
              </li>
            </ul>
          </div>

          <div class="dropdown dropdown-hover">
            <label tabindex="0" class="btn btn-ghost rounded-btn px-2 md:px-4">
              <span class="w-5">
                <LanguageIcon />
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

          <div>
            <label tabindex="0" class="btn btn-ghost rounded-btn px-2 md:px-4" @click="showDonateModal = true">
                <LikeIcon class="w-5 h-5" />
              <span class="hidden sm:inline">{{ $t("nav.donate") }}</span>
            </label>
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
    <div v-if="showDonateModal" class="modal modal-open modal-bottom sm:modal-middle" @click.self="showDonateModal = false">
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
           <a href="https://ko-fi.com/bitxeno" target="_blank" class="btn border-none w-full normal-case text-lg gap-2 text-white hover:opacity-90" style="background-color: #29abe0;">
             <img src="https://storage.ko-fi.com/cdn/cup-border.png" class="w-6 h-6" alt="Ko-fi"/>
             {{ $t("donate.kofi") }}
           </a>
           <a href="https://afdian.com/a/bitxeno" target="_blank" class="btn border-none w-full normal-case text-lg gap-2 text-white hover:opacity-90" style="background-color: #946ce6;">
             <AfdianIcon class="w-6 h-6" />
             {{ $t("donate.afdian") }}
           </a>
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
      showDonateModal: false,
    };
  },
  created() {
    api.syncLang({lang: this.$i18next.language})
    let keys = Object.keys(this.$i18next.options.resources);
    for (const key of keys) {
      this.languages.push({
        key: key,
        name: this.$i18next.options.resources[key].name,
      });
    }
  },
  methods: {
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
import LanguageIcon from "@/assets/icons/language.svg";
import GithubIcon from "@/assets/icons/github.svg";
import AccountIcon from "@/assets/icons/person.svg";
import OptionIcon from "@/assets/icons/slider.svg";
import LikeIcon from "@/assets/icons/like.svg";
import AfdianIcon from "@/assets/icons/afdian.svg";
import ToolsIcon from "@/assets/icons/tools.svg";
</script>

<style lang="postcss" scoped>
.app-container {
  width: 100%;
  min-height: 100vh;
  background-attachment: fixed;
  background-size: cover;
  background-repeat: no-repeat;
  background-image: radial-gradient(
      circle 800px at 700px 200px,
      hsl(276 100% 99%),
      #fdfcfd00
    ),
    radial-gradient(circle 800px at right center, hsl(193 99% 94.7%), #fdfcfd00),
    radial-gradient(
      circle 800px at right bottom,
      hsl(193 100% 98.8%),
      #fdfcfd00
    ),
    radial-gradient(
      circle 800px at calc(50% - 600px) calc(100% - 100px),
      hsl(323 86.3% 96.5%),
      hsl(322 100% 99.4%),
      #fdfcfd00
    );
}
.main-container {
  @apply px-6 lg:px-16 py-8 gap-y-16;
}
</style>