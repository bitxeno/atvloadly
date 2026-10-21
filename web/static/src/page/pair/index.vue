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
        <p><label class="inline-block w-16">IP：</label>{{ truncateIP(device.ip) }}</p>
        <button class="btn btn-primary" @click="start" :disabled="!device.id">
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
            ref="pinInput"
            type="text"
            inputmode="numeric"
            pattern="[0-9]*"
            maxlength="6"
            autocomplete="one-time-code"
            :placeholder="$t('pair.step.pin.placeholder')"
            class="input input-bordered input-primary w-full"
            v-model="pin"
            @input="onPinInput"
          />
          <button
            class="btn btn-primary"
            @click="confirmPin"
            :disabled="status.confirm || !pin"
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

        <p v-if="!success" role="alert">{{ msg }}</p>

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
import { truncateIP } from "@/utils/utils";
export default {
  data() {
    return {
      active: 0,
      pin: "",
      loading: false,
      id: "",
      device: {},
      output: "",
      pairTimer: null,
      pendingStart: false,
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
  beforeUnmount() {
    clearTimeout(this.pairTimer);
    if (this.websock) {
      this.websock.onopen = null;
      this.websock.onerror = null;
      this.websock.onmessage = null;
      this.websock.onclose = null;
      this.websock.close();
    }
    this.pin = "";
  },
  methods: {
    fetchData() {
      let _this = this;
      api.getDevice(_this.id).then((res) => {
        _this.device = res.data;
      });
    },
    start() {
      if (this.active !== 0) return;
      this.active = 1;
      this.loading = true;
      this.output = "";
      this.pairTimer = setTimeout(() => {
        this.finishPair(false, "pair.toast.timeout");
      }, 75000);
      if (this.websock.readyState === WebSocket.CONNECTING) {
        this.pendingStart = true;
      } else {
        this.websocketsend(1, this.device.id || this.id);
      }
    },
    confirmPin() {
      if (this.active !== 1 || this.status.confirm || !this.pin) return;
      this.status.confirm = true;
      this.websocketsend(2, this.pin);
      this.pin = "";
    },
    finishPair(success, message = "pair.toast.pair_error") {
      if (this.active !== 1) return;
      clearTimeout(this.pairTimer);
      this.pairTimer = null;
      this.pendingStart = false;
      this.active = 2;
      this.success = success;
      this.loading = false;
      this.status.confirm = false;
      this.pin = "";
      this.output = "";
      this.msg = success ? "" : this.$t(message);
      if (!success) toast.error(this.msg);
      this.websock.close();
    },
    onPinInput() {
      this.pin = this.pin.replace(/\D/g, "").slice(0, 6);
    },
    goback() {
      this.$router.push("/");
    },
    initWebSocket() {
      const wsuri =
        (location.protocol === "https:" ? "wss://" : "ws://") +
        location.host +
        "/ws/pair";
      this.websock = new WebSocket(wsuri);
      this.websock.onopen = this.websocketonopen;
      this.websock.onerror = this.websocketonerror;
      this.websock.onmessage = this.websocketonmessage;
      this.websock.onclose = this.websocketclose;
    },
    websocketonopen() {
      if (this.pendingStart && this.active === 1) {
        this.pendingStart = false;
        this.websocketsend(1, this.device.id || this.id);
      }
    },
    websocketonerror() {
      this.finishPair(false, "pair.toast.connection_lost");
    },
    websocketonmessage(e) {
      if (this.active !== 1) return;
      if (e.data === "Pairing Succeeded!") {
        this.finishPair(true);
        return;
      }
      if (e.data === "Pairing Failed!") {
        const message = this.output.includes("Invalid PIN")
          ? "pair.toast.pin_incorrect"
          : this.output.includes("No device found")
            ? "pair.toast.no_device_found"
            : "pair.toast.pair_error";
        this.finishPair(false, message);
        return;
      }
      this.output = (this.output + e.data).slice(-16384);
      if (this.loading && this.output.includes("Enter PIN")) {
        this.pin = "";
        this.loading = false;
        this.$nextTick(() => this.$refs.pinInput?.focus());
      }
    },
    websocketsend(t, data) {
      if (this.websock.readyState !== WebSocket.OPEN) {
        this.finishPair(false, "pair.toast.connection_lost");
        return;
      }
      try {
        this.websock.send(JSON.stringify({ t, d: data }));
      } catch {
        this.finishPair(false, "pair.toast.connection_lost");
      }
    },
    websocketclose() {
      this.finishPair(false, "pair.toast.connection_lost");
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
