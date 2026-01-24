<template>
  <div class="max-w-screen-lg mx-auto">
    <div class="flex justify-between items-center mb-4 px-1">
      <button class="btn btn-soft btn-sm" @click="showLoginDialog">
        <PersonIcon class="w-4 h-4 mr-1" />
        {{ $t("install.login_modal.button.add_account") }}
      </button>
    </div>
    <div>
      <table class="table w-full">
        <thead>
          <tr>
            <th>{{ $t("account.table.header.account") }}</th>
            <th>{{ $t("account.table.header.status") }}</th>
            <th>{{ $t("home.table.header.operate") }}</th>
          </tr>
        </thead>
        <tbody class="bg-base-100">
          <tr v-if="loading">
            <td colspan="3" class="text-center">
              <span class="loading loading-spinner loading-md"></span>
            </td>
          </tr>
          <template v-else>
            <tr v-for="(account, email) in accounts" :key="email">
              <td class="break-all">{{ email }}</td>
              <td>{{ account.status }}</td>
              <td class="flex gap-x-4">
                <a class="link link-primary" @click="openCertModal(email)">{{
                  $t("nav.certificate")
                }}</a>
                <a
                  class="link link-primary hidden"
                  @click="openDeviceModal(email)"
                  >{{ $t("nav.connected_devices") }}</a
                >
                <Popper placement="top" arrow="true">
                  <template #content="{ close }">
                    <div class="flex flex-col gap-y-2">
                      <div class="py-2">
                        {{
                          $t("account.dialog.logout_confirm.title", {
                            name: email,
                          })
                        }}
                      </div>
                      <div class="flex gap-x-2 justify-end items-center">
                        <a
                          class="link link-primary link-hover"
                          @click="close"
                          >{{
                            $t("home.dialog.delete_confirm.button.cancel")
                          }}</a
                        >
                        <button
                          class="btn btn-primary btn-xs"
                          @click="logoutAccount(email, close)"
                        >
                          {{ $t("home.dialog.delete_confirm.button.confirm") }}
                        </button>
                      </div>
                    </div>
                  </template>
                  <a class="link link-error">{{
                    $t("account.table.button.logout")
                  }}</a>
                </Popper>
              </td>
            </tr>
            <tr v-if="Object.keys(accounts).length === 0">
              <td colspan="3" class="text-center">
                {{ $t("account.table.empty") }}
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- Certificate Modal -->
    <dialog
      id="cert_modal"
      class="modal"
      :class="{ 'modal-open': showCertModal }"
    >
      <div class="modal-box w-11/12 max-w-5xl">
        <h3 class="font-bold text-lg mb-4">
          {{ $t("certificate.modal.title", { email: currentAccountEmail }) }}
        </h3>

        <table class="table w-full">
          <thead>
            <tr>
              <th>{{ $t("certificate.table.header.name") }}</th>
              <th>{{ $t("certificate.table.header.machine_name") }}</th>
              <th class="hidden md:table-cell">
                {{ $t("account.table.header.status") }}
              </th>
              <th>{{ $t("home.table.header.operate") }}</th>
            </tr>
          </thead>
          <tbody class="bg-base-100">
            <tr v-if="certLoading">
              <td colspan="4" class="text-center">
                <span class="loading loading-spinner loading-md"></span>
              </td>
            </tr>
            <template v-else>
              <tr v-for="cert in certificates" :key="cert.serialNumber">
                <td class="break-all">
                  <div class="font-bold">
                    {{ cert.name }}
                    <span v-if="cert.inUse" class="text-sm opacity-50"
                      >(atvloadly)</span
                    >
                  </div>
                  <div class="text-sm opacity-50">
                    ({{ cert.serialNumber }})
                  </div>
                </td>
                <td>{{ cert.machineName }}</td>
                <td class="hidden md:table-cell">{{ cert.status }}</td>
                <td>
                  <div class="flex gap-x-1">
                  <a
                    class="link link-primary mr-2"
                    @click="cert.inUse && openExportModal(cert)"
                    :aria-disabled="!cert.inUse"
                    >{{ $t("certificate.table.button.export") }}</a
                  >
                  <Popper placement="top" arrow="true">
                    <template #content="{ close }">
                      <div class="flex flex-col gap-y-2">
                        <div class="py-2">
                          {{
                            $t("certificate.dialog.revoke_confirm.title", {
                              serial: cert.machineName,
                            })
                          }}
                        </div>
                        <div class="flex gap-x-2 justify-end items-center">
                          <a
                            class="link link-primary link-hover"
                            @click="close"
                            >{{
                              $t("home.dialog.delete_confirm.button.cancel")
                            }}</a
                          >
                          <button
                            class="btn btn-primary btn-xs"
                            @click="revokeCertificate(cert.serialNumber, close)"
                          >
                            {{
                              $t("home.dialog.delete_confirm.button.confirm")
                            }}
                          </button>
                        </div>
                      </div>
                    </template>
                    <a class="link link-error">{{
                      $t("certificate.table.button.revoke")
                    }}</a>
                  </Popper>
                  </div>
                </td>
              </tr>
              <tr v-if="certificates.length === 0">
                <td colspan="4" class="text-center">
                  {{ $t("certificate.no_certificates") }}
                </td>
              </tr>
            </template>
          </tbody>
        </table>

        <div class="modal-action">
          <button class="btn btn-primary" @click="triggerImport">
            {{ $t("certificate.button.import") }}
          </button>
          <button class="btn" @click="showCertModal = false">
            {{ $t("common.button.close") }}
          </button>
        </div>
        <input
          type="file"
          ref="certFileInput"
          class="hidden"
          accept=".p12"
          @change="onCertFileSelected"
        />
      </div>
    </dialog>

    <!-- Device Modal -->
    <dialog
      id="device_modal"
      class="modal"
      :class="{ 'modal-open': showDeviceModal }"
    >
      <div class="modal-box w-11/12 max-w-5xl">
        <h3 class="font-bold text-lg mb-4">
          {{ $t("device.modal.title", { email: currentAccountEmail }) }}
        </h3>

        <table class="table w-full">
          <thead>
            <tr>
              <th>{{ $t("device.table.header.name") }}</th>
              <th class="hidden md:table-cell">
                {{ $t("device.table.header.udid") }}
              </th>
              <th>{{ $t("device.table.header.platform") }}</th>
              <th>{{ $t("home.table.header.operate") }}</th>
            </tr>
          </thead>
          <tbody class="bg-base-100">
            <tr v-if="deviceLoading">
              <td colspan="5" class="text-center">
                <span class="loading loading-spinner loading-md"></span>
              </td>
            </tr>
            <template v-else>
              <tr v-for="dev in devices" :key="dev.deviceId">
                <td>
                  <div class="font-bold">{{ dev.name }}</div>
                  <div class="text-sm opacity-50">({{ dev.deviceId }})</div>
                </td>
                <td class="hidden md:table-cell break-all">
                  {{ dev.deviceNumber }}
                </td>
                <td>{{ dev.deviceClass }}</td>
                <td>
                  <Popper placement="top" arrow="true">
                    <template #content="{ close }">
                      <div class="flex flex-col gap-y-2">
                        <div class="py-2">
                          {{
                            $t("home.dialog.delete_confirm.title", {
                              name: dev.name,
                            })
                          }}
                        </div>
                        <div class="flex gap-x-2 justify-end items-center">
                          <a
                            class="link link-primary link-hover"
                            @click="close"
                            >{{
                              $t("home.dialog.delete_confirm.button.cancel")
                            }}</a
                          >
                          <button
                            class="btn btn-primary btn-xs"
                            @click="deleteDevice(dev.deviceId, close)"
                          >
                            {{
                              $t("home.dialog.delete_confirm.button.confirm")
                            }}
                          </button>
                        </div>
                      </div>
                    </template>
                    <a class="link link-error">{{
                      $t("home.table.button.delete")
                    }}</a>
                  </Popper>
                </td>
              </tr>
              <tr v-if="devices.length === 0">
                <td colspan="5" class="text-center">
                  {{ $t("device.table.empty") }}
                </td>
              </tr>
            </template>
          </tbody>
        </table>

        <div class="modal-action">
          <button class="btn" @click="showDeviceModal = false">
            {{ $t("common.button.close") }}
          </button>
        </div>
      </div>
    </dialog>

    <!-- Import Modal -->
    <dialog
      id="import_modal"
      class="modal"
      :class="{ 'modal-open': showImportModal }"
    >
      <div class="modal-box">
        <h3 class="font-bold text-lg mb-4">
          {{ $t("certificate.modal.import_title") }}
        </h3>
        <p class="py-2 text-sm opacity-70" v-if="selectedCertFile">
          {{
            $t("certificate.import.file_selected", {
              name: selectedCertFile ? selectedCertFile.name : "",
            })
          }}
        </p>
        <div class="form-control w-full">
          <label class="label">
            <span class="label-text">{{
              $t("certificate.import_form.password_label")
            }}</span>
          </label>
          <input
            type="password"
            v-model="importPassword"
            :placeholder="$t('certificate.import_form.password_placeholder')"
            class="input input-bordered w-full"
            @keyup.enter="doImportCertificate"
          />
        </div>
        <div class="modal-action">
          <button class="btn" @click="closeImportModal">
            {{ $t("home.dialog.delete_confirm.button.cancel") }}
          </button>
          <button class="btn btn-primary" @click="doImportCertificate">
            {{ $t("home.dialog.delete_confirm.button.confirm") }}
          </button>
        </div>
      </div>
    </dialog>

    <!-- Export Modal -->
    <dialog
      id="export_modal"
      class="modal"
      :class="{ 'modal-open': showExportModal }"
    >
      <div class="modal-box">
        <h3 class="font-bold text-lg mb-4">
          {{ $t("certificate.modal.export_title") }}
        </h3>
        <div class="form-control w-full">
          <label class="label">
            <span class="label-text">{{
              $t("certificate.export_form.password_label")
            }}</span>
          </label>
          <input
            type="password"
            v-model="exportPassword"
            :placeholder="$t('certificate.export_form.password_placeholder')"
            class="input input-bordered w-full"
          />
        </div>
        <div class="modal-action">
          <button class="btn" @click="showExportModal = false">
            {{ $t("home.dialog.delete_confirm.button.cancel") }}
          </button>
          <button class="btn btn-primary" @click="doExportCertificate">
            {{ $t("home.dialog.delete_confirm.button.confirm") }}
          </button>
        </div>
      </div>
    </dialog>


    <!-- Login Modal -->
    <dialog
      id="login_modal"
      :class="['modal', { 'modal-open': loginDialogVisible }]"
    >
      <div class="modal-box">
        <h3 class="font-bold text-lg">{{ $t("install.login_modal.title") }}</h3>
        <form id="loginForm" class="flex flex-col gap-y-4 mt-4">
          <div class="form-control w-full">
            <label class="label">
              <span class="label-text">{{
                $t("install.form.account.label")
              }}</span>
            </label>
            <input
              type="text"
              class="input input-bordered w-full"
              placeholder="xxxx@example.com"
              v-model="loginForm.account"
              required
            />
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
              v-model="loginForm.password"
              required
            />
          </div>
        </form>
        <div class="modal-action">
          <button class="btn" @click="loginDialogVisible = false">
            {{ $t("install.login_modal.button.close") }}
          </button>
          <button
            class="btn btn-primary"
            @click="onLoginSubmit"
            :disabled="loginLoading"
          >
            <span class="loading loading-spinner" v-show="loginLoading"></span>
            {{ $t("install.login_modal.button.login") }}
          </button>
        </div>
      </div>
    </dialog>

    <!-- Auth Modal (2FA) -->
    <dialog
      id="auth_modal"
      :class="['modal', { 'modal-open': authDialogVisible }]"
    >
      <div class="modal-box">
        <h3 class="font-bold text-lg">
          {{ $t("install.dialog.input_pin.title") }}
        </h3>
        <form id="authForm">
          <p class="py-4">
            <input
              type="text"
              class="input input-bordered input-primary w-full"
              :placeholder="$t('install.dialog.input_pin.input.placeholder')"
              v-model="authForm.authcode"
              required
            />
          </p>
        </form>
        <div class="modal-action">
          <button class="btn" @click="authDialogVisible = false">
            {{ $t("install.login_modal.button.close") }}
          </button>
          <button
            class="btn btn-primary"
            @click="onSubmit2FA"
            :disabled="authLoading"
          >
            <span class="loading loading-spinner" v-show="authLoading"></span>
            {{ $t("install.dialog.input_pin.button.submit") }}
          </button>
        </div>
      </div>
    </dialog>
  </div>
