<template>
  <div class="max-w-screen-lg mx-auto">
    <fieldset class="section">
      <legend>通知</legend>
      <form>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">启用</span>
          </label>
          <input
            type="checkbox"
            class="toggle toggle-success"
            v-model="settings.notification.enabled"
          />
        </div>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">通知方式</span>
          </label>

          <div class="flex gap-x-2">
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
              <span class="label-text">企业微信</span>
            </label>
          </div>
        </div>
        <div class="form-item" v-show="settings.notification.type == 'weixin'">
          <label class="form-item-label">
            <span class="label-text">企业ID (CorpId)</span>
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
            <span class="label-text">密钥 (Corpsecret)</span>
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
            <span class="label-text">应用ID (AgentId)</span>
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
            <span class="label-text">指定成员 (touser)</span>
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

        <div class="form-item">
          <label class="form-item-label"></label>
          <div class="flex-1 flex justify-between">
            <button
              class="btn btn-primary w-32"
              @click.prevent="saveNotification"
            >
              保 存
            </button>

            <a
              class="link"
              @click.prevent="testNotification"
              v-show="settings.notification.enabled"
              >推送测试通知</a
            >
          </div>
        </div>
      </form>
    </fieldset>

    <fieldset class="section">
      <legend>自动刷新 App</legend>
      <form>
        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">启用</span>
          </label>
          <input
            type="checkbox"
            class="toggle toggle-success"
            v-model="settings.task.enabled"
          />
        </div>

        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text">刷新模式</span>
          </label>

          <div class="flex gap-x-2">
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.task.mode"
                value="1"
              />
              <span class="label-text">过期前一天</span>
            </label>
            <label class="label cursor-pointer flex gap-x-1">
              <input
                type="radio"
                class="radio"
                v-model="settings.task.mode"
                value="2"
              />
              <span class="label-text">每天</span>
            </label>
          </div>
        </div>

        <div class="form-item">
          <label class="form-item-label">
            <span class="label-text mb-8">运行时间段</span>
          </label>
          <div class="flex flex-col grow">
            <input
              v-model="settings.task.crod_time"
              type="text"
              placeholder=""
              class="input input-bordered"
            />
            <label class="label">
              <span class="label-text-alt"
                >linux crontab格式，受刷新模式限制</span
              >
            </label>
          </div>
        </div>

        <div class="form-item">
          <label class="form-item-label"></label>
          <div class="flex-1 flex justify-between">
            <button class="btn btn-primary w-32" @click.prevent="saveTask">
              保 存
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
      settings: {
        task: {},
        notification: {
          type: "bark",
          telegram: {},
          weixin: {},
          bark: {},
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
      });
    },

    saveNotification() {
      let _this = this;

      api.saveNotificationSettings(_this.settings).then((res) => {
        if (res.data) {
          toast.success("保存成功");
        }
      });
    },

    saveTask() {
      let _this = this;

      api.saveTaskSettings(_this.settings).then((res) => {
        if (res.data) {
          toast.success("保存成功");
        }
      });
    },

    testNotification() {
      let _this = this;

      api.sendTestNotify(_this.settings).then((res) => {
        if (res.data) {
          toast.success("推送成功");
        }
      });
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
  