import { reactive } from "vue";
import api from "@/api/api";
import i18n from "@/i18n";
import { toast } from "vue3-toastify";

// Shared state for the phone left drawer and its device panel. The layout owns
// the drawer (it is available on every page), and the home page reads the same
// data for its desktop left column, so both share one store and a single source
// of fetched devices/services instead of two independent copies.
export const homeShell = reactive({
  // Viewport has the phone drawer (phones only); set by the layout.
  available: false,
  // The drawer is open.
  open: false,
  // Devices / services shown in the sidebar panel.
  devices: [],
  services: [],
  // Id of a manual device armed for the "forget" confirmation.
  forgetConfirmId: null,
  // Whether the shared device data has been requested at least once.
  loaded: false,

  setAvailable(available) {
    this.available = available;
    if (!available) this.open = false;
  },
  toggle() {
    this.open = !this.open;
  },
  close() {
    this.open = false;
  },

  // Load devices + services once; later calls reuse the cached data. Pass
  // force to refresh after a mutation (e.g. forgetting a device).
  load(force = false) {
    if (this.loaded && !force) return;
    this.loaded = true;
    api.getDevices().then((res) => {
      this.devices = res.data || [];
    });
    api.getServiceStatus().then((res) => {
      this.services = res.data || [];
    });
  },

  // Arm or confirm deleting a device. Resolves true when the device was
  // actually deleted, so the caller can refresh its view.
  async forgetDevice(device) {
    if (this.forgetConfirmId !== device.id) {
      this.forgetConfirmId = device.id;
      return false;
    }
    this.forgetConfirmId = null;
    try {
      await api.deleteDevice(device.id);
      toast.success(i18n.t("home.sidebar.device_status.forget_done"));
      this.loaded = false;
      this.load(true);
      return true;
    } catch (error) {
      console.error(error);
      return false;
    }
  },
});
