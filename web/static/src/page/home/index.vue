<template>
  <div class="lg:flex lg:flex-row lg:gap-x-12">
    <div class="lg:basis-3/12 flex gap-x-16 flex-col gap-y-8 mb-8">
      <div v-show="pairableDevices.length > 0">
        <h4 class="mb-2">{{ $t("home.heading.pairable_devices") }}</h4>
        <div class="flex flex-col w-full border-opacity-50">
          <div class="grid card bg-base-300 rounded-box p-4">
            <a
              class="flex flex-row gap-x-2 cursor-pointer"
              v-for="item in pairableDevices"
              v-bind:key="item.id"
              @click="startPair(item)"
            >
              <div>
                <div class="avatar">
                  <div class="w-16 rounded">
                    <AppleTVIcon />
                  </div>
                </div>
              </div>

              <div class="flex flex-col justify-top">
                <h4>{{ item.name }} ({{ item.ip }})</h4>
                <p>{{ formatStatus(item.status) }}</p>
              </div>
            </a>
          </div>
        </div>
      </div>

      <div>
        <h4 class="mb-2">{{ $t("home.heading.paired_devices") }}</h4>
        <div class="flex flex-col w-full border-opacity-50">
          <div
            class="grid card bg-base-300 rounded-box p-4"
            v-show="pairedDevices.length > 0"
          >
            <a
              class="flex flex-row gap-x-2 cursor-pointer"
              v-for="item in pairedDevices"
              v-bind:key="item.id"
              @click="installIpa(item)"
            >
              <div>
                <div class="avatar online">
                  <div class="w-16 rounded">
                    <AppleTVIcon />
                  </div>
                </div>
              </div>

              <div class="flex flex-col justify-top">
                <h4>{{ item.name }} ({{ item.ip }})</h4>
                <p>{{ formatStatus(item.status) }}</p>
              </div>
            </a>
          </div>

          <div
            class="grid card bg-base-300 rounded-box p-4 h-36 overflow-hidden"
            v-show="pairedDevices.length == 0"
          >
            <h4 class="flex justify-center items-center">
              {{ $t("home.sidebar.tips.no_paired_devices") }}
            </h4>

            <div class="stat-title whitespace-normal text-sm content-center">
              {{ $t("home.sidebar.tips.how_pair_device") }}
            </div>
          </div>
        </div>
      </div>

      <div>
        <h4 class="mb-2">{{ $t("home.heading.service_status") }}</h4>
        <div class="flex flex-col w-full border-opacity-50">
          <div
            class="grid card bg-base-300 rounded-box p-4 h-32 overflow-hidden"
          >
            <ui class="flex flex-col gap-y-2">
              <li
                class="flex items-center gap-x-1"
                v-for="item in services"
                v-bind:key="item.name"
              >
                <div class="w-6 text-green-500" v-show="item.running">
                  <CheckMarkIcon />
                </div>
                <div class="w-6 text-red-500" v-show="!item.running">
                  <DismissIcon />
                </div>
                {{ item.name }}
              </li>
            </ui>
          </div>
        </div>
      </div>
    </div>

    <div class="lg:basis-9/12 flex flex-col gap-y-2">
      <h4>{{ $t("home.heading.installed_app") }}</h4>
      <div class="overflow-x-auto">
        <table class="table table-auto static">
          <!-- head -->
          <thead>
            <tr>
              <th>{{ $t("home.table.header.app") }}</th>
              <th>{{ $t("home.table.header.device") }}</th>
              <th>{{ $t("home.table.header.account") }}</th>
              <th>{{ $t("home.table.header.expired_date") }}</th>
              <th>{{ $t("home.table.header.operate") }}</th>
            </tr>
          </thead>
          <tbody class="bg-base-100">
            <!-- row 1 -->
            <tr class="hover" v-for="item in list" v-bind:key="item.ID">
              <td>
                <div class="flex items-center gap-x-2">
                  <div class="indicator">
                    <span
                      class="indicator-item badge badge-warning"
                      v-show="!item.refreshed_result"
                      >!</span
                    >
                    <div class="inline-flex">
                      <div class="w-32 rounded relative">
                        <img :src="iconUrl(item)" class="rounded-md" />
                        <div
                          class="absolute w-full h-full top-0 flex items-center justify-center bg-[#00000066]"
                          v-show="isInstalling(item)"
                        >
                          <span
                            class="loading loading-spinner loading-lg text-slate-100"
                          ></span>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div class="flex flex-col justify-start prose">
                    <div>{{ item.ipa_name }}</div>
                    <div class="stat-title text-sm">{{ item.version }}</div>
                    <div class="stat-title text-sm">
                      <a
                        class="link link-hover stat-title font-normal"
                        :href="logUrl(item)"
                        :title="$t('home.table.tips.view_log')"
                        target="_blank"
                        >{{ formatRefreshDate(item) }}</a
                      >
                    </div>
                  </div>
                </div>
              </td>
              <td class="lg:break-all" v-html="formatDeviceName(item)">
              </td>
              <td class="lg:break-all">{{ item.account }}</td>
              <td>
                <div class="badge badge-ghost min-w-max">
                  {{ formatExpiredTime(item) }}
                </div>
              </td>
              <td>
                <div class="flex gap-x-2">
                  <a class="link link-primary" @click="refreshApp(item)">{{
                    $t("home.table.button.refresh")
                  }}</a>
                  <Popper placement="top" arrow="true">
                    <template #content="{ close }">
                      <div class="flex flex-col gap-y-2">
                        <div class="py-2">
                          {{
                            $t("home.dialog.delete_confirm.title", {
                              name: item.ipa_name,
                            })
                          }}
                        </div>
                        <div class="flex gap-x-2 justify-end items-center">
                          <a
                            class="link link-primary link-hover"
                            @click="close"
                            >{{
                              $t("home.dialog.delete_confirm.button.cancel")
                            }}</a
                          >
                          <button
                            class="btn btn-primary btn-xs"
                            @click="deleteApp(item, close)"
                          >
                            {{
                              $t("home.dialog.delete_confirm.button.confirm")
                            }}
                          </button>
                        </div>
                      </div>
                    </template>
                    <a class="link link-error">{{
                      $t("home.table.button.delete")
                    }}</a>
                  </Popper>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div class="empty" v-show="list.length == 0">
          {{ $t("home.table.tips.no_data") }}
        </div>
      </div>

      <div
        class="stat-title text-sm flex flex-row items-center gap-x-1 whitespace-break-spaces"
      >
        <div class="w-4"><HelpIcon /></div>
        {{ $t("home.table.tips.footer") }}
      </div>
    </div>
  </div>
