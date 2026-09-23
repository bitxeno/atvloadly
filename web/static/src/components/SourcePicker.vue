<template>
  <div class="flex flex-col gap-y-3 w-full atv-source-picker">
    <div class="form-control w-full">
      <div class="label gap-x-2">
        <span class="label-text">{{ $t("install.form.source.label") }}</span>
        <div class="tabs tabs-boxed flex-nowrap">
          <button
            type="button"
            class="tab tab-sm gap-x-1"
            :class="{ 'tab-active': kind === 'github' }"
            :aria-pressed="kind === 'github'"
            @click="setKind('github')"
          >
            <span class="w-4 h-4"><GithubIcon /></span>GitHub
          </button>
          <button
            type="button"
            class="tab tab-sm gap-x-1"
            :class="{ 'tab-active': kind === 'altstore' }"
            :aria-pressed="kind === 'altstore'"
            @click="setKind('altstore')"
          >
            <span class="w-4 h-4"><LinkIcon /></span>AltStore
          </button>
        </div>
      </div>
      <div class="flex items-center gap-x-2 mb-2" v-if="kind === 'altstore' && savedSources?.length">
        <select
          class="select select-bordered select-sm flex-1 min-w-0"
          v-model="savedChoice"
          :aria-label="$t('install.form.source.saved.label')"
        >
          <option value="" disabled>{{ $t("install.form.source.saved.label") }}</option>
          <option value="all">{{ $t("install.form.source.saved.all") }}</option>
          <option v-for="saved in savedSources" :key="saved.id" :value="String(saved.id)">{{ saved.name }}</option>
        </select>
        <span class="loading loading-spinner loading-sm" v-show="loading && all"></span>
      </div>
      <div class="join w-full">
        <input
          type="text"
          class="input input-bordered join-item flex-1 min-w-0"
          v-model="url"
          :placeholder="kind === 'github'
            ? $t('install.form.source.url_placeholder.github')
            : $t('install.form.source.url_placeholder.altstore')"
          autocomplete="off"
          autocapitalize="off"
          spellcheck="false"
          @input="clearPreview"
          @keydown.enter.prevent="fetchPreview"
        />
        <button
          type="button"
          class="btn join-item"
          :disabled="loading || !url.trim()"
          @click="fetchPreview"
        >
          <span class="loading loading-spinner loading-sm" v-show="loading && !all"></span>
          {{ $t("install.form.source.fetch") }}
        </button>
      </div>
      <label class="label cursor-pointer justify-start gap-x-2" v-if="kind === 'github'">
        <input
          type="checkbox"
          class="checkbox checkbox-sm"
          v-model="prerelease"
          @change="fetchPreview"
        />
        <span class="label-text">{{ $t("install.form.source.prerelease") }}</span>
      </label>
    </div>

    <div class="flex flex-col gap-y-2" v-if="preview">
      <div class="flex flex-wrap items-center gap-x-2 gap-y-1 min-h-6" v-if="!all">
        <a
          class="link link-hover text-sm break-all"
          :href="preview.page_url"
          target="_blank"
          rel="noopener noreferrer"
        >{{ preview.title || preview.url }}</a>
        <button
          type="button"
          class="btn btn-ghost btn-xs gap-x-1"
          v-if="canSaveSource"
          :disabled="saving"
          @click="saveSource"
        >
          <span class="w-4 h-4"><BookmarkIcon /></span>{{ $t("install.form.source.save_source") }}
        </button>
      </div>

      <div class="alert alert-warning text-xs" v-for="failed in catalogErrors" :key="failed.id">
        <span class="whitespace-normal break-words">{{ failed.name }}: {{ failed.error }}</span>
      </div>

      <div class="alert alert-info text-sm" v-if="needsChoice">
        <span class="whitespace-normal">{{
          preview.kind === "github"
            ? $t("install.form.source.choose_tips")
            : all
              ? $t("install.form.source.choose_saved_app_tips")
              : $t("install.form.source.choose_app_tips")
        }}</span>
      </div>

      <input
        type="search"
        class="input input-bordered input-sm w-full"
        v-if="searchable"
        v-model="query"
        :placeholder="$t('install.form.source.search_placeholder')"
        :aria-label="$t('install.form.source.search_placeholder')"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        @keydown.enter.prevent
      />

      <label
        class="atv-source-build"
        :class="{ 'atv-source-build--selected': buildKey(build) === selectedId }"
        v-for="build in visibleBuilds"
        :key="buildKey(build)"
      >
        <input
          type="radio"
          class="radio radio-sm"
          :name="radioName"
          :value="buildKey(build)"
          :checked="buildKey(build) === selectedId"
          @change="selectBuild(buildKey(build))"
        />
        <img
          v-if="build.icon_url && !failedIcons[build.icon_url]"
          :src="build.icon_url"
          alt=""
          loading="lazy"
          class="w-10 h-10 rounded-lg object-cover shrink-0"
          @error="failedIcons[build.icon_url] = true"
        />
        <div class="flex flex-col gap-y-1 min-w-0">
          <div class="flex flex-wrap items-center gap-1">
            <span class="font-semibold break-all">{{ build.name }}</span>
            <span class="badge badge-outline badge-sm" v-if="platformLabel(build.platform)">{{
              platformLabel(build.platform)
            }}</span>
            <span class="badge badge-warning badge-sm" v-if="build.prerelease">{{
              $t("install.form.source.prerelease_badge")
            }}</span>
            <span class="badge badge-success badge-sm" v-if="build.id === preview.suggested_id">{{
              $t("install.form.source.recommended")
            }}</span>
            <span class="badge badge-ghost badge-sm max-w-full" v-if="build.source_name">
              <span class="truncate">{{ build.source_name }}</span>
            </span>
          </div>
          <span class="text-xs text-base-content/70 break-words" v-if="build.subtitle || build.developer">{{
            [build.subtitle, build.developer].filter(Boolean).join(" · ")
          }}</span>
          <span class="text-xs text-base-content/70 break-all">{{ buildDetails(build) }}</span>
        </div>
      </label>

      <span class="text-xs text-base-content/70" v-if="hiddenCount > 0">{{
        $t("install.form.source.more_results", { num: hiddenCount })
      }}</span>
      <span class="text-sm text-base-content/70" v-if="searchable && !matches.length">{{
        $t("install.form.source.no_match")
      }}</span>

      <div class="collapse collapse-arrow atv-source-advanced" v-if="selectedBuild">
        <input type="checkbox" :aria-label="$t('install.form.source.advanced')" />
        <div class="collapse-title text-sm font-medium">
          {{ $t("install.form.source.advanced") }}
        </div>
        <div class="collapse-content">
          <template v-if="preview.kind === 'github'">
            <label class="label gap-x-2">
              <span class="label-text">{{ $t("install.form.source.filter.label") }}</span>
              <span class="badge badge-ghost badge-sm" v-if="filterCustom">{{
                $t("install.form.source.filter.custom")
              }}</span>
            </label>
            <input
              type="text"
              class="input input-bordered w-full font-mono text-sm"
              v-model="filter"
              autocomplete="off"
              autocapitalize="off"
              spellcheck="false"
              @input="filterCustom = true"
              @keydown.enter.prevent
            />
            <label class="label gap-x-2">
              <span class="label-text-alt whitespace-normal">{{ $t("install.form.source.filter.tips") }}</span>
              <a class="link text-xs shrink-0" v-if="filterCustom" @click.prevent="resetFilter">{{
                $t("install.form.source.filter.reset")
              }}</a>
            </label>
            <span class="text-xs atv-source-mismatch" v-if="filterMismatch">{{
              $t("install.form.source.filter.mismatch")
            }}</span>
          </template>
          <template v-else>
            <label class="label">
              <span class="label-text">{{ $t("install.form.source.bundle_id.label") }}</span>
            </label>
            <input type="text" class="input input-bordered w-full font-mono text-sm" :value="filter" readonly />
            <label class="label">
              <span class="label-text-alt whitespace-normal">{{ $t("install.form.source.bundle_id.tips") }}</span>
            </label>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import dayjs from "dayjs";
