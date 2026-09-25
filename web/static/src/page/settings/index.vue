<template>
  <div class="settings-page max-w-screen-lg mx-auto">
    <section class="section bg-base-100">
      <h2 class="atv-section-heading">{{ $t("settings.notification.title") }}</h2>
      <form>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.toggle.label")
            }}</span>
          </label>
          <input
            type="checkbox"
            class="toggle toggle-success"
            v-model="settings.notification.enabled"
          />
        </div>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.type.label")
            }}</span>
          </label>

          <div class="flex gap-2 flex-wrap">
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.notification.type"
                value="bark"
              />
              <span class="label-text">Bark</span>
            </label>
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.notification.type"
                value="telegram"
              />
              <span class="label-text">Telegram</span>
            </label>
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.notification.type"
                value="weixin"
              />
              <span class="label-text">{{
                $t("settings.notification.weixin.title")
              }}</span>
            </label>
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.notification.type"
                value="webhook"
              />
              <span class="label-text">Webhook</span>
            </label>
          </div>
        </div>
        <Weixin
          v-if="settings.notification.type == 'weixin'"
          v-model="settings.notification.weixin"
        />
        <Telegram
          v-if="settings.notification.type == 'telegram'"
          v-model="settings.notification.telegram"
        />
        <Bark
          v-if="settings.notification.type == 'bark'"
          v-model="settings.notification.bark"
        />
        <Webhook
          v-if="settings.notification.type == 'webhook'"
          v-model="settings.notification.webhook"
        />

        <div class="form-item">
          <label class="form-item-label"></label>
          <div class="flex-1 flex justify-between">
            <button
              class="btn btn-primary w-32"
              @click.prevent="saveNotification"
            >
              {{ $t("settings.notification.button.submit") }}
            </button>

            <a
              class="link"
              @click.prevent="testNotification"
              >{{ $t("settings.notification.button.send_test") }}</a
            >
          </div>
        </div>
      </form>
    </section>

    <section class="section bg-base-100">
      <h2 class="atv-section-heading">{{ $t("settings.refresh.title") }}</h2>
      <form>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.refresh.toggle.label")
            }}</span>
          </label>
          <div class="flex flex-col grow">
          <input
            type="checkbox"
            class="toggle toggle-success"
            v-model="settings.task.enabled"
            @change="onTaskEnabledChange"
          />
          <label class="label">
            <span class="label-text-alt">{{
              $t("settings.refresh.iphone_toggle.tips")
            }}</span>
          </label>
          </div>
        </div>

        <div class="form-item !hidden">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.refresh.iphone_toggle.label")
            }}</span>
          </label>
          <div class="flex flex-col grow">
          <input
            type="checkbox"
            class="toggle toggle-success"
            v-model="settings.task.iphone_enabled"
          />
          <label class="label">
            <span class="label-text-alt">{{
              $t("settings.refresh.iphone_toggle.tips")
            }}</span>
          </label>
          </div>
        </div>

        <div class="form-item !hidden">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.refresh.mode.label")
            }}</span>
          </label>

          <div class="flex gap-x-2">
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.task.mode"
                value="1"
              />
              <span class="label-text">{{
                $t("settings.refresh.mode.day_before_expired")
              }}</span>
            </label>
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.task.mode"
                value="2"
              />
              <span class="label-text">{{
                $t("settings.refresh.mode.custom")
              }}</span>
            </label>
          </div>
        </div>

        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.refresh.advance_days.label")
            }}</span>
          </label>
          <div class="flex gap-x-2">
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.task.advance_days"
                :value="1"
              />
              <span class="label-text">{{
                $t("settings.refresh.advance_days.1_day")
              }}</span>
            </label>
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.task.advance_days"
                :value="2"
              />
              <span class="label-text">{{
                $t("settings.refresh.advance_days.2_days")
              }}</span>
            </label>
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.task.advance_days"
                :value="3"
              />
              <span class="label-text">{{
                $t("settings.refresh.advance_days.3_days")
              }}</span>
            </label>
          </div>
        </div>

        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text mb-8">{{
              $t("settings.refresh.run_time.label")
            }}</span>
          </label>
          <div class="flex flex-col grow">
            <div class="flex flex-row items-center gap-2">
              <select
                v-model="startHour"
                class="select select-bordered w-full"
                @change="updateCronTime"
              >
                <option v-for="h in 24" :key="h" :value="h - 1">
                  {{ (h - 1).toString().padStart(2, "0") }}:00
                </option>
              </select>
              <span>-</span>
              <select
                v-model="endHour"
                class="select select-bordered w-full"
                @change="updateCronTime"
              >
                <option v-for="h in 24" :key="h" :value="h - 1">
                  {{ (h - 1).toString().padStart(2, "0") }}:00
                </option>
              </select>
            </div>
          </div>
        </div>

        <div class="form-item">
          <label class="form-item-label"></label>
          <div class="flex-1 flex justify-between">
            <button class="btn btn-primary w-32" @click.prevent="saveTask">
              {{ $t("settings.refresh.button.submit") }}
            </button>
          </div>
        </div>
      </form>
    </section>

    <section class="section bg-base-100">
      <h2 class="atv-section-heading">{{ $t("settings.update.title") }}</h2>
      <form>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.update.interval.label")
            }}</span>
          </label>
          <div class="flex flex-col grow">
            <select
              v-model="settings.update.check_interval"
              class="select select-bordered w-full"
            >
              <option v-for="h in updateIntervals" :key="h" :value="h">
                {{ formatUpdateInterval(h) }}
              </option>
            </select>
            <label class="label">
              <span class="label-text-alt whitespace-normal">{{
                $t("settings.update.tips")
              }}</span>
            </label>
          </div>
        </div>

        <div class="form-item" v-if="!settings.task.enabled">
          <label class="form-item-label"></label>
          <div class="alert alert-warning atv-warning text-sm">
            <span class="whitespace-normal">{{ $t("settings.update.task_disabled") }}</span>
          </div>
        </div>

        <div class="form-item">
          <label class="form-item-label"></label>
          <div class="flex-1 flex justify-between">
            <button class="btn btn-primary w-32" @click.prevent="saveUpdate">
              {{ $t("settings.update.button.submit") }}
            </button>
          </div>
        </div>
      </form>
    </section>

    <section class="section bg-base-100">
      <h2 class="atv-section-heading">{{ $t("settings.sources.title") }}</h2>
      <form>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{ $t("settings.sources.list.label") }}</span>
          </label>
          <div class="flex flex-col gap-y-2 min-w-0">
            <div class="atv-saved-source" v-for="saved in savedSources" :key="saved.id">
              <div class="flex flex-col min-w-0">
                <span class="font-semibold break-words">{{ saved.name }}</span>
                <span class="text-xs text-base-content/70 break-all">{{ saved.url }}</span>
              </div>
              <Popper placement="top" arrow="true">
                <template #content="{ close }">
                  <div class="flex flex-col gap-y-2 max-w-xs">
                    <div class="py-2 text-left break-normal break-words">
                      {{ $t("settings.sources.delete_confirm", { name: saved.name }) }}
                    </div>
                    <div class="flex gap-x-2 justify-end items-center">
                      <a class="link link-primary link-hover" @click="close">{{
                        $t("settings.sources.button.cancel")
                      }}</a>
                      <button type="button" class="btn btn-primary btn-xs" @click="deleteSavedSource(saved, close)">
                        {{ $t("settings.sources.button.confirm") }}
                      </button>
                    </div>
                  </div>
                </template>
                <button type="button" class="btn atv-action atv-action--danger">
                  {{ $t("settings.sources.button.delete") }}
                </button>
              </Popper>
            </div>
            <span class="text-sm text-base-content/70" v-if="!savedSources.length">{{
              $t("settings.sources.list.empty")
            }}</span>
          </div>
        </div>

        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{ $t("settings.sources.add.label") }}</span>
          </label>
          <div class="flex flex-col grow min-w-0">
            <div class="join w-full atv-saved-source-add">
              <input
                v-model="newSourceUrl"
                type="text"
                class="input input-bordered join-item flex-1 min-w-0"
                placeholder="https://example.com/apps.json"
                autocomplete="off"
                autocapitalize="off"
                spellcheck="false"
                @keydown.enter.prevent="addSavedSource"
              />
              <button
                type="button"
                class="btn btn-primary join-item"
                :disabled="addingSource || !newSourceUrl.trim()"
                @click="addSavedSource"
              >
                <span class="loading loading-spinner loading-sm" v-show="addingSource"></span>
                {{ $t("settings.sources.button.add") }}
              </button>
            </div>
            <label class="label">
              <span class="label-text-alt whitespace-normal">{{ $t("settings.sources.tips") }}</span>
            </label>
          </div>
        </div>
      </form>
    </section>

    <section class="section bg-base-100">
      <h2 class="atv-section-heading">{{ $t("settings.network.title") }}</h2>
      <form>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{ $t("settings.network.proxy_toggle.label") }}</span>
          </label>
          <input
            type="checkbox"
            class="toggle toggle-success"
            v-model="settings.network.proxy_enabled"
          />
        </div>

        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{ $t("settings.network.http_proxy.label") }}</span>
          </label>
          <input
            v-model="settings.network.http_proxy"
            type="text"
            :placeholder="$t('settings.network.http_proxy.placeholder')"
            class="input input-bordered grow"
          />
        </div>

        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{ $t("settings.network.https_proxy.label") }}</span>
          </label>
          <input
            v-model="settings.network.https_proxy"
            type="text"
            :placeholder="$t('settings.network.https_proxy.placeholder')"
            class="input input-bordered grow"
          />
        </div>

        <div class="form-item">
          <label class="form-item-label"></label>
          <div class="flex-1 flex justify-between">
            <button class="btn btn-primary w-32" @click.prevent="saveNetwork">
              {{ $t("settings.network.button.submit") }}
            </button>
          </div>
        </div>
      </form>
    </section>
    <section class="section bg-base-100">
      <h2 class="atv-section-heading">{{ $t('settings.advanced.title') }}</h2>
      <form>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">{{ $t('settings.advanced.update_coreadi') }}</span>
          </label>
          <div class="form-item-content">
            <Popper placement="top" arrow="true">
              <template #content="{ close }">
                <div class="flex flex-col gap-y-2">
                  <div class="py-2">{{ $t('settings.advanced.update_coreadi_confirm') }}</div>
                  <div class="flex gap-x-2 justify-end items-center">
                    <a class="link link-primary link-hover" @click="close">{{ $t('settings.advanced.cancel') }}</a>
                    <button class="btn btn-primary" @click.prevent="confirmUpdateCoreADI(close)">{{ $t('settings.advanced.confirm') }}</button>
                  </div>
                </div>
              </template>
              <button type="button" class="btn btn-error w-48" :disabled="advanced.adiLoading">
                <span class="loading loading-spinner" v-show="advanced.adiLoading"></span>{{ $t('settings.advanced.update_coreadi') }}
              </button>
            </Popper>
          </div>
        </div>
      </form>
    </section>
  </div>