</template>
  

<script>
import dayjs from "dayjs";
import api from "@/api/api";
import { toast } from "vue3-toastify";

export default {
  name: "Home",
  data() {
    return {
      devices: [],
      list: [
        // {
        //   ID: 1,
        //   ipa_name: "AppleTV",
        //   version: "1.0.0",
        //   account: "admin",
        //   refreshed_date: "2023-06-30 16:10:10",
        //   refreshed_result: true,
        // },
        // {
        //   ID: 2,
        //   ipa_name: "AppleTV",
        //   version: "1.0.0",
        //   account: "admin",
        //   refreshed_date: "2023-06-10 10:10:10",
        // },
      ],
      services: [],
      installingApps: [],
      checkInstallingTimer: null,
    };
  },
  computed: {
    pairableDevices: function () {
      return this.devices.filter(function (item) {
        return item.status == "pairable";
      });
    },
    pairedDevices: function () {
      return this.devices.filter(function (item) {
        return item.status == "paired";
      });
    },
  },
  created() {
    this.fetchData();
  },
  unmounted() {
    this.checkInstallingTimer && clearTimeout(this.checkInstallingTimer);
  },
  methods: {
    fetchData() {
      let _this = this;

      api.getDevices().then((res) => {
        _this.devices = res.data;
      });

      api.getServiceStatus().then((res) => {
        _this.services = res.data;
      });

      _this.checkInstallingApp();
      _this.fetchAppList();
    },
    fetchAppList() {
      let _this = this;

      api.getAppList().then((res) => {
        _this.list = res.data;
      });
    },
    checkInstallingApp() {
      let _this = this;

      api.getInstallingApps().then((res) => {
        // res.data returns empty, indicating that the installation has been completed.
        if (_this.installingApps && (res.data || []).length == 0) {
          _this.fetchAppList();
        }

        _this.installingApps = res.data || [];
        if (_this.installingApps && _this.installingApps.length > 0) {
          // Repeat the detection until it is completed.
          _this.checkInstallingAppDelay();
        }
      });
    },
    checkInstallingAppDelay() {
      let _this = this;

      if (this.checkInstallingTimer) {
        clearTimeout(this.checkInstallingTimer);
      }

      this.checkInstallingTimer = setTimeout(function () {
        _this.checkInstallingApp();
      }, 10 * 1000);
    },
    deleteApp(item, closePopper) {
      let _this = this;

      api.deleteApp(item.ID).then((res) => {
        _this.fetchData();
      });

      closePopper?.();
    },
    refreshApp(item) {
      let _this = this;

      _this.installingApps.push(item);

      api.refreshApp(item.ID).then((res) => {
        _this.checkInstallingAppDelay();
        toast.info(
          this.$t("home.toast.refresh_app_started", {
            name: item.ipa_name,
          })
        );
      });
    },
    startPair(device) {
      this.$router.push({ name: "pair", params: { id: device.id } });
    },
    installIpa(device) {
      this.$router.push({ name: "install", params: { id: device.id } });
    },
    formatExpiredTime(item) {
      let time = item.refreshed_date || item.installed_date;
      if (!time) return "-";

      let diff = dayjs(time).add(7, "day").diff(dayjs(), "day");
      if (diff < 0)
        return this.$t("home.table.expired_time_format.expired_tips");

      return this.$t("home.table.expired_time_format.days", {
        num: diff,
      });
    },
    formatStatus(status) {
      if (status == "paired") {
        return this.$t("home.sidebar.device_status.paired");
      } else if (status == "pairable") {
        return this.$t("home.sidebar.device_status.pairable");
      } else {
        return this.$t("home.sidebar.device_status.unpaired");
      }
    },
    formatRefreshDate(item) {
      let _this = this;
      if (_this.isInstalling(item)) {
        return _this.$t("home.table.refresh_date_format.installing_tips");
      }

      if (!item.refreshed_date) return "-";

      let seconds = dayjs().diff(dayjs(item.refreshed_date), "second");
      if (seconds < 60) {
        return _this.$t("home.table.refresh_date_format.seconds", {
          num: seconds,
        });
      }
      let miniutes = parseInt(seconds / 60, 10);
      if (miniutes < 60) {
        return _this.$t("home.table.refresh_date_format.miniutes", {
          num: miniutes,
        });
      }
      let hours = parseInt(seconds / 3600, 10);
      if (hours < 24) {
        return _this.$t("home.table.refresh_date_format.hours", {
          num: hours,
        });
      }
      let days = parseInt(seconds / 24 / 3600, 10);
      return _this.$t("home.table.refresh_date_format.days", {
        num: days,
      });
    },
    formatRefreshResult(item) {
      if (!item.refreshed_date) return "";

      return item.refreshed_result
        ? this.$t("result.success")
        : this.$t("result.fail");
    },
    formatDeviceName(item) {
      let _this = this;
      for (let i = 0; i < _this.devices.length; i++) {
        const dev = _this.devices[i];
        if (dev.udid == item.udid && dev.status == "paired") {
          return `${dev.name}<br/>(${dev.ip})`;
        }
      }
      return this.$t("home.sidebar.device_status.unpaired");
    },
    isInstalling(item) {
      if (!this.installingApps || this.installingApps.length == 0) return false;

      for (let i = 0; i < this.installingApps.length; i++) {
        if( this.installingApps[i].ID == item.ID) {
          return true;
        }
      }
      return false;
    },
    iconUrl(app) {
      if (app.icon) {
        return `/apps/${app.ID}/icon`;
      } else {
        return "/img/dummy.jpg";
      }
    },
    logUrl(item) {
      return `/apps/${item.ID}/log`;
    },
  },
};
</script>

<script setup>
import AppleTVIcon from "@/assets/icons/appletv.svg";
import HelpIcon from "@/assets/icons/help.svg";
import CheckMarkIcon from "@/assets/icons/checkmark.svg";
import DismissIcon from "@/assets/icons/dismiss.svg";
</script>

  
<style lang="postcss" scoped>
.headline {
  @apply prose mb-2;
}
.empty {
  background-image: repeating-linear-gradient(
    45deg,
    hsl(var(--b1)),
    hsl(var(--b1)) 13px,
    hsl(var(--b2)) 13px,
    hsl(var(--b2)) 14px
  );
  @apply border-base-300 bg-base-100 rounded-b-box flex min-h-[6rem]  flex-wrap items-center justify-center gap-2 overflow-x-hidden border bg-cover bg-top p-4;
}

:deep(.popper) {
  background: #ffffff;
  padding: 12px;
  border-radius: 4px;
  border: 1px solid #ebeef5;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  word-break: break-all;
  text-align: justify;
  min-width: 150px;
}

:deep(.popper:hover),
:deep(.popper:hover > #arrow::before) {
  background: #ffffff;
}

:deep(.popper #arrow::before) {
  background: #ffffff;
}
</style>