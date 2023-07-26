import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import Vue3Toasity from "vue3-toastify";
import Popper from "vue3-popper";
import I18NextVue from "i18next-vue";
import i18next from "./i18n";
import "./app.css";
import "vue3-toastify/dist/index.css";

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
