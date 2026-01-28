<template>
  <div>
       <!-- Login Modal -->
    <dialog
      ref="loginModal"
      :class="['modal', { 'modal-open': loginDialogVisible }]"
    >
      <div class="modal-box">
        <h3 class="font-bold text-lg">{{ $t("install.login_modal.title") }}</h3>
        <form ref="loginForm" class="flex flex-col gap-y-4 mt-4" @submit.prevent>
          <div class="form-control w-full">
            <label class="label">
              <span class="label-text">{{
                $t("install.form.account.label")
              }}</span>
            </label>
            <input
              type="text"
              class="input input-bordered w-full"
              placeholder="xxxx@example.com"
              v-model="loginForm.account"
              required
            />
          </div>
          <div class="form-control w-full">
            <label class="label">
              <span class="label-text">{{
                $t("install.form.password.label")
              }}</span>
            </label>
            <input
              type="password"
              class="input input-bordered w-full"
              v-model="loginForm.password"
              required
            />
          </div>
        </form>
        <div class="modal-action">
          <button class="btn" @click="loginDialogVisible = false">
            {{ $t("install.login_modal.button.close") }}
          </button>
          <button
            class="btn btn-primary"
            @click="onLoginSubmit"
            :disabled="loginLoading"
          >
            <span class="loading loading-spinner" v-show="loginLoading"></span>
            {{ $t("install.login_modal.button.login") }}
          </button>
        </div>
      </div>
    </dialog>

    <!-- Auth Modal (2FA) -->
    <dialog
      ref="authModal"
      :class="['modal', { 'modal-open': authDialogVisible }]"
    >
      <div class="modal-box">
        <h3 class="font-bold text-lg">
          {{ $t("install.dialog.input_pin.title") }}
        </h3>
        <form ref="authForm" @submit.prevent>
          <p class="py-4">
            <input
              type="text"
              class="input input-bordered input-primary w-full"
              :placeholder="$t('install.dialog.input_pin.input.placeholder')"
              v-model="authForm.authcode"
              required
            />
          </p>
        </form>
        <div class="modal-action">
          <button class="btn" @click="authDialogVisible = false">
            {{ $t("install.login_modal.button.close") }}
          </button>
          <button
            class="btn btn-primary"
            @click="onSubmit2FA"
            :disabled="authLoading"
          >
            <span class="loading loading-spinner" v-show="authLoading"></span>
            {{ $t("install.dialog.input_pin.button.submit") }}
          </button>
        </div>
      </div>
    </dialog>
  </div>
</template>

<script>
import { toast } from "vue3-toastify";

export default {
    name: "Login",
    data() {
        return {
            loginWebsock: null,
            loginDialogVisible: false,
            loginLoading: false,
            loginErr: "",
            authDialogVisible: false,
            authLoading: false,
            loginForm: {
                account: "",
                password: "",
            },
            authForm: {
                authcode: "",
            },
        };
    },
    methods: {
        show() {
            this.loginDialogVisible = true;
            this.loginForm.account = "";
            this.loginForm.password = "";
            this.authForm.authcode = "";
            this.loginErr = "";
        },
        validateForm(formRef) {
            const form = this.$refs[formRef];
            if (!form) return false;
            if (!form.checkValidity()) {
                form.reportValidity();
                return false;
            }
            return true;
        },
        onLoginSubmit() {
            if (!this.validateForm("loginForm")) {
                return;
            }
            this.loginLoading = true;
            this.loginErr = "";
            this.initLoginWebSocket();
        },
        initLoginWebSocket() {
            const wsuri =
                (location.protocol === "https:" ? "wss://" : "ws://") +
                location.host +
                "/ws/login";
            this.loginWebsock = new WebSocket(wsuri);
            this.loginWebsock.onopen = this.loginWebsocketonopen;
            this.loginWebsock.onerror = this.loginWebsocketonerror;
            this.loginWebsock.onmessage = this.loginWebsocketonmessage;
            this.loginWebsock.onclose = this.loginWebsocketclose;
        },
        loginWebsocketonopen() {
            console.log("Login WebSocket connect success.");
            this.loginWebsocketsend(1, {
                account: this.loginForm.account,
                password: this.loginForm.password,
            });
        },
        loginWebsocketonmessage(e) {
            let _this = this;
            let line = e.data;

            if (line.indexOf("Enter 2FA code") !== -1) {
                _this.authForm.authcode = "";
                _this.authDialogVisible = true;
                return;
            }

            if (line.indexOf("Successfully logged in") !== -1 || line.indexOf("Login Succeeded") !== -1) {
                _this.loginLoading = false;
                _this.authLoading = false;
                toast.success(_this.$t("install.toast.login_success"));
                _this.loginDialogVisible = false;
                _this.authDialogVisible = false;
                _this.$emit("success");
                _this.loginWebsock.close();
                return;
            }

            if (line.indexOf("Login Failed") !== -1) {
                _this.loginLoading = false;
                _this.authLoading = false;
                toast.error(_this.loginErr);
                return;
            }

            if (line.toLowerCase().indexOf("error") !== -1 || _this.loginErr !== "") {
                _this.loginErr += line;
                return;
            }
        },
        loginWebsocketsend(t, data) {
            let _this = this;
            if (typeof data !== "string") {
                data = JSON.stringify(data);
            }
            const json = JSON.stringify({ t: t, d: data });
            _this.loginWebsock.send(json);
        },
        loginWebsocketclose(e) {
            console.log(`login connection closed (${e.code})`);
            this.loginLoading = false;
            this.authLoading = false;
        },
        loginWebsocketonerror(e) {
            console.log("Login WebSocket connect failed.");
            this.loginLoading = false;
            toast.error("WebSocket connection failed");
        },
        onSubmit2FA() {
            let _this = this;
            if (!_this.validateForm("authForm")) {
                return;
            }
            _this.authLoading = true;
            _this.loginWebsocketsend(2, _this.authForm.authcode);
        },
    }
}
</script>
