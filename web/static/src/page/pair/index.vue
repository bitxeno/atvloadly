<template>
  <div class="max-w-screen-md mx-auto flex flex-col gap-y-8">
    <ul class="steps w-full">
      <li class="step step-primary">{{ $t("pair.step.start.title") }}</li>
      <li :class="['step', { 'step-primary': active > 0 }]">
        {{ $t("pair.step.pin.title") }}
      </li>
      <li :class="['step', { 'step-primary': active > 1 }]">
        {{ $t("pair.step.completed.title") }}
      </li>
    </ul>

    <div class="border rounded p-6 bg-base-100">
      <div v-show="active === 0" class="flex flex-col gap-y-4">
        <p>
          <label class="inline-block w-16"
            >{{ $t("pair.step.start.device") }}：</label
          >{{ device.name }}
        </p>
        <p><label class="inline-block w-16">IP：</label>{{ device.ip }}</p>
        <p>
          <label class="inline-block w-16">MAC：</label>{{ device.mac_addr }}
        </p>
        <button class="btn btn-primary" @click="start">
          {{ $t("pair.step.start.button") }}
        </button>
      </div>

      <div v-show="active === 1">
        <div v-show="loading" class="flex flex-row justify-center gap-x-1">
          <span class="loading loading-spinner loading-sm"></span
          >{{ $t("pair.step.pin.loading") }}
        </div>
        <div v-show="!loading" class="flex flex-col gap-y-4">
          <input
            type="number"
            :placeholder="$t('pair.step.pin.placeholder')"
            class="input input-bordered input-primary w-full"
            v-model="pin"
          />
          <button
            class="btn btn-primary"
            @click="confirmPin"
            :disabled="status.confirm"
          >
            <span class="loading loading-spinner" v-show="status.confirm"></span
            >{{ $t("pair.step.pin.button") }}
          </button>
        </div>
      </div>

      <div v-show="active === 2" class="flex flex-col gap-y-4">
        <div v-show="success" class="flex flex-col items-center justify-center">
          <div class="w-16 text-green-500">
            <CheckMarkIcon />
          </div>
          {{ $t("pair.step.completed.msg") }}
        </div>

        <button class="btn" @click="goback">
          {{ $t("pair.step.completed.button") }}
        </button>
      </div>
    </div>
  </div>
</template>
  
  <script>
import api from "@/api/api";
import { toast } from "vue3-toastify";
export default {
  data() {
    return {
      active: 0,
      pin: "",
      loading: false,
      id: "",
      device: {},
      cmd: {
        output: "",
        line: "",
      },
      success: false,
      status: {
        confirm: false,
      },
      msg: "",
    };
  },
  created() {
    this.id = this.$route.params.id;

    this.initWebSocket();
    this.fetchData();
  },
  methods: {
    fetchData() {
      let _this = this;
      api.getDevice(_this.id).then((res) => {
        _this.device = res.data;
      });
    },
    start() {
      let _this = this;
      _this.active = 1;

      _this.loading = true;
      const uuid = _this.device.udid;
      _this.websocketsend(1, uuid);
    },
    confirmPin() {
      let _this = this;
      _this.status.confirm = true;

      _this.websocketsend(2, _this.pin);
      return;
    },
    onSubmit() {
      this.$message("submit!");
    },
    onCancel() {
      this.$message({
        message: "cancel!",
        type: "warning",
      });
    },
    goback() {
      this.$router.push("/");
    },
    initWebSocket() {
      const wsuri =
        (location.protocol === "https:" ? "wss://" : "ws://") +
        location.host +
        "/ws/pair";
      console.log(wsuri);
      this.websock = new WebSocket(wsuri);
      this.websock.onopen = this.websocketonopen;
      this.websock.onerror = this.websocketonerror;
      this.websock.onmessage = this.websocketonmessage;
      this.websock.onclose = this.websocketclose;
    },
    websocketonopen() {
      console.log("WebSocket connect success.");
    },
    websocketonerror(e) {
      console.log("WebSocket connect failed.");
    },
    websocketonmessage(e) {
      let _this = this;

      let line = e.data

      if (_this.active === 1) {
        // show Enter PIN
        if (line.indexOf("Enter PIN") !== -1) {
          _this.pin = "";
          _this.loading = false;
          line = "";
          return;
        }

        // pairing error
        if (line.indexOf("Invalid PIN") !== -1) {
          _this.active = 2;
          toast.error(this.$t("pair.toast.pin_incorrect"));
          return;
        }
        if (line.indexOf("ERROR") !== -1) {
          _this.active = 2;
          toast.error(this.$t("pair.toast.pair_error"));
          return;
        }

        if (line.indexOf("No device found") !== -1) {
          // usbmuxd service not started
          _this.active = 2;
          toast.error(this.$t("pair.toast.no_device_found"));
          return;
        }

        // pairing successful.
        if (line.indexOf("SUCCESS") !== -1) {
          _this.active++;
          _this.success = true;
          return;
        }
      }
    },

    websocketsend(t, data) {
      let _this = this;
      if (typeof data !== 'string') {
        data = JSON.stringify(data);
      }
      const json = JSON.stringify({ t: t, d: data });
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
import CheckMarkIcon from "@/assets/icons/checkmark.svg";
</script>

  
  <style scoped>
.line {
  text-align: center;
}
</style>
  