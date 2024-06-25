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
      cmd: {
        output: "",
        line: "",
      },
      log: {
        output: "",
        show: false,
      },
      dialogVisible: false,
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
      _this.log.show = true;

      _this.log.output += "checking afc service status...\n";
      let data = await api.checkAfcService(_this.id);
      if (data != "success") {
        _this.log.output += data;
        _this.cmd.output = "";
        _this.loading = false;
        toast.error(this.$t("install.toast.install_failed"));
        return;
      }
      _this.log.output += "afc service OK!\n";

  
      let formData = new FormData();
      for (let i = 0; i < _this.files.length; i++) {
        let file = _this.files[i];
        formData.append("files", file);
      }
      _this.log.output += "IPA uploading...\n";
      api
        .upload(formData)
        .then((res) => {
          let ipa = res.data[0];
          _this.ipa = ipa;
          _this.websocketsend(
            `sideloader install --nocolor --udid ${_this.device.udid} -a '${_this.form.account}' -p '${_this.form.password}' '${ipa.path}'`
          );
        })
        .catch((error) => {
          console.log(error);
          _this.log.output += error;
          _this.loading = false;
          toast.error(this.$t("install.toast.install_failed"));
          return;
        });
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
        "/ws/tty"; //ws地址
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

      _this.websocketsend(_this.form.authcode);
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

      _this.cmd.output += e.data;
      _this.cmd.line += e.data;
      if (e.data.indexOf("\n") >= 0) {
        // hide password string for security
        _this.cmd.output = _this.cmd.output.replace(
          _this.form.password,
          "******"
        );
        _this.cmd.line = _this.cmd.line.replace(_this.form.password, "******");

        // ignore slideloader command output
        if (
          _this.cmd.line.indexOf("sideloader") === -1
        ) {
          _this.log.output += _this.cmd.line;
          // The textbox follows the scroll to the bottom
          _this.$nextTick(() => {
            const textarea = document.querySelector("#log");
            textarea.scrollTop = textarea.scrollHeight;
          });
        }

        _this.cmd.line = "";
      }

      // input 2FA authentication code
      if (_this.cmd.output.indexOf("A code has been sent to your devices, please type it here") !== -1) {
        _this.cmd.output = "";
        _this.dialogVisible = true;
        return;
      }


      // pairing error
      if (_this.cmd.output.indexOf("ERROR") !== -1) {
        _this.cmd.output = "";
        _this.loading = false;
        toast.error(this.$t("install.toast.install_failed"));
        return;
      }

      // Installation successful.
      if (_this.cmd.output.indexOf("Installation Succeeded") !== -1) {
        _this.cmd.output = "";
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

    websocketsend(cmd) {
      let _this = this;
      _this.cmd.output = "";
      const json = JSON.stringify({ t: 1, d: `${cmd}\n` });
      console.log("--> ", json);
      _this.websock.send(json);
    },

    websocketclose(e) {
      console.log(`connection closed (${e.code})`);
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
  