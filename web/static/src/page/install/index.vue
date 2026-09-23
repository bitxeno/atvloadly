<template>
  <div class="max-w-screen-md mx-auto flex flex-col gap-y-6 atv-install-page">
    <div class="alert atv-warning">
      <div class="w-8">
        <WarningIcon />
      </div>
      <span class="text-sm">{{ $t("install.tips.warning") }}</span>
    </div>

    <div class="card atv-install-card">
      <div class="lg:flex lg:flex-row">
        <div class="flex flex-col justify-center place-items-center gap-y-4 atv-install-device">
          <div class="relative w-32 rounded atv-install-device-icon">
            <IPhoneIcon v-if="isIPhone(device)" />
            <AppleTVIcon v-else />
            <button
              v-if="canShowScreenshotAction(device)"
              class="btn btn-circle btn-sm atv-install-screenshot-btn"
              @click="openScreenshotDialog"
              :disabled="screenshot.loading || !device.id"
              :title="$t('install.screenshot.action')"
              :aria-label="$t('install.screenshot.action')"
            >
              <span class="w-4 h-4"><CameraIcon /></span>
            </button>
          </div>
          <div class="flex flex-col gap-y-2 items-center justify-center">
            <span>{{ device.name }}</span>
            <span>({{ truncateIP(device.ip) }})</span>
          </div>
        </div>

        <div class="divider divider-horizontal"></div>

        <div class="p-6 flex flex-col gap-y-4 w-full max-w-lg atv-install-form-wrap">
          <form id="form" class="flex flex-col gap-y-4">
            <div class="join w-full atv-install-mode">
              <button
                type="button"
                class="btn join-item flex-1 gap-x-2"
                :class="{ 'btn-primary': installMode === 'file' }"
                :aria-pressed="installMode === 'file'"
                :aria-label="$t('install.form.mode.file')"
                :title="$t('install.form.mode.file')"
                @click="setInstallMode('file')"
              >
                <span class="w-6 h-6"><FolderOpenIcon /></span>
                <span class="hidden md:inline">{{ $t("install.form.mode.file") }}</span>
              </button>
              <button
                type="button"
                class="btn join-item flex-1 gap-x-2"
                :class="{ 'btn-primary': installMode === 'link' }"
                :aria-pressed="installMode === 'link'"
                :aria-label="$t('install.form.mode.link')"
                :title="$t('install.form.mode.link')"
                @click="setInstallMode('link')"
              >
                <span class="w-6 h-6"><LinkIcon /></span>
                <span class="hidden md:inline">{{ $t("install.form.mode.link") }}</span>
              </button>
              <button
                type="button"
                class="btn join-item flex-1 gap-x-2"
                :class="{ 'btn-primary': installMode === 'source' }"
                :aria-pressed="installMode === 'source'"
                :aria-label="$t('install.form.mode.source')"
                :title="$t('install.form.mode.source')"
                @click="setInstallMode('source')"
              >
                <span class="w-6 h-6"><GithubIcon /></span>
                <span class="hidden md:inline">{{ $t("install.form.mode.source") }}</span>
              </button>
            </div>

            <SourcePicker
              v-if="installMode === 'source'"
              :device-class="device.device_class"
              :initial="sourceInitial"
              @update:selection="onSourceSelection"
            />
            <div class="form-control w-full" v-else>
              <label class="label">
                <span class="label-text">
                  <template v-if="installMode === 'file'">{{ $t("install.form.choose_ipa.label") }}</template>
                  <template v-else>{{ $t("install.form.ipa_url.label") }}</template>
                </span>
              </label>
              <input
                v-if="installMode === 'file'"
                type="file"
                class="file-input file-input-bordered w-full"
                @change="onFileChange"
                accept=".ipa,.tipa"
                required
              />
              <input
                v-else
                type="url"
                class="input input-bordered w-full"
                v-model="ipaUrl"
                placeholder="https://example.com/app.ipa"
                required
                @input="onIpaUrlInput"
                @change="switchToSourceIfRepo"
              />
            </div>

            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">{{
                  $t("install.form.account.label")
                }}</span>
              </label>
              <div class="join w-full atv-install-account-picker">
                <select
                  class="select select-bordered join-item flex-1 min-w-0"
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
                    {{ account.email }}
                    {{
                      account.email === recommendedAccount
                        ? "(" + $t("install.form.account.last_used") + ")"
                        : "(" + accountStatusLabel(account.status) + ")"
                    }}
                  </option>
                </select>
                <button class="btn join-item" @click.prevent="showLoginDialog">
                  <div class="w-6 h-6">
                    <PersonIcon />
                  </div>
                </button>
              </div>
              <label class="label">
                <span class="label-text-alt block whitespace-normal break-words">{{
                  $t("install.form.account.alt")
                }}</span>
              </label>
            </div>

            <div class="form-control">
              <label class="label">
                <span class="label-text">{{ $t("install.form.custom_name.label") }}</span>
                <div class="tooltip" :data-tip="$t('install.form.custom_name.tips')">
                  <div class="w-4 h-4 text-secondary-content"><HelpIcon /></div>
                </div>
              </label>
              <input
                type="text"
                class="input input-bordered w-full"
                maxlength="64"
                :placeholder="$t('install.form.custom_name.placeholder')"
                v-model="form.custom_name"
              />
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

            <div class="form-control" v-if="installMode === 'source'">
              <label class="label cursor-pointer justify-between items-center gap-x-4">
                <div class="flex items-center">
                  <span class="label-text">{{
                    $t("install.form.source.auto_update")
                  }}</span>
                  <div class="tooltip" :data-tip="$t('install.form.source.auto_update_tips')">
                    <div class="w-4 h-4 text-secondary-content"><HelpIcon /></div>
                  </div>
                </div>
                <input
                  type="checkbox"
                  class="toggle toggle-success"
                  v-model="form.auto_update"
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
        class="textarea textarea-bordered w-full h-48 atv-install-log leading-5"
        wrap="off"
        v-model="log.output"
      ></textarea>
    </div>

    <!-- Screenshot Modal -->
    <dialog
      ref="screenshotModal"
      :class="['modal', { 'modal-open': screenshot.visible }]"
    >
      <div class="modal-box max-w-3xl">
        <button class="btn btn-sm btn-circle btn-ghost absolute right-2 top-2" @click="closeScreenshotDialog">✕</button>
        <h3 class="font-bold text-lg flex items-center gap-x-2">
          <span class="w-5 h-5"><CameraIcon /></span>
          {{ $t("install.screenshot.title") }}
        </h3>

        <div class="mt-4 flex flex-col gap-y-3">
          <div
            class="flex flex-row items-center gap-x-2 text-sm"
            v-show="screenshot.status"
          >
            <span
              class="loading loading-spinner loading-xs"
              v-show="screenshot.loading"
            ></span>
            <span class="text-base-content/70">{{ screenshot.status }}</span>
          </div>

          <div
            class="flex items-center justify-center bg-neutral rounded overflow-hidden mx-auto w-full max-w-[640px] aspect-[16/9]"
            :class="{ 'p-2': !screenshot.image }"
          >
            <img
              v-if="screenshot.image"
              :src="screenshot.image"
              :alt="$t('install.screenshot.title')"
              class="w-full h-full object-contain"
            />
            <span
              v-else-if="!screenshot.loading"
              class="text-base-100/70 text-sm"
            >
              {{ $t("install.screenshot.empty") }}
            </span>
          </div>
        </div>

        <div class="modal-action">
          <button
            class="btn gap-x-2"
            @click="downloadScreenshot"
            :disabled="!screenshot.image || screenshot.loading"
          >
            <span class="w-4 h-4"><DownloadIcon /></span>
            <span>{{ $t("install.screenshot.button.download") }}</span>
          </button>
          <button
            class="btn btn-primary gap-x-2"
            @click="captureScreenshot"
            :disabled="screenshot.loading"
          >
            <span class="w-4 h-4"><RefreshIcon /></span>
            <span>{{ $t("install.screenshot.button.refresh") }}</span>
          </button>
        </div>
      </div>
    </dialog>
  </div>
