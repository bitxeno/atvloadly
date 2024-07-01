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
            <AppleTVIcon />
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
                accept=".ipa"
                required
              />
            </div>

            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">{{
                  $t("install.form.account.label")
                }}</span>
              </label>
              <input
                type="email"
                placeholder="xxxx@example.com"
                class="input input-bordered w-full"
                v-model="form.account"
                required
              />
              <label class="label">
                <span class="label-text-alt stat-title">{{
                  $t("install.form.account.alt")
                }}</span>
              </label>
            </div>

            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">{{
                  $t("install.form.password.label")
                }}</span>
              </label>
              <input
                type="password"
                class="input input-bordered w-full"
                v-model="form.password"
                required
              />
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

      <dialog
        id="auth_modal"
        :class="['modal', { 'modal-open': dialogVisible }]"
      >
        <form id="dialog" method="dialog" class="modal-box">
          <h3 class="font-bold text-lg">
            {{ $t("install.dialog.input_pin.title") }}
          </h3>
          <p class="py-4">
            <input
              type="number"
              class="input input-bordered input-primary w-full"
              :placeholder="$t('install.dialog.input_pin.input.placeholder')"
              v-model="form.authcode"
              required
            />
          </p>
          <div class="modal-action">
            <button class="btn btn-primary" @click="onSubmit2FA">
              {{ $t("install.dialog.input_pin.button.submit") }}
            </button>
          </div>
        </form>
      </dialog>
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

export default {
  data() {
    return {
      id: "",
      files: [],
      ipa: {},
      device: {},
      loading: false,
      form: {
        account: "",
        password: "",
        authcode: "",
      },
      log: {
        newcontent : "",
        output: "",
        show: false,
      },
      dialogVisible: false,
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
      _this.log.output += "checking afc service status...\n";
      try {
        await api.checkAfcService(_this.id);
        _this.log.output += "afc service OK!\n";

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
        _this.websocketsend(1, {'udid': _this.device.udid, 'account': _this.form.account, 'password': _this.form.password, 'ipa_path': ipa.path});
      } catch (error) {
        console.log(error);
        _this.log.output += error;
        _this.loading = false;
        _this.startUpdateLog();
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
    onSubmit2FA() {
      let _this = this;

      if (!_this.validateForm("#dialog")) {
        return;
      }

      _this.websocketsend(2, _this.form.authcode);
      _this.dialogVisible = false;
    },

    websocketonopen() {
      console.log("WebSocket connect success.");
    },
    websocketonerror(e) {
      console.log("WebSocket connect failed.");
    },
    websocketonmessage(e) {
      let _this = this;
      // hide password string for security
      let line = e.data.replace(_this.form.password, "******");
      
      // ignore slideloader command output
      if (line.indexOf("sideloader") === -1) {
        _this.log.newcontent += line;
      }

      // input 2FA authentication code
      if (line.indexOf("A code has been sent to your devices") !== -1) {
        _this.form.authcode = "";
        _this.dialogVisible = true;
        return;
      }


      // Installation error
      if (line.indexOf("ERROR") !== -1) {
        _this.loading = false;
        toast.error(this.$t("install.toast.install_failed"));

        // clean upload temp file
        api.clean(_this.ipa)
        return;
      }

      // Installation successful.
      if (line.indexOf("Installation Succeeded") !== -1) {
        _this.loading = false;

        // Save installation data.
        api
          .saveApp({
            ID: 0,
            ipa_name: _this.ipa.name,
            ipa_path: _this.ipa.path,
            device: _this.device.mac_addr,
            udid: _this.device.udid,
            account: _this.form.account,
            password: _this.form.password,
            refreshed_result: true,
            icon: _this.ipa.icon,
            bundle_identifier: _this.ipa.bundle_identifier,
            version: _this.ipa.version,
          })
          .then((res) => {
            toast.success(this.$t("install.toast.install_success"));
          });

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
      _this.websock.send(json);
    },

    websocketclose(e) {
      console.log(`connection closed (${e.code})`);
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
  },
};
</script>

<script setup>
import AppleTVIcon from "@/assets/icons/appletv.svg";
import WarningIcon from "@/assets/icons/warning.svg";
</script>
  
  <style scoped>
.line {
  text-align: center;
}
</style>
  