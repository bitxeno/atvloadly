<template>
  <div class="max-w-screen-md mx-auto flex flex-col gap-y-6">
    <div class="alert alert-warning">
      <div class="w-8">
        <WarningIcon />
      </div>
      <span class="text-sm"
        >首次安装时，需要授权信任本设备。请确保身边有已登录了安装帐号的 iPhone
        手机，并及时输入显示的验证码。如果超时未验证，将导致帐号被临时冻结，需重置密码才能解除冻结状态。</span
      >
    </div>

    <div class="border rounded p-6">
      <div class="lg:flex lg:flex-row">
        <div class="flex flex-col justify-center place-items-center gap-y-4">
          <div class="w-32 rounded">
            <AppleTVIcon />
          </div>
          <span>{{ device.host }} ({{ device.ip }})</span>
        </div>

        <div class="divider divider-horizontal"></div>

        <div class="p-6 flex flex-col gap-y-4">
          <form id="form" class="flex flex-col gap-y-4">
            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">选择ipa:</span>
              </label>
              <input
                type="file"
                class="file-input file-input-bordered w-full"
                @change="onFileChange"
                accept=".ipa"
                required
              />
            </div>

            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">Apple帐号:</span>
              </label>
              <input
                type="email"
                placeholder="xxxx@example.com"
                class="input input-bordered w-full"
                v-model="form.account"
                required
              />
              <label class="label">
                <span class="label-text-alt stat-title"
                  >为了帐号安全，请不要使用常用帐号安装</span
                >
              </label>
            </div>

            <div class="form-control w-full">
              <label class="label">
                <span class="label-text">Apple密码:</span>
              </label>
              <input
                type="password"
                class="input input-bordered w-full"
                v-model="form.password"
                required
              />
            </div>
          </form>

          <div class="flex flex-row gap-x-4">
            <button class="btn flex-1" @click="goBack">返 回</button>
            <button
              class="btn btn-primary flex-1"
              @click="onSubmit"
              :disabled="loading"
            >
              <span class="loading loading-spinner" v-show="loading"></span>安
              装
            </button>
          </div>
        </div>
      </div>

      <dialog
        id="auth_modal"
        :class="['modal', { 'modal-open': dialogVisible }]"
      >
        <form id="dialog" method="dialog" class="modal-box">
          <h3 class="font-bold text-lg">请输入 iPhone 上显示的验证码</h3>
          <p class="py-4">
            <input
              type="number"
              class="input input-bordered input-primary w-full"
              placeholder="请在 iPhone 上允许设备并输入显示的验证码"
              v-model="form.authcode"
              required
            />
          </p>
          <div class="modal-action">
            <button class="btn btn-primary" @click="onSubmit2FA">确 定</button>
          </div>
        </form>
      </dialog>
    </div>

    <div v-show="log.show">
      <textarea
        id="log"
        class="textarea textarea-bordered w-full h-48 bg-neutral text-base-100 leading-5"
        wrap="off"
        v-model="log.output"
      ></textarea>
    </div>
  </div>
</template>
  
  <script>
import api from "@/api/api";
import { toast } from "vue3-toastify";

export default {
  data() {
    return {
      id: "",
      files: [],
      ipa: {},
      device: {},
      loading: false,
      form: {
        account: "",
        password: "",
        authcode: "",
      },
      cmd: {
        output: "",
        line: "",
      },
      log: {
        output: "",
        show: false,
      },
      dialogVisible: false,
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
    onSubmit(e) {
      let _this = this;

      if (!_this.validateForm("#form")) {
        return;
      }

      _this.loading = true;
      _this.log.output = "";
      _this.log.show = true;
      let formData = new FormData();
      for (let i = 0; i < _this.files.length; i++) {
        let file = _this.files[i];
        formData.append("files", file);
      }
      api.upload(formData).then((res) => {
        let ipa = res.data[0];
        _this.ipa = ipa;

        // 为每个appleid创建对应的工作目录，用于存储AltServer生成的签名证书
        let dirName = _this.form.account
          .toLowerCase()
          .replace(/[^0-9a-zA-Z]+/gi, "");
        let workdir = `./AltServer/${dirName}`;
        _this.websocketsend(
          `mkdir -p ${workdir} && cd ${workdir} && AltServer -u ${_this.device.udid} -a "${_this.form.account}" -p "${_this.form.password}" "${ipa.path}"`
        );
      });
    },
    onCancel() {
      this.$message({
        message: "cancel!",
        type: "warning",
      });
    },
    reset() {
      document.getElementById("form").reset();
    },
    goBack() {
      this.$router.push("/");
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
    onSubmit2FA() {
      let _this = this;

      if (!_this.validateForm("#dialog")) {
        return;
      }

      _this.websocketsend(_this.form.authcode);
      _this.dialogVisible = false;
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
        // 打码密码字符串
        _this.cmd.output = _this.cmd.output.replace(
          _this.form.password,
          "******"
        );
        _this.cmd.line = _this.cmd.line.replace(_this.form.password, "******");

        if (
          _this.cmd.line.indexOf("Signing Progress") === -1 &&
          _this.cmd.line.indexOf("AltServer -u") === -1
        ) {
          _this.log.output += _this.cmd.line;
          // 文本框跟随滚动到底部
          _this.$nextTick(() => {
            const textarea = document.querySelector("#log");
            textarea.scrollTop = textarea.scrollHeight;
          });
        }

        _this.cmd.line = "";
      }

      // 2FA认证码输入
      if (_this.cmd.output.indexOf("Enter two factor code") !== -1) {
        _this.cmd.output = "";
        _this.dialogVisible = true;
        return;
      }

      // 覆盖之前同一appleid安装
      if (
        _this.cmd.output.indexOf(
          "Installing AltStore with Multiple AltServers Not Supported"
        ) !== -1
      ) {
        if (_this.cmd.output.indexOf("Press any key to continue...") !== -1) {
          _this.cmd.output = "";
          _this.websocketsend("");
          return;
        }
      }

      // 配对出错
      if (
        _this.cmd.output.indexOf("Could not install") !== -1 ||
        _this.cmd.output.indexOf("command not found") !== -1
      ) {
        // 配对出错，显示出错消息
        _this.cmd.output = "";
        _this.loading = false;
        toast.error("安装失败，请查看日志了解详细信息");
        return;
      }

      // 安装成功
      if (_this.cmd.output.indexOf("Installation Succeeded") !== -1) {
        _this.cmd.output = "";
        _this.loading = false;

        // 保存安装记录
        api
          .saveApp({
            ID: 0,
            ipa_name: _this.ipa.name,
            ipa_path: _this.ipa.path,
            device: _this.device.mac_addr,
            udid: _this.device.udid,
            account: _this.form.account,
            password: _this.form.password,
            refreshed_result: true,
            icon: _this.ipa.icon,
            bundle_identifier: _this.ipa.bundle_identifier,
            version: _this.ipa.version,
          })
          .then((res) => {
            toast.success("安装成功");
          });

        return;
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
import AppleTVIcon from "@/assets/icons/appletv.svg";
import WarningIcon from "@/assets/icons/warning.svg";
</script>
  
  <style scoped>
.line {
  text-align: center;
}
</style>
  