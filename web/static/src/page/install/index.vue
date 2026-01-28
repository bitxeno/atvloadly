<template>
  <div class="max-w-screen-md mx-auto flex flex-col gap-y-6">
    <div class="alert alert-warning">
      <div class="w-8">
        <WarningIcon />
      </div>
      <span class="text-sm">{{ $t("install.tips.warning") }}</span>
    </div>

    <div class="border rounded p-6 bg-base-100">
      <div class="lg:flex lg:flex-row">
        <div class="flex flex-col justify-center place-items-center gap-y-4">
          <div class="w-32 rounded">
            <IPhoneIcon v-if="isIPhone(device)" />
            <AppleTVIcon v-else />
          </div>
          <div class="flex flex-col gap-y-2 items-center justify-center">
            <span>{{ device.name }}</span>
            <span>({{ device.ip }})</span>
          </div>
        </div>

        <div class="divider divider-horizontal"></div>

        <div class="p-6 flex flex-col gap-y-4">
          <form id="form" class="flex flex-col gap-y-4">
            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">{{
                  $t("install.form.choose_ipa.label")
                }}</span>
              </label>
              <input
                type="file"
                class="file-input file-input-bordered w-full"
                @change="onFileChange"
                accept=".ipa,.tipa"
                required
              />
            </div>

            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">{{
                  $t("install.form.account.label")
                }}</span>
              </label>
              <div class="join flex w-full">
                <select
                  class="select select-bordered join-item flex-1 w-full"
                  v-model="form.account"
                  required
                >
                  <option value="" disabled selected>
                    {{ $t("install.form.account.select.placeholder") }}
                  </option>
                  <option
                    v-for="account in accounts"
                    :key="account.email"
                    :value="account.email"
                  >
                    {{ account.email }} ({{ account.status }})
                  </option>
                </select>
                <button class="btn join-item w-16" @click.prevent="showLoginDialog">
                  <div class="w-6 h-6">
                    <PersonIcon />
                  </div>
                </button>
              </div>
              <label class="label">
                <span class="label-text-alt stat-title">{{
                  $t("install.form.account.alt")
                }}</span>
              </label>
            </div>

            <div class="form-control">
              <label class="label cursor-pointer justify-between items-center gap-x-4">
                <div class="flex items-center">
                  <span class="label-text">{{
                    $t("install.form.extensions.remove_extensions")
                  }}</span>
                  <div class="tooltip" :data-tip="$t('install.form.extensions.tips')">
                    <div class="w-4 h-4 text-secondary-content"><HelpIcon /></div>
                  </div>
                </div>
                <input
                  type="checkbox"
                  class="toggle toggle-success"
                  v-model="form.remove_extensions"
                />
              </label>
            </div>

          </form>

          <div class="flex flex-row gap-x-4">
            <button class="btn flex-1" @click="goBack">
              {{ $t("install.form.button.back") }}
            </button>
            <button
              class="btn btn-primary flex-1"
              @click="onSubmit"
              :disabled="loading"
            >
              <span class="loading loading-spinner" v-show="loading"></span
              >{{ $t("install.form.button.submit") }}
            </button>
          </div>
        </div>
      </div>

      <Login ref="loginModal" @success="fetchData" />
    </div>

    <div v-show="log.show">
      <textarea
        id="log"
        class="textarea textarea-bordered w-full h-48 bg-neutral text-base-100 leading-5"
        wrap="off"
        v-model="log.output"
      ></textarea>
    </div>
  </div>
</template>
  
  <script>
import api from "@/api/api";
import { toast } from "vue3-toastify";
import { maskEmail } from "@/utils/utils";
import Login from "@/components/Login.vue";

