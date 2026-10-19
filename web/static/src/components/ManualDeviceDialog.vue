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

        <div class="form-control">
          <label class="label pb-1">{{ $t("home.dialog.manual.form.port_label") }}</label>
          <input
            type="number"
            min="1"
            max="65535"
            class="input input-bordered w-full"
            v-model="addForm.port"
          />
          <label class="label pt-1 pb-0">
            <span class="label-text-alt">{{ $t("home.dialog.manual.form.port_hint") }}</span>
          </label>
        </div>

        <div class="form-control">
          <label class="label pb-1">{{ $t("home.dialog.manual.form.install_port_label") }}</label>
          <input
            type="number"
            min="0"
            max="65535"
            class="input input-bordered w-full"
            v-model="addForm.installPort"
          />
          <label class="label pt-1 pb-0">
            <span class="label-text-alt">{{ $t("home.dialog.manual.form.install_port_hint") }}</span>
          </label>
        </div>

        <details class="collapse collapse-arrow bg-base-200 rounded-box mt-2">
          <summary class="collapse-title text-sm min-h-0 py-3">
            {{ $t("home.dialog.manual.instructions.title") }}
          </summary>
          <div class="collapse-content text-sm">
            <ol class="list-decimal list-inside flex flex-col gap-y-2">
              <li v-html="$t('home.dialog.manual.instructions.step1')"></li>
              <li v-html="$t('home.dialog.manual.instructions.step2')"></li>
              <li v-html="$t('home.dialog.manual.instructions.step3')"></li>
              <li v-html="$t('home.dialog.manual.instructions.step4')"></li>
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
          :disabled="addBusy || !addForm.ip || !addForm.port"
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

        <div class="form-control">
          <label class="label pb-1">{{ $t("home.dialog.manual.form.install_port_label") }}</label>
          <input
            type="number"
            min="1"
            max="65535"
            class="input input-bordered w-full"
            v-model="importForm.port"
          />
          <button
            type="button"
            class="btn btn-xs btn-outline mt-1"
            :disabled="scanBusy || !importForm.ip"
            @click="scanImportPort"
            :title="$t('home.dialog.manual.form.install_port_scan')"
            :aria-label="$t('home.dialog.manual.form.install_port_scan')"
          >
            <span class="loading loading-spinner loading-xs" v-show="scanBusy"></span>
            {{ $t("home.dialog.manual.form.install_port_scan") }}
          </button>
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
          :disabled="importBusy || !importForm.file || !importForm.ip || !importForm.port"
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
      scanBusy: false,
      addForm: {
        name: "",
        ip: "",
        port: 62078,
        installPort: 49152,
      },
      importForm: {
        file: null,
        ip: "",
        port: 49152,
        deviceId: "",
      },
    };
  },
  methods: {
    showAdd() {
      this.addForm = { name: "", ip: "", port: 62078, installPort: 49152 };
      this.visibleAdd = true;
    },
    showImport(device) {
      this.importForm = {
        file: null,
        ip: device ? device.ip : "",
        // Import validates the record over the tunnel-capable remote-pairing
        // service, so the install service port is what goes here.
        port: device && device.install_port ? device.install_port : device && device.port ? device.port : 49152,
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
    async scanImportPort() {
      if (!this.importForm.ip || this.scanBusy) {
        return;
      }
      this.scanBusy = true;
      try {
        const res = await api.scanInstallPort(this.importForm.ip);
        const port = res.data && res.data.install_port ? res.data.install_port : null;
        if (port) {
          this.importForm.port = port;
          toast.success(this.$t("home.dialog.manual.form.install_port_scan_success", { port }));
        } else {
          toast.error(this.$t("home.dialog.manual.form.install_port_scan_failed"));
        }
      } catch (err) {
        // request.js already shows the error.
        console.error(err);
        toast.error(this.$t("home.dialog.manual.form.install_port_scan_failed"));
      } finally {
        this.scanBusy = false;
      }
    },
    async submitAdd() {
      this.addBusy = true;
      try {
        await api.addManualDevice({
          name: this.addForm.name,
          ip: this.addForm.ip,
          port: Number(this.addForm.port),
          install_port: Number(this.addForm.installPort) || 0,
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
      data.append("port", String(Number(this.importForm.port)));
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
