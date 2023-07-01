<template>
  <div class="max-w-screen-md mx-auto flex flex-col gap-y-8">
    <ul class="steps w-full">
      <li class="step step-primary">开始</li>
      <li :class="['step', { 'step-primary': active > 0 }]">输入PIN码</li>
      <li :class="['step', { 'step-primary': active > 1 }]">配对完成</li>
    </ul>

    <div class="border rounded p-6">
      <div v-show="active === 0" class="flex flex-col gap-y-4">
        <p><label class="inline-block w-16">设备：</label>{{ device.name }}</p>
        <p><label class="inline-block w-16">IP：</label>{{ device.ip }}</p>
        <p>
          <label class="inline-block w-16">MAC：</label>{{ device.mac_addr }}
        </p>
        <button class="btn btn-primary" @click="start">开始配对</button>
      </div>

      <div v-show="active === 1">
        <div v-show="loading" class="flex flex-row justify-center gap-x-1">
          <span class="loading loading-spinner loading-sm"></span>连接中...
        </div>
        <div v-show="!loading" class="flex flex-col gap-y-4">
          <input
            type="number"
            placeholder="请输入 AppleTV 上显示的 PIN 码"
            class="input input-bordered input-primary w-full"
            v-model="pin"
          />
          <button class="btn btn-primary" @click="confirmPin" :disabled="status.confirm">
            <span class="loading loading-spinner" v-show="status.confirm"></span>确定</button>
        </div>
      </div>

      <div v-show="active === 2" class="flex flex-col gap-y-4">
        <div v-show="success" class="flex flex-col items-center justify-center">
          <div class="w-16 text-green-500">
            <CheckMarkIcon />
          </div>
          配对成功
        </div>

        <button class="btn" @click="goback">返回</button>
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
      status : {
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
      _this.websocketsend(`idevicepair pair -u ${uuid} -w`);
    },
    confirmPin() {
      let _this = this;
      _this.status.confirm = true;

      _this.websocketsend(_this.pin);
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
      //初始化weosocket
      const wsuri =
        (location.protocol === "https:" ? "wss://" : "ws://") +
        location.host +
        "/ws/tty"; //ws地址
      console.log(wsuri);
      this.websock = new WebSocket(wsuri);
      this.websock.onopen = this.websocketonopen;
      this.websock.onerror = this.websocketonerror;
      this.websock.onmessage = this.websocketonmessage;
      this.websock.onclose = this.websocketclose;
    },

    websocketonopen() {
      console.log("WebSocket连接成功");
    },
    websocketonerror(e) {
      //错误
      console.log("WebSocket连接发生错误");
    },
    websocketonmessage(e) {
      let _this = this;

      //数据接收
      // const redata = JSON.parse(e.data); // 接收数据
      _this.cmd.output += e.data;
      _this.cmd.line += e.data;
      if (e.data.indexOf("\n") >= 0) {
        console.log("<--", _this.cmd.line);
        _this.cmd.line = "";
      }

      if (_this.active === 1) {
        // 提示输入PIN
        if (_this.cmd.output.indexOf("Enter PIN") !== -1) {
          _this.loading = false;
          _this.cmd.output = "";
          return;
        }

        // 配对出错
        if (_this.cmd.output.indexOf("Invalid PIN") !== -1) {
          // 配对出错，显示出错消息
          _this.cmd.output = "";
          _this.active = 2;
          toast.error("PIN码不正确");
          return;
        }
        if (_this.cmd.output.indexOf("ERROR") !== -1) {
          // 配对出错，显示出错消息
          _this.cmd.output = "";
          _this.active = 2;
          toast.error("配对出错，请到控制台查看详细信息");
          return;
        }

        if (_this.cmd.output.indexOf("No device found") !== -1) {
          // usbmuxd未启动
          _this.cmd.output = "";
          _this.active = 2;
          toast.error("找不到设备，请确认 usbmuxd 服务已启动");
          return;
        }

        // 配对成功
        if (_this.cmd.output.indexOf("SUCCESS") !== -1) {
          _this.cmd.output = "";
          _this.active++;
          _this.success = true;
          return;
        }
      }
    },

    websocketsend(cmd) {
      let _this = this;
      //数据发送
      _this.cmd.output = "";
      const json = JSON.stringify({ t: 1, d: `${cmd}\n` });
      console.log("--> ", json);
      _this.websock.send(json);
    },

    websocketclose(e) {
      //关闭
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
  