</template>
          

<script>
import api from "@/api/api";
import { toast } from "vue3-toastify";
import Bark from "./components/Bark.vue";
import Telegram from "./components/Telegram.vue";
import Weixin from "./components/Weixin.vue";
import Webhook from "./components/Webhook.vue";

export default {
  name: "Home",
  components: {
    Bark,
    Telegram,
    Weixin,
    Webhook,
  },
  data() {
    return {
      startHour: 0,
      endHour: 23,
      updateIntervals: [0, 1, 3, 6, 12, 24],
      settings: {
        task: {},
        update: {
          check_interval: 6,
        },
        notification: {
          type: "bark",
          telegram: {},
          weixin: {},
          bark: {},
          webhook: {},
        },
        network: {
          proxy_enabled: false,
          http_proxy: "",
          https_proxy: "",
        },
      },
      advanced: {
        adiLoading: false,
      },
      savedSources: [],
      newSourceUrl: "",
      addingSource: false,
    };
  },

  created() {
    this.fetchData();
    this.fetchSavedSources();
  },
  methods: {
    fetchData() {
      let _this = this;
      api.getSettings().then((res) => {
        _this.settings = res.data;
        _this.parseCronTime();
      });
    },

    saveNotification() {
      let _this = this;

      api.saveNotificationSettings(_this.settings).then((res) => {
        if (res.data) {
          toast.success(this.$t("settings.toast.save_success"));
        }
      });
    },

    saveTask() {
      let _this = this;

      api.saveTaskSettings(_this.settings).then((res) => {
        if (res.data) {
          toast.success(this.$t("settings.toast.save_success"));
        }
      });
    },

    saveUpdate() {
      let _this = this;

      api.saveUpdateSettings(_this.settings).then((res) => {
        if (res.data) {
          toast.success(this.$t("settings.toast.save_success"));
        }
      });
    },

    fetchSavedSources() {
      api.getSavedSources().then((res) => {
        this.savedSources = res.data || [];
      });
    },

    addSavedSource() {
      const url = this.newSourceUrl.trim();
      if (!url || this.addingSource) return;

      this.addingSource = true;
      api
        .addSavedSource({ url })
        .then((res) => {
          toast.success(this.$t("settings.sources.toast.added", { name: res.data.name }));
          this.newSourceUrl = "";
          this.fetchSavedSources();
        })
        .catch((err) => {
          // request.js already shows the error.
          console.error(err);
        })
        .finally(() => {
          this.addingSource = false;
        });
    },

    deleteSavedSource(saved, closePopper) {
      closePopper?.();
      api.deleteSavedSource(saved.id).then(() => {
        toast.success(this.$t("settings.sources.toast.deleted", { name: saved.name }));
        this.fetchSavedSources();
      });
    },

    formatUpdateInterval(hours) {
      if (hours === 0) {
        return this.$t("settings.update.interval.off");
      }
      if (hours === 1) {
        return this.$t("settings.update.interval.every_hour");
      }
      return this.$t("settings.update.interval.every_hours", { num: hours });
    },

    testNotification() {
      let _this = this;

      api.sendTestNotify(_this.settings).then((res) => {
        if (res.data) {
          toast.success(this.$t("settings.toast.notify_success"));
        }
      });
    },

    saveNetwork() {
      let _this = this;

      api.saveNetworkSettings(_this.settings).then((res) => {
        if (res.data) {
          toast.success(this.$t("settings.toast.save_success"));
        }
      });
    },

    confirmUpdateCoreADI(close) {
      let _this = this;
      close && close();

      _this.advanced.adiLoading = true;
      api.updateCoreADI()
        .then((res) => {
          if (res.data) {
            toast.success(this.$t('settings.advanced.toast.success'));
          } else {
            toast.error(this.$t('settings.advanced.toast.failed'));
          }
        })
        .catch((err) => {
          console.error(err);
          toast.error(this.$t('settings.advanced.toast.failed'));
        })
        .finally(() => {
          _this.advanced.adiLoading = false;
        });
    },

    parseCronTime() {
      if (!this.settings.task.crod_time) return;
      try {
        const parts = this.settings.task.crod_time.split(" ");
        if (parts.length < 2) return;
        const hour = parts[1];
        if (hour.includes(",")) {
          // 22-23,0-2 or 22,23,0,1,2
          const hours = hour.split(",");
          let firstPart = hours[0];
          let lastPart = hours[hours.length - 1];

          if (firstPart.includes("-")) {
            this.startHour = parseInt(firstPart.split("-")[0]);
          } else {
            this.startHour = parseInt(firstPart);
          }

          if (lastPart.includes("-")) {
            this.endHour = parseInt(lastPart.split("-")[1]);
          } else {
            this.endHour = parseInt(lastPart);
          }
        } else if (hour.includes("-")) {
          const [start, end] = hour.split("-");
          this.startHour = parseInt(start);
          this.endHour = parseInt(end);
        } else if (hour !== "*") {
          this.startHour = parseInt(hour);
          this.endHour = parseInt(hour);
        } else {
          // * or invalid
          this.startHour = 0;
          this.endHour = 23; 
        }
      } catch (e) {
        console.error(e);
      }
    },

    onTaskEnabledChange() {
      if (!this.settings.task.enabled) {
        this.settings.task.iphone_enabled = false;
      }
    },

    updateCronTime() {
        let h = "";
        const start = parseInt(this.startHour);
        const end = parseInt(this.endHour);

        if (start === end) {
            h = `${start}`;
        } else if (start < end) {
            h = `${start}-${end}`;
        } else {
            // Cross-day time range (e.g., 22:00 to 02:00)
            h = `${start}-23,0-${end}`;
        }
        this.settings.task.crod_time = `0,30 ${h} * * *`;
    },
  },
};
</script>