</template>
  
  <script>
import api from "@/api/api";
import { toast } from "vue3-toastify";
import { parseBundleIdFromPlist } from "@/utils/utils";
import { installFailureMessage as formatInstallFailureMessage } from "@/utils/install-error-feedback.mjs";
import { accountStatusLabel as formatAccountStatus } from "@/utils/install-feedback.mjs";
import { guessSourceKind } from "@/utils/source.mjs";
import JSZip from "jszip";
import Login from "@/components/Login.vue";
import SourcePicker from "@/components/SourcePicker.vue";

export default {
  components: { Login, SourcePicker },
  data() {
    return {
      id: "",
      installMode: "file",
      ipaUrl: "",
      files: [],
      // Source to prefill when a repository URL switched the page to source mode.
      sourceInitial: null,
      sourceSelection: null,
      ipa: {},
      device: {},
      loading: false,
      accounts: [],
      installedApps: [],
      recommendedAccount: "",
      form: {
        account: "",
        password: "",
        custom_name: "",
        remove_extensions: false,
        auto_update: false,
      },
      log: {
        newcontent : "",
        output: "",
        show: false,
      },

      refreshLogInterval: null,

      screenshot: {
        visible: false,
        loading: false,
        image: "",
        status: "",
      },
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
      api.getAppList().then((res) => {
        _this.installedApps = res.data || [];
      }).catch(() => {
        _this.installedApps = [];
      });
    },
    async onSubmit(e) {
      let _this = this;

      if (!_this.validateForm("#form")) {
        return;
      }
      if (_this.installMode === "source" && !_this.sourceSelection) {
        toast.error(this.$t("install.form.source.select_build"));
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
        _this.log.output += `connection mode: ${_this.device.connection}\n`;
        if (_this.device.connection === "Lockdown") {
          _this.log.output += `product type: ${_this.device.product_type}\n`;
          _this.log.output += `product version: ${_this.device.product_version}\n`;
          _this.log.output += `developer mode: ${_this.device.developer_mode_status ? "enabled" : "disabled"}\n`;
          _this.log.output += `personalized image: ${_this.device.personalized_image_mounted ? "mounted" : "not mounted"}\n`;

          await _this.checkAfcService(_this.id);
        }

        let ipa;
        let source;
        if (_this.installMode === "file") {
          let formData = new FormData();
          for (let i = 0; i < _this.files.length; i++) {
            let file = _this.files[i];
            formData.append("files", file);
          }
          _this.log.output += "IPA uploading...\n";
          let data = await api.upload(formData)
          ipa = data[0];
        } else if (_this.installMode === "link") {
          _this.log.output += "IPA URL: " + _this.ipaUrl + "\n";
          ipa = {
            name: _this.ipaUrl.split('/').pop() || 'remote.ipa',
            path: _this.ipaUrl,
            icon: '',
            bundle_identifier: '',
            version: '',
          };
        } else {
          const selection = _this.sourceSelection;
          const build = selection.build;
          _this.log.output += `Source: ${selection.url} ${build.version} ${build.name}\n`;
          // The server resolves the download URL from the source; path and name are for display.
          ipa = {
            name: build.name,
            path: build.download_url,
            icon: '',
            bundle_identifier: build.bundle_id,
            version: build.version,
          };
          source = {
            kind: selection.kind,
            url: selection.url,
            filter: selection.filter,
            prerelease: selection.prerelease,
            auto_update: _this.form.auto_update,
            build_id: build.id,
          };
        }
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
            custom_name: _this.form.custom_name.trim(),
            remove_extensions: _this.form.remove_extensions,
            source,
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
    setInstallMode(mode) {
      // The inputs of the current mode keep what they show: keep their state too.
      if (this.installMode === mode) return;
      this.installMode = mode;
      this.files = [];
      this.ipaUrl = "";
      this.sourceInitial = null;
      this.sourceSelection = null;
    },
    onIpaUrlInput(e) {
      if (e.inputType === "insertFromPaste") {
        this.switchToSourceIfRepo();
      }
    },
    // A GitHub repository is not an IPA link: open it as a tracked source instead.
    switchToSourceIfRepo() {
      const url = this.ipaUrl.trim();
      if (this.installMode !== "link" || guessSourceKind(url) !== "github") {
        return;
      }
      this.setInstallMode("source");
      this.sourceInitial = { kind: "github", url };
    },
    onSourceSelection(selection) {
      this.sourceSelection = selection;
      this.recommendedAccount = "";
      if (!selection) {
        return;
      }

      // Reuse the account of the app already installed from this source on this device.
      const url = selection.url.toLowerCase();
      const bundleId = selection.build.bundle_id;
      const app =
        this.installedApps.find((a) => a.udid === this.device.udid && a.source.url.toLowerCase() === url) ||
        (bundleId && this.installedApps.find((a) => a.bundle_identifier === bundleId));
      if (app) {
        this.recommendedAccount = app.account;
        this.form.account = app.account;
        if (app.custom_name) {
          this.form.custom_name = app.custom_name;
        }
      }
    },
    async onFileChange(e) {
      this.files = e.target.files;
      this.recommendedAccount = "";
      if (this.files.length > 0) {
        const file = this.files[0];
        try {
          const arrayBuffer = await file.arrayBuffer();
          const zip = await JSZip.loadAsync(arrayBuffer);

          const plistEntries = Object.keys(zip.files).filter((name) =>
            name.match(/^Payload\/[^/]+\.app\/Info\.plist$/)
          );

          for (const entry of plistEntries) {
            const plistData = await zip.files[entry].async("arraybuffer");
            const bundleId = parseBundleIdFromPlist(plistData);

            if (bundleId) {
              for (let app of this.installedApps) {
                if (app.bundle_identifier === bundleId) {
                  this.recommendedAccount = app.account;
                  this.form.account = app.account;
                  if (app.custom_name) {
                    this.form.custom_name = app.custom_name;
                  }
                  break;
                }
              }
              if (this.recommendedAccount) break;
            }
          }
        } catch (err) {
          console.error("Failed to read IPA bundle identifier:", err);
        }
      }
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
      let line = e.data;

      if (line.indexOf("sealing regular file") !== -1) {
        return;
      }

      // append new log content
      _this.log.newcontent += line;


      // Installation successful.
      if (line.indexOf("Installation Succeeded") !== -1) {
        _this.loading = false;
        toast.success(this.$t("install.toast.install_success"));
        return;
      }


      // Installation error
      if (line.indexOf("Installation Failed") !== -1) {
        _this.loading = false;
        toast.error(_this.installFailureMessage());
        return;
      }
    },

    installFailureMessage() {
      return formatInstallFailureMessage(
        this.log.output + this.log.newcontent,
        (key) => this.$t(key),
      );
    },
    
    accountStatusLabel(status) {
      return formatAccountStatus(status, (key) => this.$t(key));
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
        _this.log.output += "afc service: OK!\n";
      } catch (error) {
        _this.log.output += `afc service: Failed!\n`;
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
    isAppleTV(item) {
      if (item.device_class) {
        return item.device_class.toLowerCase() === "appletv";
      }
      return item.name && item.name.toLowerCase().includes("appletv");
    },
    canShowScreenshotAction(item) {
      return !!item && this.isAppleTV(item) && item.connection === "RPPairing";
    },
    async openScreenshotDialog() {
      if (!this.device || !this.device.id) {
        toast.error(this.$t("install.screenshot.toast.device_not_ready"));
        return;
      }
      this.screenshot.visible = true;
      this.screenshot.image = "";
      if (await this.mountDeviceImageAsync(this.device.id)) {
        this.captureScreenshot();
      }
    },
    closeScreenshotDialog() {
      this.screenshot.visible = false;
    },
    async mountDeviceImageAsync(deviceId) {
      try {
        this.screenshot.loading = true;
        this.screenshot.status = this.$t("install.screenshot.status.capturing");
        await api.mountDeviceImageAsync(deviceId);
        return true;
      } catch (error) {
        this.screenshot.status = this.$t("install.screenshot.status.failed");
        toast.error(error.message || this.$t("install.screenshot.toast.mount_failed"));
        this.screenshot.loading = false;
        return false;
      }
    },
    captureScreenshot() {
      if (!this.device || !this.device.id) {
        toast.error(this.$t("install.screenshot.toast.device_not_ready"));
        return;
      }
      this.screenshot.loading = true;
      this.screenshot.status = this.$t("install.screenshot.status.capturing");
      api
        .takeDeviceScreenshot(this.device.id)
        .then((res) => {
          const payload = (res && res.data) || {};
          if (payload.type === "screenshot" && payload.data) {
            this.screenshot.image = "data:image/jpeg;base64," + payload.data;
            this.screenshot.status = this.$t("install.screenshot.status.success");
          } else {
            this.screenshot.status = this.$t("install.screenshot.status.failed");
            toast.error(this.$t("install.screenshot.toast.failed"));
          }
        })
        .catch((err) => {
          this.screenshot.status = this.$t("install.screenshot.status.failed");
          // request.js already shows a toast on error; we just keep a status line.
          console.error("screenshot failed", err);
        })
        .finally(() => {
          this.screenshot.loading = false;
        });
    },
    downloadScreenshot() {
      if (!this.screenshot.image) {
        return;
      }
      const now = new Date();
      const pad = (n) => String(n).padStart(2, "0");
      const stamp =
        now.getFullYear() +
        pad(now.getMonth() + 1) +
        pad(now.getDate()) +
        "-" +
        pad(now.getHours()) +
        pad(now.getMinutes()) +
        pad(now.getSeconds());
      const filename = `screenshot-${stamp}.jpg`;
      const a = document.createElement("a");
      a.href = this.screenshot.image;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    },
  },
};
</script>

<script setup>
import { truncateIP } from "@/utils/utils";
import AppleTVIcon from "@/assets/icons/appletv.svg";
import IPhoneIcon from "@/assets/icons/iphone.svg";
import WarningIcon from "@/assets/icons/warning.svg";
import PersonIcon from "@/assets/icons/person.badge.plus.svg";
import HelpIcon from "@/assets/icons/help.svg";
import CameraIcon from "@/assets/icons/camera.svg";
import RefreshIcon from "@/assets/icons/refresh.svg";
import DownloadIcon from "@/assets/icons/download.svg";
import FolderOpenIcon from "@/assets/icons/folder-open.svg";
import LinkIcon from "@/assets/icons/link.svg";
import GithubIcon from "@/assets/icons/github.svg";
</script>
  
  <style scoped>
.line {
  text-align: center;
}

/* Install log follows the daisyUI mockup-code terminal look:
   background and text come from the active theme (--n / --nc).
   Placed on the page wrapper (higher specificity than the themed
   .textarea override in app.css) so it wins over the light field style. */
.atv-install-page .atv-install-log {
  background-color: hsl(var(--n) / var(--tw-bg-opacity, 1));
  border-color: hsl(var(--n) / var(--tw-bg-opacity, 1));
  color: hsl(var(--nc) / var(--tw-text-opacity, 1));
}

.atv-install-page .atv-install-log:focus {
  outline: 2px solid transparent;
  border-color: hsl(var(--nc) / 0.45);
  box-shadow: none;
}

/* Keep daisyUI join groups fused: the theme layer sets input/select/btn
   heights and radii that would otherwise split the joined edges apart. */
.atv-install-page .join {
  align-items: stretch;
}

.atv-install-page .join > .join-item:is(.input, .select, .file-input, .btn) {
  height: auto;
  min-height: 3rem;
  border-radius: 0;
}

.atv-install-page .join > .join-item:is(.input, .select, .file-input, .btn):first-child {
  border-start-start-radius: var(--rounded-btn, 0.5rem);
  border-end-start-radius: var(--rounded-btn, 0.5rem);
}

.atv-install-page .join > .join-item.btn:last-child {
  border-start-end-radius: var(--rounded-btn, 0.5rem);
  border-end-end-radius: var(--rounded-btn, 0.5rem);
}

/* Screenshot action sits on the device icon's bottom-right corner,
   like a profile-avatar edit badge. */
.atv-install-device-icon {
  position: relative;
}

.atv-install-device-icon .atv-install-screenshot-btn {
  position: absolute;
  right: -6px;
  bottom: -6px;
  width: 36px;
  min-width: 36px;
  height: 36px;
  min-height: 36px;
  padding: 0;
  border-radius: 9999px;
  box-shadow: var(--atv-menu-shadow);
}
</style>
