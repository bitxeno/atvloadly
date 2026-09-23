<template>
  <dialog :class="['modal', { 'modal-open': visible }]">
    <div class="modal-box atv-source-dialog" v-if="app">
      <button type="button" class="btn btn-sm btn-circle btn-ghost absolute right-2 top-2" @click="close">✕</button>
      <h3 class="font-bold text-lg pr-8 break-words">
        {{ tracked
          ? $t("source_dialog.title", { name: appName })
          : $t("source_dialog.title_track", { name: appName }) }}
      </h3>

      <div class="mt-4 flex flex-col gap-y-4">
        <dl class="atv-source-facts text-sm">
          <dt>{{ $t("source_dialog.installed_version") }}</dt>
          <dd class="break-all">{{ app.version || "—" }}</dd>
          <template v-if="tracked">
            <dt>{{ $t("source_dialog.installed_build") }}</dt>
            <dd class="break-all">{{ installedBuild }}</dd>
            <dt>{{ $t("source_dialog.checked_at") }}</dt>
            <dd>{{ checkedAt }}</dd>
          </template>
        </dl>

        <div class="alert alert-warning atv-warning text-sm" v-if="warning">
          <span class="w-5 h-5 shrink-0"><WarningIcon /></span>
          <span class="whitespace-normal break-words">{{ warning }}</span>
        </div>

        <SourcePicker
          :key="pickerKey"
          :device-class="app.device_class"
          :initial="initial"
          @update:selection="selection = $event"
        />

        <div class="form-control">
          <label class="label cursor-pointer justify-between items-center gap-x-4">
            <div class="flex items-center">
              <span class="label-text">{{ $t("install.form.source.auto_update") }}</span>
              <div class="tooltip" :data-tip="$t('install.form.source.auto_update_tips')">
                <div class="w-4 h-4 text-secondary-content"><HelpIcon /></div>
              </div>
            </div>
            <input type="checkbox" class="toggle toggle-success" v-model="autoUpdate" />
          </label>

          <label class="label cursor-pointer justify-start gap-x-2" v-if="sourceChanged">
            <input type="checkbox" class="checkbox checkbox-sm" v-model="alreadyInstalled" />
            <span class="label-text whitespace-normal">{{
              $t("source_dialog.already_installed", { version: selection.build.version })
            }}</span>
          </label>
        </div>
      </div>

      <div class="modal-action flex-wrap gap-y-2">
        <div class="mr-auto" v-if="tracked">
          <Popper placement="top" arrow="true">
            <template #content="{ close: closePopper }">
              <div class="flex flex-col gap-y-2">
                <div class="py-2">{{ $t("source_dialog.untrack_confirm", { name: appName }) }}</div>
                <div class="flex gap-x-2 justify-end items-center">
                  <a class="link link-primary link-hover" @click="closePopper">{{
                    $t("source_dialog.button.cancel")
                  }}</a>
                  <button type="button" class="btn btn-primary btn-xs" @click="untrack(closePopper)">
                    {{ $t("source_dialog.button.confirm") }}
                  </button>
                </div>
              </div>
            </template>
            <button type="button" class="btn btn-error" :disabled="busy">
              {{ $t("source_dialog.button.untrack") }}
            </button>
          </Popper>
        </div>
        <button type="button" class="btn" :disabled="!selection || busy" @click="save">
          {{ $t("source_dialog.button.save") }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="!selection || busy" @click="install">
          <span class="loading loading-spinner" v-show="busy"></span>
          {{ $t("source_dialog.button.install") }}
        </button>
      </div>
    </div>
  </dialog>
</template>

<script>
import dayjs from "dayjs";
import api from "@/api/api";
import { toast } from "vue3-toastify";
import SourcePicker from "@/components/SourcePicker.vue";
import { installLinkFilter, updateFailed } from "@/utils/source.mjs";

export default {
  name: "SourceDialog",
  components: { SourcePicker },
  emits: ["saved", "update-started"],
  data() {
    return {
      visible: false,
      app: null,
      pickerKey: 0,
      selection: null,
      autoUpdate: false,
      alreadyInstalled: false,
      busy: false,
    };
  },
  computed: {
    appName() {
      return this.app.custom_name || this.app.ipa_name;
    },
    tracked() {
      return !!this.app.source.kind;
    },
    initial() {
      if (!this.tracked) return null;
      const { kind, url, filter, prerelease } = this.app.source;
      return { kind, url, filter, prerelease };
    },
    installedBuild() {
      const { version, build_name } = this.app.source;
      return [version, build_name].filter(Boolean).join(" · ") || "—";
    },
    checkedAt() {
      const checkedAt = this.app.source.checked_at;
      return checkedAt ? dayjs(checkedAt).format("YYYY-MM-DD HH:mm") : this.$t("source_dialog.never");
    },
    warning() {
      const source = this.app.source;
      if (source.check_error) return source.check_error;
      return updateFailed(source) ? this.$t("home.source.update_failed", { version: source.latest_version }) : "";
    },
    // True when saving links the app to a new source (or the first one).
    sourceChanged() {
      if (!this.selection) return false;
      const source = this.app.source;
      return (
        this.selection.kind !== source.kind ||
        this.selection.url.toLowerCase() !== source.url.toLowerCase()
      );
    },
  },
  methods: {
    show(app) {
      this.app = app;
      this.selection = null;
      this.autoUpdate = app.source.auto_update;
      this.alreadyInstalled = false;
      this.pickerKey++;
      this.visible = true;
    },
    close() {
      this.visible = false;
    },
    async link(markInstalled, filter = this.selection.filter) {
      const s = this.selection;
      const res = await api.linkAppSource(this.app.ID, {
        kind: s.kind,
        url: s.url,
        filter,
        prerelease: s.prerelease,
        auto_update: this.autoUpdate,
        installed_build_id: markInstalled && this.sourceChanged && this.alreadyInstalled ? s.build.id : "",
      });
      this.app = res.data;
      this.$emit("saved", res.data);
    },
    async save() {
      this.busy = true;
      try {
        await this.link(true);
        toast.success(this.$t("source_dialog.toast.saved"));
        this.close();
      } catch (err) {
        // request.js already shows the error.
        console.error(err);
      } finally {
        this.busy = false;
      }
    },
    async install() {
      const s = this.selection;
      this.busy = true;
      try {
        // The update installs the tracked source, so persist the source first.
        // A new filter of the same source is saved only once the update installs.
        const filter = installLinkFilter(this.app.source, s, this.autoUpdate);
        if (filter !== null) {
          await this.link(false, filter);
        }
        const res = await api.updateAppFromSource(this.app.ID, {
          build_id: s.build.id,
          filter: s.filter,
        });
        this.$emit("update-started", this.app, res.data);
        this.close();
      } catch (err) {
        console.error(err);
      } finally {
        this.busy = false;
      }
    },
    untrack(closePopper) {
      closePopper?.();
      this.busy = true;
      api
        .unlinkAppSource(this.app.ID)
        .then(() => {
          toast.success(this.$t("source_dialog.toast.untracked", { name: this.appName }));
          this.$emit("saved");
          this.close();
        })
        .catch((err) => {
          console.error(err);
        })
        .finally(() => {
          this.busy = false;
        });
    },
  },
};
</script>

<script setup>
import HelpIcon from "@/assets/icons/help.svg";
import WarningIcon from "@/assets/icons/warning.svg";
</script>

<style scoped>
/* The modal stays below the sticky navbar, so the box leaves room for it. */
.modal .modal-box.atv-source-dialog {
  width: min(640px, calc(100vw - 24px));
  max-width: 640px;
  max-height: calc(100dvh - 160px);
  overflow-y: auto;
}

/* Keep the actions visible while the dialog content scrolls. */
.atv-source-dialog .modal-action {
  position: sticky;
  bottom: -1.5rem;
  z-index: 1;
  margin-bottom: -1.5rem;
  padding: 12px 0 1.5rem;
  background: inherit;
}

.atv-source-facts {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 4px 16px;
}

.atv-source-facts dt {
  color: var(--atv-muted);
}

:deep(.popper) {
  background: var(--atv-surface);
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--atv-border);
  box-shadow: var(--atv-menu-shadow);
  color: var(--atv-ink);
  word-break: break-all;
  min-width: 150px;
}

:deep(.popper:hover),
:deep(.popper:hover > #arrow::before),
:deep(.popper #arrow::before) {
  background: var(--atv-surface);
}

@media (max-width: 767px) {
  .modal .modal-box.atv-source-dialog {
    padding: 16px;
  }

  .atv-source-dialog .modal-action {
    bottom: -16px;
    margin-bottom: -16px;
    padding-bottom: 16px;
  }
}
</style>