<style lang="postcss" scoped>
.section {
  border: 1px solid var(--atv-border);
  border-radius: 22px;
  padding: 30px;
  margin-bottom: 34px;
}


form {
  @apply flex flex-col gap-y-5;
}

:deep(.form-item) {
  display: grid;
  grid-template-columns: minmax(150px, 220px) minmax(0, 1fr);
  gap: 16px 20px;
  align-items: center;
}

:deep(.form-item-label) {
  @apply label;
  width: auto;
  justify-content: flex-start;
  padding-right: 0;
  font-weight: 650;
  color: var(--atv-muted);
}

:deep(.form-item-content) {
  width: 100%;
  max-width: none;
}

.form-item-content :deep(.input),
.form-item-content :deep(.select),
.form-item-content :deep(.textarea) {
  width: 100% !important;
  min-height: 46px;
}

.section :deep(.btn.w-48) {
  width: 11.5rem;
  justify-content: center;
}

.section :deep(.btn) {
  box-shadow: none;
}

.section :deep(a) {
  color: var(--atv-accent);
  font-weight: 650;
  text-underline-offset: 3px;
}

.section :deep(a:hover) {
  color: var(--atv-accent-hover);
}

.atv-saved-source {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--atv-border);
  border-radius: 12px;
}

/* Keep the URL input and the Add button fused like other join rows. */
.atv-saved-source-add > .join-item {
  height: auto;
  min-height: 46px;
  border-radius: 0;
}

.atv-saved-source-add > .join-item:first-child {
  border-start-start-radius: var(--rounded-btn, 0.5rem);
  border-end-start-radius: var(--rounded-btn, 0.5rem);
}

.atv-saved-source-add > .join-item:last-child {
  border-start-end-radius: var(--rounded-btn, 0.5rem);
  border-end-end-radius: var(--rounded-btn, 0.5rem);
}

:deep(.popper) {
  background: var(--atv-surface);
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid var(--atv-border);
  box-shadow: var(--atv-menu-shadow);
  color: var(--atv-ink);
  word-break: break-all;
  text-align: justify;
  min-width: 150px;
}

:deep(.popper:hover),
:deep(.popper:hover > #arrow::before) {
  background: var(--atv-surface);
}

:deep(.popper #arrow::before) {
  background: var(--atv-surface);
}

@media (max-width: 980px) {
  .section {
    padding: 20px;
  }

  :deep(.form-item) {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  :deep(.form-item-label) {
    padding-bottom: 0;
  }

  .section :deep(.btn.w-48) {
    width: 100%;
  }
}
</style>