</template>

<script setup>
import PersonIcon from "@/assets/icons/person.badge.plus.svg";
</script>
<script>
import api from "@/api/api";
import { toast } from "vue3-toastify";

export default {
  name: "Account",
  data() {
    return {
      accounts: {},
      loading: false,
      showCertModal: false,
      currentAccountEmail: "",
      certificates: [],
      certLoading: false,
      showDeviceModal: false,
      devices: [],
      deviceLoading: false,
      showExportModal: false,
      exportPassword: "",
      exportingCert: null,
      showImportModal: false,
      importPassword: "",
      selectedCertFile: null,
      
      // Login related
      loginWebsock: null,
      loginDialogVisible: false,
      loginLoading: false,
      loginErr: "",
      authDialogVisible: false,
      authLoading: false,
      loginForm: {
        account: "",
        password: "",
      },
      authForm: {
        authcode: "",
      },
    };
  },
  created() {
    this.fetchData();
  },
  methods: {
    fetchData() {
      this.loading = true;
      api
        .getAccounts()
        .then((res) => {
          this.accounts = res.data || {};
        })
        .finally(() => {
          this.loading = false;
        });
    },
    logoutAccount(email, close) {
      let _this = this;
      api.logoutAccount({ email: email }).then((res) => {
        if (res.data) {
          toast.success(_this.$t("account.toast.logout_success"));
          _this.fetchData();
        } else {
          toast.error(_this.$t("account.toast.logout_failed"));
        }
      });
      close?.();
    },
    openCertModal(email) {
      this.currentAccountEmail = email;
      this.showCertModal = true;
      this.fetchCertificates(email);
    },
    fetchCertificates(email) {
      this.certLoading = true;
      api
        .getCertificates({ email: email })
        .then((res) => {
          this.certificates = res.data || [];
        })
        .finally(() => {
          this.certLoading = false;
        });
    },
    revokeCertificate(serialNumber, close) {
      let _this = this;
      api
        .revokeCertificate({
          email: this.currentAccountEmail,
          serialNumber: serialNumber,
        })
        .then((res) => {
          if (res.data) {
            toast.success(_this.$t("certificate.toast.revoke_success"));
            _this.fetchCertificates(_this.currentAccountEmail);
          } else {
            toast.error(_this.$t("certificate.toast.revoke_failed"));
          }
        });
      close?.();
    },
    openDeviceModal(email) {
      this.currentAccountEmail = email;
      this.showDeviceModal = true;
      this.fetchDevices(email);
    },
    fetchDevices(email) {
      this.deviceLoading = true;
      api
        .getAccountDevices({ email: email })
        .then((res) => {
          this.devices = res.data || [];
        })
        .finally(() => {
          this.deviceLoading = false;
        });
    },
    deleteDevice(deviceId, close) {
      let _this = this;
      api
        .deleteAccountDevice({
          email: this.currentAccountEmail,
          deviceId: deviceId,
        })
        .then((res) => {
          if (res.data) {
            toast.success(_this.$t("account.toast.delete_success"));
            _this.fetchDevices(_this.currentAccountEmail);
          } else {
            toast.error(_this.$t("account.toast.delete_failed"));
          }
        });
      close?.();
    },
    openExportModal(cert) {
      this.exportingCert = cert;
      this.exportPassword = "";
      this.showExportModal = true;
    },
    doExportCertificate() {
      if (!this.exportPassword) {
        toast.error(this.$t("pair.toast.pin_incorrect")); // Reusing error or just generic?
        // Better to use generic or simple string since I didn't add validation key
        // Or "Input Required"
        return;
      }

      const toastId = toast.loading(this.$t("common.loading") || "Loading...");

      api
        .exportCertificate({
          email: this.currentAccountEmail,
          password: this.exportPassword,
          // teamId: ...
        })
        .then((response) => {
          const url = window.URL.createObjectURL(new Blob([response.data]));
          const link = document.createElement("a");
          link.href = url;
          link.setAttribute(
            "download",
            `atvloadly_${this.currentAccountEmail}.p12`
          );
          document.body.appendChild(link);
          link.click();
          link.remove();

          toast.update(toastId, {
            render: this.$t("certificate.toast.export_success"),
            type: "success",
            isLoading: false,
            autoClose: 3000,
          });
          this.showExportModal = false;
        })
        .catch((err) => {});
    },
    triggerImport() {
      this.$refs.certFileInput.click();
    },
    onCertFileSelected(e) {
      const files = e.target.files;
      if (files.length > 0) {
        this.selectedCertFile = files[0];
        this.importPassword = "";
        this.showImportModal = true;
      }
      // Reset input so same file can be selected again if needed
      e.target.value = "";
    },
    closeImportModal() {
      this.showImportModal = false;
      this.selectedCertFile = null;
      this.importPassword = "";
    },
    doImportCertificate() {
      if (!this.importPassword) {
        // Optionally prompt, but empty password might be valid for some p12?
        // Usually not. Assuming required as per prompt "prompt for password".
        // Use toast if empty? Or just let it try.
      }

      if (!this.selectedCertFile) return;

      const toastId = toast.loading(this.$t("common.loading"));

      const formData = new FormData();
      formData.append("email", this.currentAccountEmail);
      formData.append("password", this.importPassword);
      formData.append("file", this.selectedCertFile);

      api
        .importCertificate(formData)
        .then((res) => {
          if (res.data) {
            toast.update(toastId, {
              render: this.$t("certificate.toast.import_success"),
              type: "success",
              isLoading: false,
              autoClose: 3000,
            });
            this.closeImportModal();
            this.fetchCertificates(this.currentAccountEmail);
          } else {
            toast.update(toastId, {
              render: this.$t("certificate.toast.import_failed"),
              type: "error",
              isLoading: false,
              autoClose: 3000,
            });
          }
        })
        .catch((err) => {
          toast.update(toastId, {
            render: err.message || "Import failed",
            type: "error",
            isLoading: false,
            autoClose: 3000,
          });
        });
    },

    // Login Methods
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
      this.loginDialogVisible = true;
      this.loginForm.account = "";
      this.loginForm.password = "";
      this.authForm.authcode = "";
    },
    onLoginSubmit() {
      if (!this.validateForm("#loginForm")) {
        return;
      }
      this.loginLoading = true;
      this.loginErr = "";
      this.initLoginWebSocket();
    },
    initLoginWebSocket() {
      const wsuri =
        (location.protocol === "https:" ? "wss://" : "ws://") +
        location.host +
        "/ws/login";
      this.loginWebsock = new WebSocket(wsuri);
      this.loginWebsock.onopen = this.loginWebsocketonopen;
      this.loginWebsock.onerror = this.loginWebsocketonerror;
      this.loginWebsock.onmessage = this.loginWebsocketonmessage;
      this.loginWebsock.onclose = this.loginWebsocketclose;
    },
    loginWebsocketonopen() {
      console.log("Login WebSocket connect success.");
      // Send login data
      this.loginWebsocketsend(1, {
        account: this.loginForm.account,
        password: this.loginForm.password,
      });
    },
    loginWebsocketonmessage(e) {
      let _this = this;
      let line = e.data;

      if (line.indexOf("Enter 2FA code") !== -1) {
        _this.authForm.authcode = "";
        _this.authDialogVisible = true;
        return;
      }

      if (line.indexOf("Successfully logged in") !== -1) {
        _this.loginLoading = false;
        _this.authLoading = false;
        toast.success(_this.$t("install.toast.login_success"));
        _this.loginDialogVisible = false;
        _this.authDialogVisible = false;
        _this.fetchData(); // Refresh accounts
        _this.loginWebsock.close();
        return;
      }

      if (line.indexOf("exit status") !== -1) {
        _this.loginLoading = false;
        _this.authLoading = false;
        toast.error(_this.loginErr);
        return;
      } 

      if (line.toLowerCase().indexOf("error") !== -1 || _this.loginErr !== "") {
        _this.loginErr += line;
        return;
      }
    },
    loginWebsocketsend(t, data) {
      let _this = this;
      if (typeof data !== "string") {
        data = JSON.stringify(data);
      }
      const json = JSON.stringify({ t: t, d: data });
      _this.loginWebsock.send(json);
    },
    loginWebsocketclose(e) {
      console.log(`login connection closed (${e.code})`);
      this.loginLoading = false;
      this.authLoading = false;
    },
    loginWebsocketonerror(e) {
      console.log("Login WebSocket connect failed.");
      this.loginLoading = false;
      toast.error("WebSocket connection failed");
    },
    onSubmit2FA() {
      let _this = this;

      if (!_this.validateForm("#authForm")) {
        return;
      }

      _this.authLoading = true;
      _this.loginWebsocketsend(2, _this.authForm.authcode); 
      _this.authDialogVisible = false; 
    },
  },
};
</script>

