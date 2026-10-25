<template>
  <div class="max-w-screen-md mx-auto flex flex-col gap-y-6">
    <!-- Tab-specific guidance -->
    <div class="alert atv-warning" v-if="activeTab === 'appletv'">
      <div class="w-8 shrink-0">
        <WarningIcon />
      </div>
      <div class="text-left">
        <ul class="list-disc pl-6 space-y-1 text-left">
          <li class="text-sm">{{ $t("addDevice.appletv.pairing_step1") }}</li>
          <li class="text-sm">{{ $t("addDevice.appletv.pairing_step2") }}</li>
        </ul>
      </div>
    </div>
    <div class="alert atv-warning" v-else>
      <div class="w-8 shrink-0">
        <WarningIcon />
      </div>
      <div class="text-left">
        <ul class="list-disc pl-6 space-y-1 text-left">
          <li class="text-sm">{{ $t("laboratory.tips.step1") }}</li>
          <li class="text-sm" v-html="$t('laboratory.tips.step2')"></li>
          <li class="text-sm">{{ $t("laboratory.tips.step3") }}</li>
          <li class="text-sm">{{ $t("laboratory.tips.step5") }}</li>
        </ul>
      </div>
    </div>

    <div class="card bg-base-100 atv-connect-card">
      <h2 class="atv-section-heading">{{ $t("addDevice.title") }}</h2>

      <!-- Device type tabs: Apple TV on the left and selected by default -->
      <div class="join w-full atv-connect-mode" role="tablist">
        <button
          type="button"
          role="tab"
          class="btn join-item flex-1 gap-x-2"
          :class="{ 'btn-primary': activeTab === 'appletv' }"
          :aria-selected="activeTab === 'appletv'"
          :aria-label="$t('addDevice.tabs.appletv')"
          @click="selectTab('appletv')"
        >
          <span class="w-6 h-6 atv-tab-icon">
            <AppleTVTabIcon />
          </span>
          {{ $t("addDevice.tabs.appletv") }}
        </button>
        <button
          type="button"
          role="tab"
          class="btn join-item flex-1 gap-x-2"
          :class="{ 'btn-primary': activeTab === 'iphone' }"
          :aria-selected="activeTab === 'iphone'"
          :aria-label="$t('addDevice.tabs.iphone')"
          @click="selectTab('iphone')"
        >
          <span class="w-6 h-6 atv-tab-icon">
            <IPhoneTabIcon />
          </span>
          {{ $t("addDevice.tabs.iphone") }}
        </button>
      </div>

      <!-- Apple TV: pair over the network, no port and no pairing file -->
      <div v-show="activeTab === 'appletv'" class="flex flex-col gap-y-4 mt-2">
        <div class="form-control w-full">
          <label class="label">
            <span class="label-text">{{ $t("addDevice.appletv.device_label") }}</span>
          </label>
          <DeviceComboBox
            v-model="atvIp"
            :options="atvOptions"
            :placeholder="$t('addDevice.appletv.ip_placeholder')"
            :refresh-label="$t('laboratory.form.refresh_devices')"
            :refreshing="loadingAtv"
            @refresh="refreshAppleTVDevices"
          />
          <label class="label">
            <span class="label-text-alt whitespace-normal">{{ $t("addDevice.appletv.ip_tips") }}</span>
          </label>
        </div>

        <button
          class="btn btn-primary w-full"
          type="button"
          :disabled="!atvIp"
          @click="startPair"
        >
          {{ $t("addDevice.appletv.submit") }}
        </button>
      </div>

      <!-- iPhone: pick a device and validate an uploaded pairing file -->
      <div v-show="activeTab === 'iphone'" class="flex flex-col gap-y-4 mt-2">
        <div class="form-control w-full">
          <label class="label">
            <span class="label-text">{{ $t("addDevice.iphone.device_label") }}</span>
          </label>
          <DeviceComboBox
            v-model="iphoneIp"
            :options="iphoneOptions"
            :placeholder="$t('addDevice.iphone.device_placeholder')"
            :refresh-label="$t('laboratory.form.refresh_devices')"
            :refreshing="loadingIphone"
            @refresh="loadIphoneDevices"
          />
          <label class="label">
            <span class="label-text-alt whitespace-normal">{{ $t("addDevice.iphone.device_tips") }}</span>
          </label>
        </div>

        <div class="form-control w-full">
          <label class="label">
            <span class="label-text">{{ $t("addDevice.iphone.upload_label") }}</span>
          </label>
          <input
            type="file"
            class="file-input file-input-bordered w-full"
            @change="onFileChange"
          />
        </div>

        <button
          class="btn btn-primary w-full"
          type="button"
          :disabled="connecting"
          @click="startConnect"
        >
          <span class="loading loading-spinner" v-show="connecting"></span>
          {{ $t("addDevice.iphone.submit") }}
        </button>
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
import { truncateIP } from "@/utils/utils";
import DeviceComboBox from "@/components/DeviceComboBox.vue";

