<template>
  <dialog :class="['modal', { 'modal-open': visible }]">
    <div class="modal-box atv-cert-revoke-dialog">
      <h3 class="font-bold text-lg">{{ $t("install.cert_revoke.title") }}</h3>
      <p class="py-2 text-sm opacity-80">{{ $t("install.cert_revoke.warning") }}</p>

      <div v-if="loading" class="flex items-center gap-x-2 py-4">
        <span class="loading loading-spinner loading-sm"></span>
        <span>{{ $t("common.loading") }}</span>
      </div>
      <div v-else-if="certificates.length === 0" class="py-4 text-sm">
        {{ $t("install.cert_revoke.no_certificates") }}
      </div>
      <table v-else class="table w-full">
        <thead>
          <tr>
            <th></th>
            <th>{{ $t("certificate.table.header.name") }}</th>
            <th>{{ $t("certificate.table.header.machine_name") }}</th>
            <th>{{ $t("certificate.table.header.expiration") }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="cert in certificates" :key="cert.serialNumber">
            <td>
              <input
                type="radio"
                name="revoke_cert"
                class="radio radio-sm"
                :value="cert.serialNumber"
                v-model="selectedSerial"
              />
            </td>
            <td class="break-all">{{ cert.name }}</td>
            <td class="break-all">{{ cert.machineName }}</td>
            <td class="whitespace-nowrap">{{ cert.expirationDate }}</td>
          </tr>
        </tbody>
      </table>

      <div class="modal-action">
        <button class="btn" @click="close">
          {{ $t("home.dialog.delete_confirm.button.cancel") }}
        </button>
        <button
          class="btn btn-error"
          :disabled="loading || submitting || !selectedSerial"
          @click="confirm"
        >
          <span v-if="submitting" class="loading loading-spinner loading-xs"></span>
          {{ $t("install.cert_revoke.button.revoke") }}
        </button>
      </div>
    </div>
  </dialog>
</template>

<script>
import api from "@/api/api";
import { pickRevocableCertificates } from "@/utils/install-error-feedback.mjs";

// CertificateRevokeDialog asks which one certificate may be revoked when an
// Apple ID account is at its certificate limit. The caller retries its own
// operation with the confirmed serial; the dialog never revokes anything.
export default {
  name: "CertificateRevokeDialog",
  emits: ["confirm", "close"],
  data() {
    return {
      visible: false,
      loading: false,
      submitting: false,
      certificates: [],
      selectedSerial: "",
      account: "",
      requestID: 0,
    };
  },
  methods: {
    show(account) {
      this.reset();
      this.visible = true;
      this.loading = true;
      this.account = account || "";

      const requestID = this.requestID;
      api
        .getCertificates({ email: this.account })
        .then((res) => {
          if (requestID !== this.requestID) return;
          this.certificates = pickRevocableCertificates(res.data);
        })
        .catch(() => {
          // request.js already reports the failure; the empty table tells the
          // user no certificate can be offered.
        })
        .finally(() => {
          if (requestID !== this.requestID) return;
          this.loading = false;
        });
    },
    confirm() {
      if (this.loading || this.submitting || !this.selectedSerial) return;

      const serial = this.selectedSerial;
      this.submitting = true;
      this.visible = false;
      this.reset();
      this.$emit("confirm", serial);
    },
    close() {
      this.reset();
      this.$emit("close");
    },
    reset() {
      this.requestID++;
      this.visible = false;
      this.loading = false;
      this.submitting = false;
      this.certificates = [];
      this.selectedSerial = "";
      this.account = "";
    },
  },
};
</script>

<style scoped>
.modal .modal-box.atv-cert-revoke-dialog {
  width: min(768px, calc(100vw - 24px));
  max-width: 768px;
}
</style>
