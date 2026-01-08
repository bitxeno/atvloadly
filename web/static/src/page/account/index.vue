<template>
  <div class="max-w-screen-lg mx-auto">
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
              <a class="link link-primary" @click="openCertModal(email)">{{ $t("nav.certificate") }}</a>
              <a class="link link-primary hidden" @click="openDeviceModal(email)">{{ $t("nav.connected_devices") }}</a>
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
                            {{
                              $t("home.dialog.delete_confirm.button.confirm")
                            }}
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
            <td colspan="3" class="text-center">{{ $t("account.table.empty") }}</td>
          </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- Certificate Modal -->
    <dialog id="cert_modal" class="modal" :class="{ 'modal-open': showCertModal }">
      <div class="modal-box w-11/12 max-w-5xl">
        <h3 class="font-bold text-lg mb-4">{{ $t("certificate.modal.title", { email: currentAccountEmail }) }}</h3>
        
        <table class="table w-full">
            <thead>
            <tr>
                <th>{{ $t("certificate.table.header.name") }}</th>
                <th>{{ $t("certificate.table.header.machine_name") }}</th>
                <th class="hidden md:table-cell">{{ $t("account.table.header.status") }}</th>
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
                <td>
                  <div class="font-bold">{{ cert.name }}</div>
                  <div class="text-sm opacity-50">({{ cert.serialNumber }})</div>
                </td>
                <td>{{ cert.machineName }}</td>
                <td class="hidden md:table-cell">{{ cert.status }}</td>
                <td>
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
                </td>
            </tr>
            <tr v-if="certificates.length === 0">
              <td colspan="4" class="text-center">{{ $t("certificate.no_certificates") }}</td>
            </tr>
            </template>
            </tbody>
        </table>

        <div class="modal-action">
          <button class="btn" @click="showCertModal = false">{{ $t("common.button.close") }}</button>
        </div>
      </div>
    </dialog>

    <!-- Device Modal -->
    <dialog id="device_modal" class="modal" :class="{ 'modal-open': showDeviceModal }">
      <div class="modal-box w-11/12 max-w-5xl">
        <h3 class="font-bold text-lg mb-4">{{ $t("device.modal.title", { email: currentAccountEmail }) }}</h3>
        
        <table class="table w-full">
            <thead>
            <tr>
              <th>{{ $t("device.table.header.name") }}</th>
              <th class="hidden md:table-cell">{{ $t("device.table.header.udid") }}</th>
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
                <td class="hidden md:table-cell break-all">{{ dev.deviceNumber }}</td>
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
              <td colspan="5" class="text-center">{{ $t("device.table.empty") }}</td>
            </tr>
            </template>
            </tbody>
        </table>

        <div class="modal-action">
          <button class="btn" @click="showDeviceModal = false">{{ $t("common.button.close") }}</button>
        </div>
      </div>
    </dialog>
  </div>
</template>

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
    };
  },
  created() {
    this.fetchData();
  },
  methods: {
    fetchData() {
      this.loading = true;
      api.getAccounts().then((res) => {
        this.accounts = res.data || {};
      }).finally(() => {
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
      api.getCertificates({ email: email }).then((res) => {
        this.certificates = res.data || [];
      }).finally(() => {
        this.certLoading = false;
      });
    },
    revokeCertificate(serialNumber, close) {
      let _this = this;
      api.revokeCertificate({ email: this.currentAccountEmail, serialNumber: serialNumber }).then((res) => {
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
      api.getAccountDevices({ email: email }).then((res) => {
        this.devices = res.data || [];
      }).finally(() => {
        this.deviceLoading = false;
      });
    },
    deleteDevice(deviceId, close) {
      let _this = this;
      api.deleteAccountDevice({ email: this.currentAccountEmail, deviceId: deviceId }).then((res) => {
        if (res.data) {
          toast.success(_this.$t("account.toast.delete_success"));
          _this.fetchDevices(_this.currentAccountEmail);
        } else {
          toast.error(_this.$t("account.toast.delete_failed"));
        }
      });
      close?.();
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
</style>