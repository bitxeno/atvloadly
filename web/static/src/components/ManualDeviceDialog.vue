<template>
  <dialog :class="['modal', { 'modal-open': visibleAdd }]">
    <div class="modal-box">
      <button type="button" class="btn btn-sm btn-circle btn-ghost absolute right-2 top-2" @click="closeAdd">✕</button>
      <h3 class="font-bold text-lg pr-8">{{ $t("home.dialog.manual.add_title") }}</h3>

      <div class="mt-4 flex flex-col gap-y-2">
        <div class="form-control">
          <label class="label pb-1">{{ $t("home.dialog.manual.form.name_label") }}</label>
          <input
            type="text"
            class="input input-bordered w-full"
            v-model="addForm.name"
            :placeholder="$t('home.dialog.manual.form.name_placeholder')"
          />
        </div>

        <div class="form-control">
          <label class="label pb-1">{{ $t("home.dialog.manual.form.ip_label") }}</label>
          <input
            type="text"
            class="input input-bordered w-full"
            v-model="addForm.ip"
            :placeholder="$t('home.dialog.manual.form.ip_placeholder')"
          />
        </div>

        <details class="collapse collapse-arrow bg-base-200 rounded-box mt-2">
          <summary class="collapse-title text-sm min-h-0 py-3">
            {{ $t("home.dialog.manual.instructions.title") }}
          </summary>
          <div class="collapse-content text-sm">
            <ol class="list-decimal list-inside flex flex-col gap-y-2">
              <li v-html="$t('home.dialog.manual.instructions.step1')"></li>
              <li v-html="$t('home.dialog.manual.instructions.step2')"></li>
            </ol>
          </div>
        </details>
      </div>

      <div class="modal-action">
        <button type="button" class="btn" @click="closeAdd">
          {{ $t("common.button.close") }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="addBusy || !addForm.ip"
          @click="submitAdd"
        >
          <span class="loading loading-spinner loading-xs" v-show="addBusy"></span>
          {{ $t("home.dialog.manual.button.add") }}
        </button>
      </div>
    </div>
  </dialog>

  <dialog :class="['modal', { 'modal-open': visibleImport }]">
    <div class="modal-box">
      <button type="button" class="btn btn-sm btn-circle btn-ghost absolute right-2 top-2" @click="closeImport">✕</button>
      <h3 class="font-bold text-lg pr-8">{{ $t("home.dialog.manual.import_title") }}</h3>

      <div class="mt-4 flex flex-col gap-y-2">
        <div class="form-control">
          <label class="label pb-1">{{ $t("home.dialog.manual.import.file_label") }}</label>
          <input
            type="file"
            class="file-input file-input-bordered w-full"
            accept=".plist,.mobiledevicepairing,application/x-plist,application/xml"
            @change="onFileChange"
          />
        </div>

        <div class="form-control">
          <label class="label pb-1">{{ $t("home.dialog.manual.form.ip_label") }}</label>
          <input
            type="text"
            class="input input-bordered w-full"
            v-model="importForm.ip"
            :placeholder="$t('home.dialog.manual.form.ip_placeholder')"
          />
        </div>

        <p class="text-sm text-base-content/70 whitespace-normal">
          {{ $t("home.dialog.manual.import.tips") }}
        </p>

        <details class="collapse collapse-arrow bg-base-200 rounded-box">
          <summary class="collapse-title text-sm min-h-0 py-3">
            {{ $t("home.dialog.manual.import.instructions.title") }}
          </summary>
          <div class="collapse-content text-sm">
            <ol class="list-decimal list-inside flex flex-col gap-y-2">
              <li v-html="$t('home.dialog.manual.import.instructions.step1')"></li>
              <li v-html="$t('home.dialog.manual.import.instructions.step2')"></li>
              <li v-html="$t('home.dialog.manual.import.instructions.step3')"></li>
            </ol>
          </div>
        </details>
      </div>

      <div class="modal-action">
        <button type="button" class="btn" @click="closeImport">
          {{ $t("common.button.close") }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="importBusy || !importForm.file || !importForm.ip"
          @click="submitImport"
        >
          <span class="loading loading-spinner loading-xs" v-show="importBusy"></span>
          {{ $t("home.dialog.manual.button.import") }}
        </button>
      </div>
    </div>
  </dialog>
</template>

<script>
import api from "@/api/api";
import { toast } from "vue3-toastify";

export default {
  name: "ManualDeviceDialog",
  emits: ["saved"],
  data() {
    return {
      visibleAdd: false,
      visibleImport: false,
      addBusy: false,
      importBusy: false,
      addForm: {
        name: "",
        ip: "",
      },
      importForm: {
        file: null,
        ip: "",
        deviceId: "",
      },
    };
  },
  methods: {
    showAdd() {
      this.addForm = { name: "", ip: "" };
      this.visibleAdd = true;
    },
    showImport(device) {
      this.importForm = {
        file: null,
        // The install service port is detected by scanning during the
        // import itself; only the address goes in.
        ip: device ? device.ip : "",
        deviceId: device && device.id ? device.id : "",
      };
      this.visibleImport = true;
    },
    closeAdd() {
      this.visibleAdd = false;
    },
    closeImport() {
      this.visibleImport = false;
    },
    onFileChange(event) {
      this.importForm.file = event.target.files && event.target.files.length > 0 ? event.target.files[0] : null;
    },
    async submitAdd() {
      this.addBusy = true;
      try {
        await api.addManualDevice({
          name: this.addForm.name,
          ip: this.addForm.ip,
        });
        toast.success(this.$t("home.dialog.manual.toast.added"));
        this.visibleAdd = false;
        this.$emit("saved");
      } catch (err) {
        // request.js already shows the error.
        console.error(err);
      } finally {
        this.addBusy = false;
      }
    },
    async submitImport() {
      const data = new FormData();
      data.append("file", this.importForm.file);
      data.append("ip", this.importForm.ip);
      if (this.importForm.deviceId) {
        data.append("device_id", this.importForm.deviceId);
      }

      this.importBusy = true;
      try {
        await api.importPair(data);
        toast.success(this.$t("home.dialog.manual.toast.imported"));
        this.visibleImport = false;
        this.$emit("saved");
      } catch (err) {
        // request.js already shows the error.
        console.error(err);
      } finally {
        this.importBusy = false;
      }
    },
  },
};
</script>