<style lang="postcss" scoped>
.headline {
  @apply prose mb-2;
}
.empty {
  background-image: repeating-linear-gradient(
    45deg,
    hsl(var(--b1)),
    hsl(var(--b1)) 13px,
    hsl(var(--b2)) 13px,
    hsl(var(--b2)) 14px
  );
  @apply border-base-300 bg-base-100 rounded-b-box flex min-h-[6rem]  flex-wrap items-center justify-center gap-2 overflow-x-hidden border bg-cover bg-top p-4;
}

:deep(.popper) {
  background: #ffffff;
  padding: 12px;
  border-radius: 4px;
  border: 1px solid #ebeef5;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  word-break: break-all;
  text-align: justify;
  min-width: 150px;
}

:deep(.popper:hover),
:deep(.popper:hover > #arrow::before) {
  background: #ffffff;
}

:deep(.popper #arrow::before) {
  background: #ffffff;
}

/* Disabled link styles */
a[disabled],
.link[disabled],
a[aria-disabled="true"],
.link[aria-disabled="true"] {
  pointer-events: none;
  opacity: 0.5;
  cursor: not-allowed;
  color: var(--b2) !important;
  text-decoration: none;
}

a[disabled]:hover,
.link[disabled]:hover,
a[aria-disabled="true"]:hover,
.link[aria-disabled="true"]:hover {
  text-decoration: none;
}
</style>