import api from "@/api/api";
import { toast } from "vue3-toastify";
import {
  catalogBuilds,
  filterMatches,
  formatBytes,
  guessSourceKind,
  listedBuilds,
  platformLabel,
  searchBuilds,
} from "@/utils/source.mjs";

let pickerCount = 0;

// Most AltStore apps listed at once; the search narrows longer lists.
const maxListedBuilds = 50;

export default {
  name: "SourcePicker",
  props: {
    deviceClass: {
      type: String,
      default: "",
    },
    // Source to start from: {kind, url, filter, prerelease}. A URL is fetched on mount.
    initial: {
      type: Object,
      default: null,
    },
  },
  emits: ["update:selection"],
  data() {
    const initial = this.initial || {};
    pickerCount++;
    return {
      radioName: `source-build-${pickerCount}`,
      kind: initial.kind || "github",
      url: initial.url || "",
      prerelease: !!initial.prerelease,
      loading: false,
      // Identifies the latest preview or catalog request: older responses are ignored.
      requestSeq: 0,
      preview: null,
      // Key (buildKey) of the selected build.
      selectedId: "",
      // Builds whose icon could not be loaded, hidden instead of shown broken.
      failedIcons: {},
      filter: initial.filter || "",
      filterCustom: false,
      // Saved AltStore sources, null until loaded.
      savedSources: null,
      saving: false,
      // True when preview lists the apps of all saved sources.
      all: false,
      // Saved sources the catalog could not read.
      catalogErrors: [],
      query: "",
      // Key of the build selected when the list was loaded or searched: it
      // stays listed beyond the cap. Later clicks do not change it, so rows
      // do not move under the pointer.
      pinnedKey: "",
    };
  },
  computed: {
    selectedBuild() {
      if (!this.preview) return null;
      return this.preview.builds.find((b) => this.buildKey(b) === this.selectedId) || null;
    },
    needsChoice() {
      return this.preview.builds.length > 1 && !this.preview.suggested_id;
    },
    filterMismatch() {
      return !!this.filter.trim() && !filterMatches(this.preview.kind, this.filter.trim(), this.selectedBuild);
    },
    selection() {
      if (!this.selectedBuild) return null;
      return {
        kind: this.preview.kind,
        // Builds of the catalog are tracked in their own source.
        url: this.all ? this.selectedBuild.source_url : this.preview.url,
        filter: this.filter.trim(),
        prerelease: this.preview.kind === "github" && this.prerelease,
        build: this.selectedBuild,
      };
    },
    // Value of the saved sources select: "all", the id of the saved source
    // shown, or "" for another source.
    savedChoice: {
      get() {
        if (this.all) return "all";
        const url = this.url.trim();
        const saved = (this.savedSources || []).find((s) => s.url === url || s.url === this.preview?.url);
        return saved ? String(saved.id) : "";
      },
      set(value) {
        if (value === "all") {
          this.showCatalog();
          return;
        }
        const saved = this.savedSources.find((s) => String(s.id) === value);
        if (!saved) return;
        this.clearPreview();
        this.url = saved.url;
        // Saved sources are AltStore sources, even on github.com.
        this.loadPreview();
      },
    },
    canSaveSource() {
      return (
        this.preview.kind === "altstore" &&
        !!this.savedSources &&
        !this.savedSources.some((s) => s.url === this.preview.url)
      );
    },
    searchable() {
      return this.preview.kind === "altstore" && this.preview.builds.length > 1;
    },
    matches() {
      return this.searchable ? searchBuilds(this.preview.builds, this.query) : this.preview.builds;
    },
    visibleBuilds() {
      if (!this.searchable) return this.matches;
      return listedBuilds(this.matches, maxListedBuilds, (b) => this.buildKey(b) === this.pinnedKey);
    },
    hiddenCount() {
      return this.matches.length - this.visibleBuilds.length;
    },
  },
  watch: {
    selection(value) {
      this.$emit("update:selection", value);
    },
    deviceClass() {
      if (this.all) this.fetchCatalog();
      else if (this.preview) this.loadPreview();
    },
    query() {
      this.pinnedKey = this.selectedId;
    },
  },
  mounted() {
    this.loadSavedSources();
    // The initial source has a known kind.
    if (this.url) this.loadPreview();
  },
  methods: {
    platformLabel,
    // buildKey identifies a build in the list: catalog builds of different
    // sources may share an id.
    buildKey(build) {
      return build.source_url ? `${build.source_url} ${build.id}` : build.id;
    },
    setKind(kind) {
      if (this.kind === kind) return;
      this.kind = kind;
      this.filter = "";
      this.filterCustom = false;
      this.clearPreview();
    },
    clearPreview() {
      this.requestSeq++;
      this.loading = false;
      this.preview = null;
      this.selectedId = "";
      this.all = false;
      this.catalogErrors = [];
      this.query = "";
      this.pinnedKey = "";
    },
    loadSavedSources() {
      return api
        .getSavedSources()
        .then((res) => {
          this.savedSources = res.data || [];
        })
        .catch((err) => {
          // request.js already shows the error.
          console.error(err);
        });
    },
    saveSource() {
      this.saving = true;
      api
        .addSavedSource({ url: this.preview.url })
        .then((res) => {
          toast.success(this.$t("install.form.source.toast.source_saved", { name: res.data.name }));
          return this.loadSavedSources();
        })
        .catch((err) => {
          console.error(err);
        })
        .finally(() => {
          this.saving = false;
        });
    },
    showCatalog() {
      this.clearPreview();
      this.url = "";
      this.all = true;
      this.fetchCatalog();
    },
    fetchCatalog() {
      const seq = ++this.requestSeq;
      this.loading = true;
      api
        .getSourceCatalog({ device_class: this.deviceClass })
        .then((res) => {
          if (seq === this.requestSeq) this.applyCatalog(res.data || []);
        })
        .catch(() => {
          if (seq === this.requestSeq) this.clearPreview();
        })
        .finally(() => {
          if (seq === this.requestSeq) this.loading = false;
        });
    },
    applyCatalog(catalog) {
      const key = this.selectedId;
      this.catalogErrors = catalog.filter((s) => s.error);
      // The catalog is listed like the preview of one AltStore source whose
      // builds know their own source.
      this.preview = {
        kind: "altstore",
        url: "",
        title: "",
        page_url: "",
        builds: catalogBuilds(catalog, this.deviceClass),
        suggested_id: "",
      };

      // Keep the selected build, else the only build of the tracked app, else
      // the only build.
      const builds = this.preview.builds;
      const tracked = builds.filter((b) => filterMatches("altstore", this.filter, b));
      const kept =
        builds.find((b) => this.buildKey(b) === key) ||
        (tracked.length === 1 ? tracked[0] : null) ||
        (builds.length === 1 ? builds[0] : null);
      this.selectBuild(kept ? this.buildKey(kept) : "");
      this.pinnedKey = this.selectedId;
    },
    // fetchPreview previews the typed URL as the kind of source it looks like.
    fetchPreview() {
      const guessed = guessSourceKind(this.url);
      if (guessed) this.setKind(guessed);
      this.loadPreview();
    },
    // loadPreview previews the URL as a source of the current kind.
    loadPreview() {
      const url = this.url.trim();
      if (!url) return;

      const seq = ++this.requestSeq;
      this.loading = true;
      api
        .previewSource({
          kind: this.kind,
          url,
          device_class: this.deviceClass,
          prerelease: this.kind === "github" && this.prerelease,
        })
        .then((res) => {
          if (seq === this.requestSeq) this.applyPreview(res.data);
        })
        .catch(() => {
          // request.js already shows the error.
          if (seq === this.requestSeq) this.clearPreview();
        })
        .finally(() => {
          if (seq === this.requestSeq) this.loading = false;
        });
    },
    applyPreview(preview) {
      this.preview = { ...preview, builds: preview.builds || [] };

      // Keep the current filter when it still selects exactly one build.
      const tracked = this.filter
        ? this.preview.builds.filter((b) => filterMatches(this.preview.kind, this.filter, b))
        : [];
      const builds = this.preview.builds;
      if (tracked.length === 1) {
        this.selectedId = tracked[0].id;
      } else {
        this.selectBuild(this.preview.suggested_id || (builds.length === 1 ? builds[0].id : ""));
      }
      this.pinnedKey = this.selectedId;
    },
    selectBuild(key) {
      this.selectedId = key;
      const build = this.selectedBuild;
      if (build && (this.preview.kind === "altstore" || !this.filterCustom)) {
        this.filter = build.filter;
      }
    },
    resetFilter() {
      this.filterCustom = false;
      if (this.selectedBuild) this.filter = this.selectedBuild.filter;
    },
    buildDetails(build) {
      const date = dayjs(build.date);
      return [
        build.version,
        build.date && date.year() > 1 ? date.format("YYYY-MM-DD") : "",
        formatBytes(build.size),
      ]
        .filter(Boolean)
        .join(" · ");
    },
  },
};
</script>

