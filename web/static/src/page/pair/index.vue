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
          >{{ deviceLabel }}
        </p>
        <p><label class="inline-block w-16">IP：</label>{{ truncateIP(deviceIp) }}</p>

        <!-- Manual IP: discover the pairing port before pairing is allowed -->
        <div v-if="isManual" class="atv-discovery" :class="'atv-discovery--' + discovery.stage">
          <div class="atv-discovery-head">
            <span
              v-if="discovery.stage === 'found'"
              class="w-5 h-5 text-green-500"
            ><CheckMarkIcon /></span>
            <span
              v-else-if="discovery.stage === 'failed'"
              class="w-5 h-5 text-red-500"
            ><DismissIcon /></span>
            <span
              v-else
              class="loading loading-spinner loading-sm"
            ></span>
            <span class="atv-discovery-title">{{ discoveryTitle }}</span>
          </div>

          <!-- Scanning progress bar -->
          <div
            v-if="discovery.stage === 'mdns' || discovery.stage === 'scanning' || discovery.stage === 'probing'"
            class="atv-discovery-bar"
          >
            <div class="atv-discovery-bar-fill" :style="{ width: discoveryBarWidth }"></div>
          </div>

          <p v-if="discovery.stage === 'found'" class="atv-discovery-detail">
            {{ $t("pair.manual.found", { port: discovery.port }) }}
            <span v-if="discovery.name">· {{ discovery.name }}</span>
          </p>
          <p v-else-if="discovery.stage === 'failed'" class="atv-discovery-detail atv-discovery-detail--error">
            {{ $t("pair.manual.failed") }}
          </p>

          <button
            v-if="discovery.stage === 'failed' || discovery.stage === 'found'"
            type="button"
            class="btn btn-sm btn-soft atv-discovery-retry"
            @click="discover"
          >
            <span class="w-4 h-4"><RefreshIcon /></span>
            {{ $t("pair.manual.retry") }}
          </button>
        </div>

        <button
          class="btn btn-primary"
          @click="start"
          :disabled="isManual && discovery.stage !== 'found'"
        >
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
import { truncateIP } from "@/utils/utils";
export default {
  data() {
    return {
      active: 0,
      pin: "",
      loading: false,
      id: "",
      device: {},
      // Manual-IP pairing state
      isManual: false,
      manualIp: "",
      discovery: {
        stage: "", // mdns | scanning | probing | found | failed
        percent: 0,
        port: 0,
        name: "",
      },
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
  computed: {
    deviceIp() {
      return this.isManual ? this.manualIp : this.device.ip;
    },
    deviceLabel() {
      if (this.isManual) {
        return this.device.name || this.manualIp;
      }
      return this.device.name;
    },
    discoveryTitle() {
      switch (this.discovery.stage) {
        case "mdns":
          return this.$t("pair.manual.stage_mdns");
        case "scanning":
          return this.$t("pair.manual.stage_scanning", { percent: this.discovery.percent });
        case "probing":
          return this.$t("pair.manual.stage_probing");
        case "found":
          return this.$t("pair.manual.stage_found");
        case "failed":
          return this.$t("pair.manual.stage_failed");
        default:
          return this.$t("pair.manual.discovering");
      }
    },
    discoveryBarWidth() {
      const pct = Math.min(100, Math.max(0, this.discovery.percent || 0));
      return pct + "%";
    },
  },
  created() {
    this.isManual = this.$route.name === "pair-manual" || !!this.$route.query.ip;
    this.id = this.$route.params.id || "";

    this.initWebSocket();

    if (this.isManual) {
      this.manualIp = (this.$route.query.ip || "").toString();
      this.device = { ip: this.manualIp, name: this.manualIp };
      // Discovery starts in websocketonopen: the socket is still CONNECTING
      // here, so the start button stays disabled until the port is found.
    } else {
      this.fetchData();
    }
  },
  unmounted() {
    if (this.websock) {
      try {
        this.websock.close();
      } catch (e) {
        /* ignore */
      }
    }
  },
  methods: {
    fetchData() {
      let _this = this;
      api.getDevice(_this.id).then((res) => {
        _this.device = res.data;
      });
    },
    discover() {
      let _this = this;
      _this.discovery = { stage: "", percent: 0, port: 0, name: "" };
      // t=3: request manual pairing port discovery for the entered IP.
      _this.websocketsend(3, _this.manualIp);
    },
    start() {
      let _this = this;
      _this.active = 1;

      _this.loading = true;
      if (_this.isManual) {
        // t=4: pair the manually entered IP with the discovered port.
        _this.websocketsend(4, { ip: _this.manualIp, port: _this.discovery.port });
      } else {
        _this.websocketsend(1, _this.device.id || _this.id);
      }
    },
    confirmPin() {
      let _this = this;
      _this.status.confirm = true;

      _this.websocketsend(2, _this.pin);
      return;
    },
    onPinInput() {
      this.pin = this.pin.replace(/\D/g, "").slice(0, 6);
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
      // Manual mode: begin discovery once the socket is open.
      if (this.isManual) {
        this.discover();
      }
    },
    websocketonerror(e) {
      console.log("WebSocket connect failed.");
    },
    websocketonmessage(e) {
      let _this = this;

      let line = e.data;

      // Discovery progress arrives as a JSON object (leading '{'); everything
      // else is raw pairing output.
      if (typeof line === "string" && line.charAt(0) === "{") {
        let msg;
        try {
          msg = JSON.parse(line);
        } catch (err) {
          msg = null;
        }
        if (msg && msg.kind === "discover") {
          _this.discovery.stage = msg.stage || "";
          _this.discovery.percent = msg.percent || 0;
          if (msg.stage === "found") {
            _this.discovery.port = msg.port || 0;
            _this.discovery.name = msg.name || "";
          }
          return;
        }
      }

      if (_this.active === 1) {
        // show Enter PIN
        if (line.indexOf("Enter PIN") !== -1) {
          _this.pin = "";
          _this.loading = false;
          _this.$nextTick(() => {
            _this.$refs.pinInput?.focus();
          });
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
          console.log(line);
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
      // Guard against a send while the socket is still CONNECTING (0) or
      // closing. readyState 1 === OPEN; a test double may omit it.
      const sock = _this.websock;
      if (!sock) {
        return;
      }
      if (sock.readyState !== undefined && sock.readyState !== 1) {
        return;
      }
      if (typeof data !== 'string') {
        data = JSON.stringify(data);
      }
      const json = JSON.stringify({ t: t, d: data });
      sock.send(json);
    },

    websocketclose(e) {
      console.log(`connection closed (${e.code})`);
    },
  },
};
</script>

<script setup>
import CheckMarkIcon from "@/assets/icons/checkmark.svg";
import DismissIcon from "@/assets/icons/dismiss.svg";
import RefreshIcon from "@/assets/icons/refresh.svg";
</script>
<style scoped>
.line {
  text-align: center;
}

/* Manual-IP pairing port discovery panel */
.atv-discovery {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid var(--atv-border);
  border-radius: 12px;
  background: var(--atv-surface-alt);
}

.atv-discovery--found {
  border-color: var(--atv-success-border, #b7e4c7);
  background: var(--atv-success-soft, #f0fbf3);
}

.atv-discovery--failed {
  border-color: var(--atv-danger, #e5a3a3);
  background: var(--atv-danger-soft, #fdf2f2);
}

.atv-discovery-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 650;
  color: var(--atv-ink);
}

.atv-discovery-title {
  font-size: 0.9rem;
}

.atv-discovery-bar {
  height: 6px;
  width: 100%;
  border-radius: 999px;
  background: var(--atv-border);
  overflow: hidden;
}

.atv-discovery-bar-fill {
  height: 100%;
  border-radius: 999px;
  background: var(--atv-accent);
  transition: width 200ms ease;
}

.atv-discovery-detail {
  margin: 0;
  font-size: 0.86rem;
  color: var(--atv-muted);
}

.atv-discovery-detail--error {
  color: var(--atv-danger, #b23b3b);
}

.atv-discovery-retry {
  align-self: flex-start;
  gap: 6px;
}
</style>
