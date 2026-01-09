<template>
  <div class="max-w-screen-md mx-auto flex flex-col gap-y-6">
    <div class="alert alert-warning">
      <div class="w-8">
        <WarningIcon />
      </div>
      <div class="text-left">
        <ul class="list-disc pl-6 space-y-1 text-left">
          <li class="text-sm">{{ $t("laboratory.tips.step1") }}</li>
          <li class="text-sm" v-html="$t('laboratory.tips.step2')"></li>
          <li class="text-sm">{{ $t("laboratory.tips.step3") }}</li>
          <li class="text-sm hidden" v-html="$t('laboratory.tips.step4')"></li>
          <li class="text-sm">{{ $t("laboratory.tips.step5") }}</li>
        </ul>
      </div>
    </div>

    <div class="border rounded p-6 bg-base-100">
      <div class="p-6 flex flex-col gap-y-4">
        <h2 class="text-xl font-bold">{{ $t("laboratory.title") }}</h2>
        <form id="form" class="flex flex-col gap-y-4">
          <div class="form-control w-full">
            <label class="label">
              <span class="label-text">{{ $t("laboratory.form.upload_label") }}</span>
            </label>
            <input
              type="file"
              class="file-input file-input-bordered w-full"
              @change="onFileChange"
              required
            />
          </div>
          <div class="form-control w-full">
            <label class="label">
              <span class="label-text">{{ $t("laboratory.form.device_label") }}</span>
            </label>
            <div class="join flex w-full">
              <select
                class="select select-bordered join-item flex-1 w-full"
                v-model="selectedDeviceIp"
                required
              >
                <option value="" disabled>{{ $t("laboratory.form.device_placeholder") }}</option>
                <option v-for="device in devices" :key="device.ip" :value="device.ip">
                  {{ device.name }} ({{ device.ip }})
                </option>
              </select>
              <button 
                class="btn join-item w-16" 
                @click.prevent="loadDevices"
                :disabled="loading"
                :title="$t('laboratory.form.refresh_devices')"
              >
                <div class="w-6 h-6" :class="{ 'animate-spin': loadingDevices }">
                  <RefreshIcon />
                </div>
              </button>
            </div>
          </div>

        </form>

        <div class="flex flex-row gap-x-4">
          <button
            class="btn btn-primary flex-1"
            @click="onSubmit(false)"
            :disabled="loading"
          >
            <span class="loading loading-spinner" v-show="loading"></span
            >{{ $t("laboratory.form.submit") }}
          </button>
        </div>
      </div>
    </div>

    <dialog id="confirm_modal" class="modal" :class="{ 'modal-open': confirmDialogVisible }">
      <div class="modal-box">
        <h3 class="font-bold text-lg">{{ $t("laboratory.dialog.override.title") }}</h3>
        <p class="py-4">{{ $t("laboratory.dialog.override.content") }}</p>
        <div class="modal-action">
          <button class="btn" @click="confirmDialogVisible = false">{{ $t("laboratory.dialog.override.cancel") }}</button>
          <button class="btn btn-primary" @click="onConfirmOverride">{{ $t("laboratory.dialog.override.confirm") }}</button>
        </div>
      </div>
    </dialog>
  </div>
</template>

<script>
import api from "@/api/api";
import { toast } from "vue3-toastify";

export default {
  data() {
    return {
      files: [],
      loading: false,
      loadingDevices: false,
      confirmDialogVisible: false,
      devices: [],
      selectedDeviceIp: "",
    };
  },
  mounted() {
    this.loadDevices();
  },
  methods: {
    async loadDevices() {
      this.loadingDevices = true;
      try {
        const res = await api.scanWireless();
        this.devices = res.data || [];
      } catch (error) {
        console.error("Failed to load devices:", error);
        toast.error(this.$t("laboratory.toast.load_devices_failed", { msg: error.message || "未知错误" }));
      } finally {
        this.loadingDevices = false;
      }
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
    async onSubmit(override = false) {
      if (!this.validateForm("#form")) {
        return;
      }
      
      this.loading = true;
      try {
        let formData = new FormData();
        if (this.files.length > 0) {
            formData.append("file", this.files[0]);
        }
        if (override) {
            formData.append("override", "true");
        }
        if (this.selectedDeviceIp) {
            formData.append("ip", this.selectedDeviceIp);
        }
        
        await api.importPair(formData);
        toast.success(this.$t("laboratory.toast.import_success"));
        document.getElementById("form").reset();
        this.files = [];
        this.confirmDialogVisible = false;
      } catch (error) {
        console.error(error);
        if (!override && error.message && error.message.includes("pairing file already exists")) {
            this.confirmDialogVisible = true;
        }
      } finally {
        this.loading = false;
      }
    },
    onConfirmOverride() {
        this.onSubmit(true);
    }
  },
};
</script>


<script setup>
import WarningIcon from "@/assets/icons/warning.svg";
import RefreshIcon from "@/assets/icons/refresh.svg";
</script>