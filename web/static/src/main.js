import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import Vue3Toasity, { toast } from "vue3-toastify";
import Popper from "vue3-popper";
import I18NextVue from "i18next-vue";
import i18next from "./i18n";
import "./app.css";
import "./mobile.css";
import "vue3-toastify/dist/index.css";

// Error toasts live longer so they can be read; success and other toasts keep
// the global 3s. An explicit autoClose in a call still wins.
const ERROR_AUTOCLOSE = 6000;
const baseError = toast.error.bind(toast);
toast.error = (message, options) =>
  baseError(message, { autoClose: ERROR_AUTOCLOSE, ...(options || {}) });

createApp(App)
  .use(router)
  .use(I18NextVue, { i18next })
  .use(Vue3Toasity, {
    autoClose: 3000,
    hideProgressBar: true,
    position: "top-center",
    theme: "colored",
  })
  .component("Popper", Popper)
  .mount("#app");
