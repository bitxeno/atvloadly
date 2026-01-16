<template>
  <div class="max-w-screen-lg mx-auto">
    <fieldset class="section bg-base-100">
      <legend>{{ $t("settings.notification.title") }}</legend>
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
        <div class="form-item" v-show="settings.notification.type == 'weixin'">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.weixin.corp_id")
            }}</span>
          </label>
          <input
            v-model="settings.notification.weixin.corp_id"
            type="text"
            placeholder=""
            class="input input-bordered grow"
          />
        </div>
        <div class="form-item" v-show="settings.notification.type == 'weixin'">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.weixin.corp_secret")
            }}</span>
          </label>
          <input
            v-model="settings.notification.weixin.corp_secret"
            type="text"
            placeholder=""
            class="input input-bordered grow"
          />
        </div>
        <div class="form-item" v-show="settings.notification.type == 'weixin'">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.weixin.agent_id")
            }}</span>
          </label>
          <input
            v-model="settings.notification.weixin.agent_id"
            type="text"
            placeholder=""
            class="input input-bordered grow"
          />
        </div>
        <div class="form-item" v-show="settings.notification.type == 'weixin'">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.weixin.touser")
            }}</span>
          </label>
          <input
            v-model="settings.notification.weixin.to_user"
            type="text"
            placeholder=""
            class="input input-bordered grow"
          />
        </div>

        <div
          class="form-item"
          v-show="settings.notification.type == 'telegram'"
        >
          <label class="form-item-label">
            <span class="label-text">BotToken</span>
          </label>
          <input
            v-model="settings.notification.telegram.bot_token"
            type="text"
            placeholder=""
            class="input input-bordered grow"
          />
        </div>
        <div
          class="form-item"
          v-show="settings.notification.type == 'telegram'"
        >
          <label class="form-item-label">
            <span class="label-text">ChatID</span>
          </label>
          <input
            v-model="settings.notification.telegram.chat_id"
            type="text"
            placeholder=""
            class="input input-bordered grow"
          />
        </div>

        <div class="form-item" v-show="settings.notification.type == 'bark'">
          <label class="form-item-label">
            <span class="label-text">Device Key</span>
          </label>
          <input
            v-model="settings.notification.bark.device_key"
            type="text"
            placeholder=""
            class="input input-bordered grow"
          />
        </div>
        <div class="form-item" v-show="settings.notification.type == 'bark'">
          <label class="form-item-label">
            <span class="label-text">Bark Server</span>
          </label>
          <input
            v-model="settings.notification.bark.bark_server"
            type="text"
            placeholder=""
            class="input input-bordered grow"
          />
        </div>

        <div class="form-item" v-show="settings.notification.type == 'webhook'">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.webhook.url")
            }}</span>
          </label>
          <input
            v-model="settings.notification.webhook.url"
            type="url"
            :placeholder="$t('settings.notification.webhook.url_placeholder')"
            class="input input-bordered grow"
          />
        </div>
        <div class="form-item" v-show="settings.notification.type == 'webhook'">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.webhook.method")
            }}</span>
          </label>
          <select
            v-model="settings.notification.webhook.method"
            class="select select-bordered grow"
          >
            <option value="GET">GET</option>
            <option value="POST">POST</option>
          </select>
        </div>
        <div class="form-item" v-show="settings.notification.type == 'webhook' && settings.notification.webhook.method == 'POST'">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.webhook.content_type")
            }}</span>
          </label>
          <select
            v-model="settings.notification.webhook.content_type"
            class="select select-bordered grow"
          >
            <option value="text/plain">text/plain</option>
            <option value="application/json">application/json</option>
          </select>
        </div>
        <div class="form-item" v-show="settings.notification.type == 'webhook' && settings.notification.webhook.method == 'POST'">
          <label class="form-item-label">
            <span class="label-text">{{
              $t("settings.notification.webhook.body")
            }}</span>
          </label>
          <textarea
            v-model="settings.notification.webhook.body"
            :placeholder="$t('settings.notification.webhook.body_placeholder')"
            class="textarea textarea-bordered grow"
            rows="4"
          ></textarea>
        </div>

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
    </fieldset>

    <fieldset class="section bg-base-100">
      <legend>{{ $t("settings.refresh.title") }}</legend>
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
    </fieldset>

    <fieldset class="section bg-base-100">
      <legend>{{ $t("settings.network.title") }}</legend>
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
    </fieldset>
  </div>
</template>
          

<script>
import api from "@/api/api";
import { toast } from "vue3-toastify";

export default {
  name: "Home",
  data() {
    return {
      startHour: 0,
      endHour: 23,
      settings: {
        task: {},
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
    };
  },

  created() {
    this.fetchData();
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

      api.saveTaskSettings(_this.settings).then((res) => {
        if (res.data) {
          toast.success(this.$t("settings.toast.save_success"));
        }
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
  border: 1px solid #ebebeb;
  border-radius: 5px;
  padding: 28px;
  margin-bottom: 30px;
}

legend {
  -webkit-box-sizing: border-box;
  box-sizing: border-box;
  color: inherit;
  display: table;
  max-width: 100%;
  padding: 0;
  white-space: normal;
}

form {
  @apply flex flex-col gap-y-4;
}

.form-item {
  @apply form-control w-full lg:flex lg:flex-row lg:items-center;
}
.form-item-label {
  @apply label w-48 lg:justify-end pr-4 font-medium;
}
.form-item-content {
  @apply grow;
}
</style>
  