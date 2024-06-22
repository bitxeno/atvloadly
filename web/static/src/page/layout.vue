<template>
  <div class="app-container">
    <header class="navbar border-b pt-0 pb-0">
      <div class="flex-1">
        <a href="/" class="btn btn-ghost normal-case text-xl">atvloadly</a>
      </div>
      <div class="flex-none">
        <nav class="navbar w-full">
          <div>
            <router-link :to="{ name: 'settings' }">
              <label tabindex="0" class="btn btn-ghost rounded-btn">
                <span class="w-5">
                  <SettingsIcon />
                </span>
                {{ $t("nav.settings") }}</label
              >
            </router-link>
          </div>

          <div class="dropdown dropdown-hover">
            <label tabindex="0" class="btn btn-ghost rounded-btn">
              <span class="w-5">
                <LanguageIcon />
              </span>
              {{ $t("nav.language") }}</label
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
            <a href="https://github.com/bitxeno/atvloadly" target="_blank">
              <label tabindex="0" class="btn btn-ghost rounded-btn">
                <span class="w-5"> <GithubIcon /> </span
              ></label>
            </a>
          </div>
        </nav>
      </div>
    </header>

    <div class="main-container">
      <router-view />
    </div>
  </div>
</template>
  
  <script>
export default {
  name: "App",
  data() {
    return {
      languages: [],
    };
  },
  created() {
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
    },
  },
};
</script>

<script setup>
import SettingsIcon from "@/assets/icons/settings.svg";
import LanguageIcon from "@/assets/icons/language.svg";
import GithubIcon from "@/assets/icons/github.svg";
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