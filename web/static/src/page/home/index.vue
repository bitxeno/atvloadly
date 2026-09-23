<template>
  <div class="lg:flex lg:flex-row lg:gap-x-12">
    <div class="lg:basis-3/12 flex gap-x-16 flex-col gap-y-8 mb-8">
      <div v-show="pairableDevices.length > 0">
        <h4 class="mb-2">{{ $t("home.heading.pairable_devices") }}</h4>
        <div class="flex flex-col w-full border-opacity-50">
          <div class="grid card atv-device-panel">
            <a
              class="atv-device-row flex flex-row gap-x-3 cursor-pointer"
              v-for="item in pairableDevices"
              v-bind:key="item.id"
              @click="startPair(item)"
            >
              <div>
                <div class="avatar">
                  <div class="w-16 rounded">
                    <IPhoneIcon v-if="isIPhone(item)" />
                    <AppleTVIcon v-else />
                  </div>
                </div>
              </div>

              <div class="flex flex-col justify-top">
                <h4>{{ item.name }} ({{ truncateIP(item.ip) }})</h4>
                <p>{{ formatDeviceStatus(item) }}</p>
              </div>
            </a>
          </div>
        </div>
      </div>

      <div>
        <h4 class="mb-2">{{ $t("home.heading.paired_devices") }}</h4>
        <div class="flex flex-col w-full border-opacity-50">
          <div
            class="grid card atv-device-panel"
            v-show="pairedDevices.length > 0"
          >
            <a
              class="atv-device-row flex flex-row gap-x-3 cursor-pointer"
              v-for="item in pairedDevices"
              v-bind:key="item.id"
              @click="installIpa(item)"
            >
              <div>
                <div class="avatar online">
                  <div class="w-16 rounded">
                    <IPhoneIcon v-if="item.name.toLowerCase().includes('iphone')" />
                    <AppleTVIcon v-else />
                  </div>
                </div>
              </div>

              <div class="flex flex-col justify-top">
                <h4>{{ item.name }} ({{ truncateIP(item.ip) }})</h4>
                <p>{{ formatDeviceStatus(item) }}</p>
              </div>
            </a>
          </div>

          <div
            class="grid card atv-device-panel h-36 overflow-hidden"
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
            class="grid card atv-device-panel min-h-24 overflow-hidden"
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
      <div class="flex items-center justify-between gap-x-2">
        <h4>{{ $t("home.heading.installed_app") }}</h4>
        <button
          type="button"
          class="btn btn-sm btn-ghost gap-x-2"
          v-if="hasTrackedApps"
          :disabled="checkingUpdates"
          @click="checkUpdates"
        >
          <span class="loading loading-spinner loading-xs" v-if="checkingUpdates"></span>
          <span class="w-4 h-4" v-else><RefreshIcon /></span>
          {{ $t("home.source.check_updates") }}
        </button>
      </div>
      <div class="overflow-x-auto">
        <table class="table table-auto static">
          <!-- head -->
          <thead>
            <tr>
              <th>{{ $t("home.table.header.app") }}</th>
              <th @click="toggleSort('device')" class="cursor-pointer select-none">
                <div class="flex items-center gap-x-1">
                  {{ $t("home.table.header.device") }}
                  <span v-if="sortKey != 'device'" class="text-xs">⇅</span>
                  <span v-if="sortKey == 'device'" class="text-xs">{{ sortOrder == 'asc' ? '▲' : '▼' }}</span>
                </div>
              </th>
              <th @click="toggleSort('account')" class="cursor-pointer select-none">
                <div class="flex items-center gap-x-1">
                  {{ $t("home.table.header.account") }}
                  <span v-if="sortKey != 'account'" class="text-xs">⇅</span>
                  <span v-if="sortKey == 'account'" class="text-xs">{{ sortOrder == 'asc' ? '▲' : '▼' }}</span>
                </div>
              </th>
              <th @click="toggleSort('expired_date')" class="cursor-pointer select-none">
                <div class="flex items-center gap-x-1">
                  {{ $t("home.table.header.expired_date") }}
                  <span v-if="sortKey != 'expired_date'" class="text-xs">⇅</span>
                  <span v-if="sortKey == 'expired_date'" class="text-xs">{{ sortOrder == 'asc' ? '▲' : '▼' }}</span>
                </div>
              </th>
              <th>{{ $t("home.table.header.operate") }}</th>
            </tr>
          </thead>
          <tbody class="bg-base-100">
            <!-- row 1 -->
            <tr class="hover" v-for="item in sortedList" v-bind:key="item.ID">
              <td>
                <div class="flex items-center gap-x-2">
                  <div class="indicator">
                    <span
                      class="indicator-item badge badge-warning"
                      v-show="!item.refreshed_result"
                      >!</span
                    >
                    <div class="inline-flex">
                      <div
                        class="atv-app-icon relative flex items-center justify-center"
                        :class="{ 'atv-app-icon--ios': isIOSApp(item) }"
                      >
                        <img
                          v-if="!failedIcons[item.ID]"
                          :src="iconUrl(item)"
                          :alt="appName(item)"
                          loading="lazy"
                          @error="markIconFailed(item.ID)"
                          class="atv-app-icon-img shadow-sm"
                        />
                        <span
                          v-else
                          class="atv-app-initial"
                          :aria-label="appName(item)"
                        >{{ (appName(item) || "?").charAt(0).toUpperCase() }}</span>
                        <div
                          class="absolute w-full h-full top-0 flex items-center justify-center bg-[#00000066] atv-app-icon-mask"
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
                    <div>{{ appName(item) }}</div>
                    <div class="stat-title text-sm">{{ item.version }}</div>
                    <div class="flex flex-wrap items-center gap-1 atv-source-line">
                      <button
                        type="button"
                        class="atv-source-track"
                        v-if="!item.source.kind"
                        @click="openSourceDialog(item)"
                      >{{ $t("home.source.track") }}</button>
                      <template v-else>
                        <button
                          type="button"
                          class="atv-source-chip"
                          :title="item.source.url"
                          @click="openSourceDialog(item)"
                        >
                          <span class="w-3.5 h-3.5 shrink-0">
                            <GithubIcon v-if="item.source.kind === 'github'" />
                            <LinkIcon v-else />
                          </span>
                          <span class="break-all">{{ item.source.version || "—" }}</span>
                          <span class="badge badge-ghost badge-xs" v-if="item.source.auto_update">{{
                            $t("home.source.auto")
                          }}</span>
                        </button>
                        <div class="tooltip" :data-tip="item.source.check_error" v-if="item.source.check_error">
                          <span class="block w-4 h-4 atv-source-warning"><WarningIcon /></span>
                        </div>
                        <span class="badge badge-info badge-sm" v-if="item.source.latest_build_id">{{
                          $t("home.source.available", { version: item.source.latest_version })
                        }}</span>
                      </template>
                    </div>
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
              <td class="lg:break-all">
                <div class="tooltip" :data-tip="$t('home.table.tips.account_invalid')" v-if="item.refreshed_error == 1">
                  <div class="text-red-500"> {{ item.account }}</div>
                </div>
                <div v-else>
                  {{ item.account }}
                </div>
              </td>
              <td>
                <div :class="expiryHue(item) === null ? 'badge badge-ghost min-w-max' : 'badge atv-expiry min-w-max'"
                  :style="{ '--expiry-hue': expiryHue(item) }">
                  {{ formatExpiredTime(item) }}
                </div>
              </td>
              <td>
                <div class="flex gap-x-2">
                  <button
                    type="button"
                    class="btn atv-action atv-action--update"
                    v-if="item.source.latest_build_id"
                    :disabled="isInstalling(item)"
                    @click="updateApp(item)"
                  >{{ $t("home.table.button.update") }}</button>
                  <button type="button" class="btn atv-action atv-action--refresh" @click="refreshApp(item)">{{
                    $t("home.table.button.refresh")
                  }}</button>
                  <Popper placement="top" arrow="true">
                    <template #content="{ close }">
                      <div class="flex flex-col gap-y-2">
                        <div class="py-2">
                          {{
                            $t("home.dialog.delete_confirm.title", {
                              name: appName(item),
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
                    <button type="button" class="btn atv-action atv-action--danger">{{
                      $t("home.table.button.delete")
                    }}</button>
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
        class="stat-title text-sm whitespace-normal atv-refresh-footer-note"
      >
        {{ $t("home.table.tips.footer") }}
      </div>
    </div>

    <SourceDialog
      ref="sourceDialog"
      @saved="fetchAppList"
      @update-started="onUpdateStarted"
    />
  </div>
</template>
  

<script>
import dayjs from "dayjs";
import api from "@/api/api";
import { toast } from "vue3-toastify";
import { truncateIP } from "@/utils/utils";
import SourceDialog from "@/components/SourceDialog.vue";

export default {
  name: "Home",
  components: { SourceDialog },
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
      newInstallToastId: null,
      sortKey: "",
      sortOrder: "asc",
      failedIcons: {},
      checkingUpdates: false,
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
    hasTrackedApps: function () {
      return this.list.some((item) => item.source.kind);
    },
    sortedList: function () {
      let list = this.list.slice();
      if (this.sortKey) {
        let order = this.sortOrder === "asc" ? 1 : -1;
        list.sort((a, b) => {
          let valA = this.getSortValue(a, this.sortKey);
          let valB = this.getSortValue(b, this.sortKey);
          if (valA < valB) return -1 * order;
          if (valA > valB) return 1 * order;
          return 0;
        });
      }
      return list;
    },
  },
  created() {
    this.fetchData();
  },
  unmounted() {
    this.checkInstallingTimer && clearTimeout(this.checkInstallingTimer);
    if (this.newInstallToastId) {
      toast.update(this.newInstallToastId, {
        type: "success",
        isLoading: false,
        autoClose: 3000,
      });
      this.newInstallToastId = null;
    }
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

          if (_this.newInstallToastId) {
            toast.remove(_this.newInstallToastId);
            _this.newInstallToastId = null;
          }
        }

        _this.installingApps = res.data || [];

        // Show persistent toast for new installs triggered via REST API (ID == 0)
        let newInstall = (_this.installingApps || []).find(a => a.ID == 0);
        if (newInstall) {
          if (!_this.newInstallToastId) {
            _this.newInstallToastId = toast.loading(
              _this.$t("home.toast.installing_app", { name: newInstall.ipa_name }),
              { autoClose: false }
            );
          }
        }

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
    updateApp(item) {
      api.updateAppFromSource(item.ID, {}).then((res) => {
        this.onUpdateStarted(item, res.data);
      });
    },
    onUpdateStarted(item, data) {
      this.installingApps.push(item);
      this.checkInstallingAppDelay();
      toast.info(
        this.$t("home.toast.update_started", {
          name: this.appName(item),
          version: data.version,
        })
      );
    },
    openSourceDialog(item) {
      this.$refs.sourceDialog.show(item);
    },
    checkUpdates() {
      this.checkingUpdates = true;
      api
        .checkSourceUpdates()
        .then((res) => {
          this.list = res.data || [];
          const tracked = this.list.filter((item) => item.source.kind);
          const updates = tracked.filter((item) => item.source.latest_build_id).length;
          const errors = tracked.filter((item) => item.source.check_error).length;
          if (updates > 0) {
            toast.info(this.$t("home.toast.updates_available", { count: updates }));
          } else if (errors > 0) {
            toast.warning(this.$t("home.toast.check_errors", { count: errors }));
          } else {
            toast.success(this.$t("home.toast.check_done"));
          }
        })
        .catch((err) => {
          // request.js already shows the error.
          console.error(err);
        })
        .finally(() => {
          this.checkingUpdates = false;
        });
    },
    startPair(device) {
      this.$router.push({ name: "pair", params: { id: device.id } });
    },
    installIpa(device) {
      this.$router.push({ name: "install", params: { id: device.id } });
    },
    appName(item) {
      return item.custom_name || item.ipa_name;
    },
    expiryHue(item) {
      const date = item.expiration_date || item.refreshed_date || item.installed_date;
      if (!date) return null;
      const expires = item.expiration_date ? dayjs(date) : dayjs(date).add(7, "day");
      if (!expires.isValid()) return null;

      const remainingMs = expires.diff(dayjs());
      const sevenDaysMs = 7 * 24 * 60 * 60 * 1000;
      const fraction = Math.max(0, Math.min(1, remainingMs / sevenDaysMs));
      return Math.round(fraction * 120);
    },
    formatExpiredTime(item) {
      let time = item.refreshed_date || item.installed_date;
      if (!time && !item.expiration_date) return "-";

      let expired_date = time ? dayjs(time).add(7, "day") : null;
      if (item.expiration_date) {
        expired_date = dayjs(item.expiration_date);
      }

      // If it has already expired
      if (expired_date.diff(dayjs()) <= 0) {
        return this.$t("home.table.expired_time_format.expired_tips");
      }

      // Display by day priority, show days for durations of 1 day or more
      let days = expired_date.diff(dayjs(), "day");
      if (days >= 1) {
        return this.$t("home.table.expired_time_format.days", {
          num: days,
          count: days,
        });
      }

      // Display hours when less than 1 day
      let hours = expired_date.diff(dayjs(), "hour");
      if (hours >= 1) {
        return this.$t("home.table.expired_time_format.hours", {
          num: hours,
          count: hours,
        });
      }

      // Less than 1 hour
      return this.$t("home.table.expired_time_format.less_than_hour");
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
    formatConnection(connection) {
      return connection;
    },
    formatDeviceStatus(item) {
      const status = this.formatStatus(item.status);
      const connection = this.formatConnection(item.connection);

      if (!connection) {
        return status;
      }

      return `${status} · ${connection}`;
    },
    isIPhone(item) {
      if (item.device_class) {
        return item.device_class.toLowerCase() == "iphone" || item.device_class.toLowerCase() == "ipad";
      }
      return item.name && (item.name.toLowerCase().includes("iphone") || item.name.toLowerCase().includes("ipad"));
    },
    isIOSApp(item) {
      if (!item || !item.device_class) return false;
      const deviceClass = item.device_class.toLowerCase();
      return deviceClass === "iphone" || deviceClass === "ipad";
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
          count: seconds,
        });
      }
      let miniutes = parseInt(seconds / 60, 10);
      if (miniutes < 60) {
        return _this.$t("home.table.refresh_date_format.miniutes", {
          num: miniutes,
          count: miniutes,
        });
      }
      let hours = parseInt(seconds / 3600, 10);
      if (hours < 24) {
        return _this.$t("home.table.refresh_date_format.hours", {
          num: hours,
          count: hours,
        });
      }
      let days = parseInt(seconds / 24 / 3600, 10);
      return _this.$t("home.table.refresh_date_format.days", {
        num: days,
        count: days,
      });
    },
    formatRefreshResult(item) {
      if (!item.refreshed_date) return "";

      return item.refreshed_result
        ? this.$t("result.success")
        : this.$t("result.fail");
    },
    getSortValue(item, key) {
      if (key === "device") {
        for (let i = 0; i < this.devices.length; i++) {
          const dev = this.devices[i];
          if (dev.udid == item.udid && dev.status == "paired") {
            return dev.name;
          }
        }
        return "";
      } else if (key === "account") {
        return item.account || "";
      } else if (key === "expired_date") {
        // Compute expiration timestamp for sorting.
        // Prefer explicit expiration_date; otherwise use refreshed_date/installed_date + 7 days.
        let time = item.refreshed_date || item.installed_date;
        if (item.expiration_date) {
          return dayjs(item.expiration_date).valueOf();
        }
        if (time) {
          return dayjs(time).add(7, "day").valueOf();
        }
        return 0;
      }
      return "";
    },
    toggleSort(key) {
      if (this.sortKey === key) {
        this.sortOrder = this.sortOrder === "asc" ? "desc" : "asc";
      } else {
        this.sortKey = key;
        this.sortOrder = "asc";
      }
    },
    formatDeviceName(item) {
      let _this = this;
      for (let i = 0; i < _this.devices.length; i++) {
        const dev = _this.devices[i];
        if (dev.udid == item.udid && dev.status == "paired") {
          return `${dev.name}<br/>(${truncateIP(dev.ip)})`;
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
    markIconFailed(id) {
      this.failedIcons[id] = true;
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
import IPhoneIcon from "@/assets/icons/iphone.svg";
import HelpIcon from "@/assets/icons/help.svg";
import CheckMarkIcon from "@/assets/icons/checkmark.svg";
import DismissIcon from "@/assets/icons/dismiss.svg";
import GithubIcon from "@/assets/icons/github.svg";
import LinkIcon from "@/assets/icons/link.svg";
import RefreshIcon from "@/assets/icons/refresh.svg";
import WarningIcon from "@/assets/icons/warning.svg";
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

.atv-source-track {
  color: var(--atv-muted);
  font-size: 0.8rem;
}

.atv-source-track:hover,
.atv-source-track:focus-visible {
  color: var(--atv-accent);
  text-decoration: underline;
  text-underline-offset: 3px;
}

.atv-source-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 22px;
  padding: 1px 8px;
  border: 1px solid var(--atv-border);
  border-radius: 999px;
  background: var(--atv-surface-alt);
  color: var(--atv-muted);
  font-size: 0.75rem;
  font-weight: 650;
  transition: border-color 150ms ease, color 150ms ease;
}

.atv-source-chip:hover,
.atv-source-chip:focus-visible {
  border-color: var(--atv-accent);
  color: var(--atv-accent);
}

.atv-source-warning {
  color: var(--atv-warning-text);
}

.atv-source-line .badge-info {
  color: var(--atv-success-text);
  background: var(--atv-success-soft);
  border-color: var(--atv-success-border);
}

:deep(.popper) {
  background: var(--atv-surface);
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--atv-border);
  box-shadow: var(--atv-menu-shadow);
  color: var(--atv-ink);
  word-break: break-all;
  text-align: justify;
  min-width: 150px;
}

:deep(.popper:hover),
:deep(.popper:hover > #arrow::before) {
  background: var(--atv-surface);
}

:deep(.popper #arrow::before) {
  background: var(--atv-surface);
}
</style>