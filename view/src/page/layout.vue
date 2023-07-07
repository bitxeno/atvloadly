<template>
  <div class="app-container">
    <header class="navbar border-b pt-0 pb-0">
      <div class="flex-1">
        <a href="/" class="btn btn-ghost normal-case text-xl">atvloadly</a>
      </div>
      <div class="flex-none">
        <ul class="menu menu-horizontal px-1">
          <li>
            <router-link :to="{ name: 'settings' }">
              <span class="w-5">
                <SettingsIcon />
              </span>
              设置</router-link
            >
          </li>
        </ul>
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
};
</script>

<script setup>
import SettingsIcon from "@/assets/icons/settings.svg";
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