export default {
  components: { Login },
  data() {
    return {
      id: "",
      files: [],
      ipa: {},
      device: {},
      loading: false,
        accounts: [],
      form: {
        account: "",
        password: "",
        remove_extensions: false,
      },
      log: {
        newcontent : "",
        output: "",
        show: false,
      },
      
      refreshLogInterval: null,
    };
  },
  created() {
    this.id = this.$route.params.id;

    this.fetchData();
  },
  mounted() {
    this.initWebSocket();
  },
  unmounted() {
    this.closeWebSocket();
    this.stopUpdateLog();
  },
  methods: {
    fetchData() {
      let _this = this;
      api.getDevice(_this.id).then((res) => {
        _this.device = res.data;
      });
      api.getAccounts().then((res) => {
        const m = res.data || {};
        _this.accounts = Object.keys(m).map((k) => m[k]);
      }).catch(() => {
        _this.accounts = [];
      });
    },
    async onSubmit(e) {
      let _this = this;

      if (!_this.validateForm("#form")) {
        return;
      }

     
      _this.loading = true;
      _this.log.output = "";
      _this.log.newcontent = "";
      _this.log.show = true;

      _this.stopUpdateLog();
      _this.startUpdateLog();
      _this.log.output += "checking device status...\n";
      try {
        _this.log.output += `product type: ${_this.device.product_type}\n`;
        _this.log.output += `product version: ${_this.device.product_version}\n`;
        let devmode = await api.checkDeveloperMode(_this.id);
        _this.log.output += `developer mode: ${devmode.enabled ? "enabled" : "disabled"}${devmode.mounted ? " (mounted)" : ""}\n`;

        await _this.checkAfcService(_this.id);

        let formData = new FormData();
        for (let i = 0; i < _this.files.length; i++) {
          let file = _this.files[i];
          formData.append("files", file);
        }
        _this.log.output += "IPA uploading...\n";
        let data = await api.upload(formData)
        let ipa = data[0];
        _this.ipa = ipa;
        // send start install msg
        _this.websocketsend(1, {
            ID: 0,
            ipa_name: _this.ipa.name,
            ipa_path: _this.ipa.path,
            device: _this.device.mac_addr,
            device_class: _this.device.device_class,
            udid: _this.device.udid,
            account: _this.form.account,
            password: _this.form.password,
            icon: _this.ipa.icon,
            bundle_identifier: _this.ipa.bundle_identifier,
            version: _this.ipa.version,
            remove_extensions: _this.form.remove_extensions,
        });
      } catch (error) {
        console.log(error);
        _this.log.newcontent += error;
        _this.loading = false;
        toast.error(this.$t("install.toast.install_failed"));
        return;
      }
    },
    reset() {
      document.getElementById("form").reset();
    },
    goBack() {
      this.$router.push("/");
    },
    onFileChange(e) {
      this.files = e.target.files;
    },
    validateForm(id) {
      let form = document.querySelector(id);
      if (!form) {
        throw new Error(`not found form: ${id}`);
      }
      if (!form.checkValidity()) {
        form.reportValidity();
        return false;
      }
      return true;
    },
    showLoginDialog() {
        this.$refs.loginModal.show();
    },
    initWebSocket() {
      //初始化weosocket
      const wsuri =
        (location.protocol === "https:" ? "wss://" : "ws://") +
        location.host +
        "/ws/install"; //ws地址
      console.log(wsuri);
      this.websock = new WebSocket(wsuri);
      this.websock.onopen = this.websocketonopen;
      this.websock.onerror = this.websocketonerror;
      this.websock.onmessage = this.websocketonmessage;
      this.websock.onclose = this.websocketclose;
    },
    closeWebSocket() {
      this.websock.close();
    },

    websocketonopen() {
      console.log("WebSocket connect success.");
    },
    websocketonerror(e) {
      console.log("WebSocket connect failed.");
    },
    websocketonmessage(e) {
      let _this = this;
      // hide password string
      let line = e.data.replace(_this.form.password, "******");

      if (line.indexOf("sealing regular file") !== -1) {
        return;
      }

      // append new log content
      _this.log.newcontent += line;


      // Installation successful.
      if (line.indexOf("Installation Succeeded") !== -1 || line.indexOf("Installation complete") !== -1) {
        _this.loading = false;
        toast.success(this.$t("install.toast.install_success"));
        return;
      }


      // Installation error
      if (line.indexOf("Installation Failed") !== -1) {
        _this.loading = false;
        toast.error(_this.$t("install.toast.install_failed"));
        return;
      }
    },

    websocketsend(t, data) {
      let _this = this;
      if (typeof data !== 'string') {
        data = JSON.stringify(data);
      }
      const json = JSON.stringify({ t: t, d: data });
      console.log("--> ", json);
      if (_this.websock.readyState !== WebSocket.OPEN) {
        throw new Error("WebSocket is in CLOSING or CLOSED state.");
      }
      _this.websock.send(json);
    },

    websocketclose(e) {
      console.log(`connection closed (${e.code})`);
    },
    async checkAfcService(id) {
      let _this = this;
      try {
        await api.checkAfcService(id);
        _this.log.newcontent += "afc service: OK!\n";
      } catch (error) {
        _this.log.newcontent += `afc service: Failed!\n`;
        throw error;
      }
    },
    startUpdateLog() {
      let _this = this;
      _this.refreshLogInterval = setInterval(() => {
        if (_this.log.newcontent !== '') {
          _this.log.output += _this.log.newcontent;
          _this.log.newcontent = '';
          // The textbox follows the scroll to the bottom
          _this.$nextTick(() => {
            const textarea = document.querySelector("#log");
            textarea.scrollTop = textarea.scrollHeight;
          });
        }
      }, 500);
    },
    stopUpdateLog() {
      let _this = this;
      if (_this.refreshLogInterval) {
        clearInterval(_this.refreshLogInterval);
        _this.refreshLogInterval = null;
      }
    },
    isIPhone(item) {
      if (item.device_class) {
        return item.device_class.toLowerCase() == "iphone" || item.device_class.toLowerCase() == "ipad";
      }
      return item.name && (item.name.toLowerCase().includes("iphone") || item.name.toLowerCase().includes("ipad"));
    },
  },
};
</script>

<script setup>
import AppleTVIcon from "@/assets/icons/appletv.svg";
import IPhoneIcon from "@/assets/icons/iphone.svg";
import WarningIcon from "@/assets/icons/warning.svg";
import PersonIcon from "@/assets/icons/person.badge.plus.svg";
import HelpIcon from "@/assets/icons/help.svg";
</script>
  
  <style scoped>
.line {
  text-align: center;
}
</style>
  