import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import "./app.css";
import Vue3Toasity from "vue3-toastify";
import "vue3-toastify/dist/index.css";
import Popper from "vue3-popper";

createApp(App)
  .use(router)
  .use(Vue3Toasity, {
    autoClose: 3000,
    hideProgressBar: true,
    position: "top-center",
    theme: "colored",
  })
  .component("Popper", Popper)
  .mount("#app");
