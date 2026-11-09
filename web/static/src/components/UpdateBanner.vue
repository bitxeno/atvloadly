<template>
  <div v-if="visible" class="alert atv-update-banner mb-6" role="status">
    <span class="text-sm flex-1">
      {{ $t("home.release_banner.message", { version: release.latest, current: release.current }) }}
    </span>
    <div class="flex gap-x-2">
      <a class="btn btn-sm btn-primary" :href="release.url" target="_blank" rel="noopener">
        {{ $t("home.release_banner.view") }}
      </a>
      <button type="button" class="btn btn-sm btn-ghost" @click="dismiss">
        {{ $t("home.release_banner.dismiss") }}
      </button>
    </div>
  </div>
</template>

<script>
import api from "@/api/api";

// The dismissed release tag: a newer release shows the banner again.
const dismissedKey = "atvloadly-dismissed-release";

function readDismissed() {
  try {
    return window.localStorage.getItem(dismissedKey);
  } catch {
    return null;
  }
}

export default {
  name: "UpdateBanner",
  data() {
    return {
      release: null,
      dismissed: readDismissed(),
    };
  },
  computed: {
    visible() {
      return !!this.release?.available && this.release.latest !== this.dismissed;
    },
  },
  created() {
    api.getLatestRelease().then((res) => {
      this.release = res.data;
    }).catch(() => {});
  },
  methods: {
    dismiss() {
      this.dismissed = this.release.latest;
      try {
        window.localStorage.setItem(dismissedKey, this.dismissed);
      } catch {
        // Without storage the banner only stays hidden for this page load.
      }
    },
  },
};
</script>