export default {
  name: "AddDevice",
  components: { DeviceComboBox },
  data() {
    return {
      activeTab: "appletv",
      // Minimum spinner duration for the Apple TV list (first load and
      // manual refresh): the devices endpoint answers almost instantly,
      // so the animation would otherwise not be visible.
      appleTVSpinnerMs: 1000,
      // Each tab's list is fetched only the first time it is shown;
      // later switches reuse the cached list.
      atvLoaded: false,
      iphoneLoaded: false,
      // Apple TV tab
      atvDevices: [],
      atvIp: "",
      loadingAtv: false,
      // iPhone tab
      iphoneDevices: [],
      iphoneIp: "",
      loadingIphone: false,
      files: [],
      connecting: false,
      confirmDialogVisible: false,
      // Parameters remembered for the overwrite confirmation of an import.
      pendingImport: null,
    };
  },
  computed: {
    atvOptions() {
      return this.atvDevices.map((device) => ({
        value: device.ip,
        label: `${device.name} (${truncateIP(device.ip)})`,
      }));
    },
    iphoneOptions() {
      return this.iphoneDevices.map((device) => ({
        value: device.ip,
        label: `${device.name} (${truncateIP(device.ip)})`,
      }));
    },
  },
  mounted() {
    // The Apple TV tab is the default view, so its list loads on open; the
    // iPhone list loads the first time its tab is opened.
    this.loadAppleTVDevices(this.appleTVSpinnerMs);
  },
  methods: {
    selectTab(tab) {
      this.activeTab = tab;
      // A tab's list is refreshed only the first time it is shown; later
      // switches reuse the loaded list. The combobox refresh button
      // reloads on demand.
      if (tab === "appletv" && !this.atvLoaded) {
        this.loadAppleTVDevices(this.appleTVSpinnerMs);
      } else if (tab === "iphone" && !this.iphoneLoaded) {
        this.loadIphoneDevices();
      }
    },
    isAppleTV(device) {
      const cls = (device.device_class || "").toLowerCase();
      return cls !== "iphone" && cls !== "ipad";
    },
    isIPhone(device) {
      const cls = (device.device_class || "").toLowerCase();
      return cls === "iphone" || cls === "ipad";
    },
    async loadAppleTVDevices(minSpinnerMs = 0) {
      // The devices endpoint answers almost instantly, so without a minimum
      // duration the spinner would flash by unnoticed. Keep it up for at
      // least the requested time so first load and refresh both give
      // visible feedback.
      const startedAt = Date.now();
      this.loadingAtv = true;
      try {
        const res = await api.getDevices();
        // Only Apple TV devices that are currently advertising the manual
        // pairing service can be handed to the existing pairing flow; devices
        // without a known class are also kept.
        this.atvDevices = (res.data || []).filter(
          (device) =>
            device.status === "pairable" &&
            (!device.device_class || this.isAppleTV(device)),
        );
        this.atvLoaded = true;
      } catch (error) {
        console.error("Failed to load devices:", error);
      } finally {
        const elapsed = Date.now() - startedAt;
        if (minSpinnerMs > elapsed) {
          await new Promise((resolve) => setTimeout(resolve, minSpinnerMs - elapsed));
        }
        this.loadingAtv = false;
      }
    },
    // Manual refresh from the combobox button: hold the spinner for the
    // same minimum duration as the initial load.
    refreshAppleTVDevices() {
      this.loadAppleTVDevices(this.appleTVSpinnerMs);
    },
    async loadIphoneDevices() {
      this.loadingIphone = true;
      try {
        const res = await api.scanWireless();
        // The iPhone tab lists iPhone/iPad devices; a device without a known
        // class is also kept so a directly-entered IP can still be used.
        this.iphoneDevices = (res.data || []).filter(
          (device) => !device.device_class || this.isIPhone(device),
        );
        this.iphoneLoaded = true;
      } catch (error) {
        console.error("Failed to load devices:", error);
        toast.error(
          this.$t("laboratory.toast.load_devices_failed", {
            msg: error.message || "未知错误",
          }),
        );
      } finally {
        this.loadingIphone = false;
      }
    },
    // findDevice matches a typed IP against the current list. A miss triggers
    // one more scan: the user may have typed the IP before the device was
    // listed, and a fresh discovery can resolve its port (and Apple TV id).
    async findDevice(getList, reload, ip) {
      const match = (getList() || []).find((item) => item.ip === ip);
      if (match) return match;

      await reload();
      return (getList() || []).find((item) => item.ip === ip);
    },
    async startPair() {
      const ip = (this.atvIp || "").trim();
      if (!ip) {
        toast.error(this.$t("addDevice.toast.device_required"));
        return;
      }
      const device = await this.findDevice(() => this.atvDevices, this.loadAppleTVDevices, ip);
      if (device) {
        // Discovered device: hand it to the existing pairing flow page.
        this.$router.push({ name: "pair", params: { id: device.id } });
        return;
      }
      // Manual IP: open the pair page in manual mode, which discovers the
      // pairing port before allowing pairing.
      this.$router.push({ name: "pair-manual", query: { ip } });
    },
    onFileChange(e) {
      this.files = e.target.files;
    },
    async startConnect() {
      if (!this.files || this.files.length === 0) {
        toast.error(this.$t("addDevice.toast.file_required"));
        return;
      }
      const ip = (this.iphoneIp || "").trim();
      if (!ip) {
        toast.error(this.$t("addDevice.toast.device_required"));
        return;
      }

      await this.connectIPhone(ip);
    },
    async connectIPhone(ip) {
      // A discovered device already carries its _remotepairing tunnel port.
      const known = (this.iphoneDevices || []).find((item) => item.ip === ip);
      if (known && known.port) {
        await this.submitImport(ip, known.port, false, false);
        return;
      }

      // Manual IP: discover the tunnel port, then validate the pairing file
      // over it with --manual. The button spinner covers both steps.
      this.connecting = true;
      let tunnel;
      try {
        const res = await api.discoverTunnelPort(ip);
        tunnel = res && res.data;
      } catch (error) {
        console.error("Failed to discover tunnel port:", error);
        tunnel = null;
      } finally {
        this.connecting = false;
      }
      if (!tunnel || !tunnel.port) {
        toast.error(this.$t("addDevice.iphone.no_device"));
        return;
      }

      await this.submitImport(ip, tunnel.port, true, false);
    },
    async submitImport(ip, port, manual, override) {
      this.connecting = true;
      try {
        const formData = new FormData();
        formData.append("file", this.files[0]);
        if (override) {
          formData.append("override", "true");
        }
        if (manual) {
          formData.append("manual", "true");
        }
        formData.append("ip", ip);
        formData.append("port", port);

        await api.importPair(formData);
        toast.success(this.$t("laboratory.toast.import_success"));
        this.files = [];
        this.confirmDialogVisible = false;
        const input = this.$el.querySelector('input[type="file"]');
        if (input) input.value = "";
      } catch (error) {
        console.error(error);
        if (
          !override &&
          error.message &&
          error.message.includes("pairing file already exists")
        ) {
          // Remember how to resubmit on overwrite confirmation.
          this.pendingImport = { ip, port, manual };
          this.confirmDialogVisible = true;
        }
      } finally {
        this.connecting = false;
      }
    },
    async onConfirmOverride() {
      const pending = this.pendingImport;
      this.pendingImport = null;
      this.confirmDialogVisible = false;
      if (!pending) {
        return;
      }
      await this.submitImport(pending.ip, pending.port, pending.manual, true);
    },
  },
};
</script>

<script setup>
import WarningIcon from "@/assets/icons/warning.svg";
import AppleTVTabIcon from "@/assets/icons/appletv-tab.svg";
import IPhoneTabIcon from "@/assets/icons/iphone-tab.svg";
</script>

<style lang="postcss" scoped>
.atv-connect-card {
  border: 1px solid var(--atv-border);
  border-radius: 22px;
  padding: 26px 30px 30px;
}

/* Device-type switch reuses the install page's segmented join buttons;
   this only sets the gap to the form below. */
.atv-connect-mode {
  margin-bottom: 20px;
}

/* Tab icons are monochrome line glyphs drawn with currentColor: they follow
   the button's text colour, so selecting a tab (btn-primary) turns them
   white along with the label. */
.atv-tab-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: inherit;
}

.atv-tab-icon svg {
  width: 100%;
  height: 100%;
}

@media (max-width: 640px) {
  .atv-connect-card {
    padding: 20px;
  }
}
</style>