<script setup>
import BookmarkIcon from "@/assets/icons/bookmark.svg";
import GithubIcon from "@/assets/icons/github.svg";
import LinkIcon from "@/assets/icons/link.svg";
</script>

<style scoped>
/* The theme layer gives inputs and buttons their own radius and height;
   keep the URL input and the Fetch button fused like other join rows. */
.atv-source-picker .join {
  align-items: stretch;
}

.atv-source-picker .join > .join-item {
  height: auto;
  min-height: 3rem;
  border-radius: 0;
}

.atv-source-picker .join > .join-item:first-child {
  border-start-start-radius: var(--rounded-btn, 0.5rem);
  border-end-start-radius: var(--rounded-btn, 0.5rem);
}

.atv-source-picker .join > .join-item:last-child {
  border-start-end-radius: var(--rounded-btn, 0.5rem);
  border-end-end-radius: var(--rounded-btn, 0.5rem);
}

/* Match the theme's primary buttons and soft notices. */
.atv-source-picker .tabs-boxed .tab-active {
  background: var(--atv-button-bg);
  color: var(--atv-button-text);
}

.atv-source-picker .badge-success {
  color: var(--atv-success-text);
  background: var(--atv-success-soft);
  border-color: var(--atv-success-border);
}

.atv-source-picker .alert-info {
  background: var(--atv-accent-soft);
  border: 1px solid var(--atv-border);
  color: var(--atv-ink);
  text-align: start;
}

.atv-source-picker .alert-warning {
  padding: 6px 12px;
  border-radius: 10px;
  background: var(--atv-warning-bg);
  border: 1px solid var(--atv-warning-border);
  color: var(--atv-warning-text);
  text-align: start;
}

.atv-source-build {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--atv-border);
  border-radius: 12px;
  cursor: pointer;
  transition: background-color 150ms ease, border-color 150ms ease;
}

.atv-source-build:hover {
  background: var(--atv-surface-hover);
}

.atv-source-build--selected {
  border-color: var(--atv-accent);
  background: var(--atv-accent-soft);
}

.atv-source-advanced {
  border: 1px solid var(--atv-border);
  border-radius: 12px;
}

.atv-source-mismatch {
  color: var(--atv-warning-text);
}

@media (max-width: 767px) {
  .atv-source-picker input[type="text"],
  .atv-source-picker input[type="search"],
  .atv-source-picker select {
    font-size: 16px;
  }
}
</style>
