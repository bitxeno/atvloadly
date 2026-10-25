<template>
  <div class="xl:flex xl:flex-row xl:gap-x-12">
    <div class="xl:basis-3/12 min-w-0 flex gap-x-16 flex-col gap-y-8 mb-8 xl:mb-0">
      <div class="grid card atv-device-panel">
        <router-link
          :to="{ name: 'add-device' }"
          class="atv-device-row flex flex-row gap-x-3 cursor-pointer"
        >
          <div class="atv-add-device-icon">
            <PlusIcon />
          </div>
          <div class="atv-add-device-text">
            <h4>{{ $t("home.sidebar.add_device.title") }}</h4>
            <p>{{ $t("home.sidebar.add_device.subtitle") }}</p>
          </div>
          <span class="atv-add-device-chevron">
            <ChevronRightIcon />
          </span>
        </router-link>
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
              :class="{ 'atv-device-row--manual': item.manual }"
              @click="installIpa(item)"
              @mouseleave="forgetConfirmId = null"
            >
              <div>
                <div class="avatar" :class="{ online: !item.manual }">
                  <span v-if="item.manual" class="atv-ip-badge">{{ $t("home.sidebar.device_status.ip_badge") }}</span>
                  <div class="w-16 rounded">
                    <IPhoneIcon v-if="item.name.toLowerCase().includes('iphone')" />
                    <AppleTVIcon v-else />
                  </div>
                </div>
              </div>

              <div class="flex flex-col justify-top flex-1 min-w-0">
                <h4>{{ item.name }} ({{ truncateIP(item.ip) }})</h4>
                <p class="atv-device-subline">
                  <span>{{ formatDeviceStatus(item) }}</span>
                </p>
              </div>

              <button
                v-if="item.manual"
                type="button"
                class="atv-device-forget"
                :class="{ 'atv-device-forget--armed': forgetConfirmId === item.id }"
                :title="forgetConfirmId === item.id ? $t('home.sidebar.device_status.forget_confirm') : $t('home.sidebar.device_status.forget')"
                :aria-label="$t('home.sidebar.device_status.forget')"
                @click.stop.prevent="forgetDevice(item)"
              >
                <span class="atv-device-forget-icon"><TrashIcon /></span>
                <span
                  class="atv-device-forget-label"
                  v-if="forgetConfirmId === item.id"
                >{{ $t("home.sidebar.device_status.forget_confirm") }}</span>
              </button>
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

      <div class="atv-service-status">
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

    <div class="xl:basis-9/12 min-w-0 flex-1 flex flex-col gap-y-2">
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
      <div class="overflow-x-auto min-w-0 max-w-full">
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
            <tr class="hover" v-for="(item, index) in sortedList" v-bind:key="item.ID">
              <td>
                <div class="flex items-center gap-x-2 atv-app-cell">
                  <div class="indicator">
                    <div
                      class="tooltip tooltip-right indicator-item atv-indicator-tip"
                      :data-tip="indicatorTip(item)"
                      v-if="indicatorTip(item)"
                    >
                      <span class="badge badge-warning">!</span>
                    </div>
                    <span
                      v-else-if="!item.refreshed_result || sourceWarning(item)"
                      class="indicator-item badge badge-warning"
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

                  <div class="flex flex-col justify-start prose atv-app-text">
                    <div>{{ appName(item) }}</div>
                    <div class="stat-title text-sm">{{ item.version }}</div>
                    <!-- The same IPA installed with and without a custom bundle
                         identifier is two different apps on the device: show the
                         identifier so the two rows can be told apart. -->
                    <div
                      v-if="item.custom_identifier"
                      class="stat-title text-xs truncate"
                      :title="item.custom_identifier"
                    >
                      {{ item.custom_identifier }}
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
                    <div class="flex flex-wrap items-center gap-1 atv-source-line">
                      <span class="badge badge-info badge-sm max-w-full" v-if="item.source.latest_build_id">
                        <span class="truncate">{{
                          $t("home.source.available", { version: item.source.latest_version })
                        }}</span>
                      </span>
                      <button
                        v-else-if="item.source.kind"
                        type="button"
                        class="atv-source-chip max-w-full"
                        :title="item.source.url"
                        @click="openSourceDialog(item)"
                      >
                        <span class="w-3.5 h-3.5 shrink-0">
                          <GithubIcon v-if="item.source.kind === 'github'" />
                          <LinkIcon v-else />
                        </span>
                        <span class="truncate">{{ item.source.version || "—" }}</span>
                        <span class="badge badge-ghost badge-xs shrink-0" v-if="item.source.auto_update">{{
                          $t("home.source.auto")
                        }}</span>
                      </button>
                    </div>
                  </div>
                </div>
              </td>
              <td class="lg:break-all" v-html="formatDeviceName(item)">
              </td>
              <td class="lg:break-all">
                <div v-if="isExternalApp(item)" class="flex flex-col items-start gap-y-1">
                  <span class="badge atv-badge-external min-w-max">{{
                    $t("home.table.signing.external_badge")
                  }}</span>
                  <div
                    class="tooltip"
                    :data-tip="externalErrorText(item)"
                    v-if="externalErrorText(item)"
                  >
                    <div class="text-red-500">{{ identityName(item) }}</div>
                  </div>
                  <div v-else>{{ identityName(item) }}</div>
                </div>
                <template v-else>
                  <div class="tooltip" :data-tip="$t('home.table.tips.account_invalid')" v-if="item.refreshed_error == 1">
                    <div class="text-red-500"> {{ item.account }}</div>
                  </div>
                  <div v-else>
                    {{ item.account }}
                  </div>
                </template>
              </td>
              <td>
                <div :class="expiryHue(item) === null ? 'badge badge-ghost min-w-max' : 'badge atv-expiry min-w-max'"
                  :style="{ '--expiry-hue': expiryHue(item) }">
                  {{ formatExpiredTime(item) }}
                </div>
                <div
                  v-if="isExternalApp(item) && item.expiration_date"
                  class="stat-title text-xs mt-1 whitespace-nowrap"
                  :title="$t('home.table.tips.reinstall_external')"
                >
                  {{ $t("home.table.signing.valid_until", { date: formatDate(item.expiration_date) }) }}
                </div>
              </td>
              <td>
                <div class="flex gap-x-2 atv-app-actions">
                  <button
                    type="button"
                    class="btn atv-action atv-action--update"
                    v-if="item.source.latest_build_id"
                    :disabled="isInstalling(item)"
                    @click="updateApp(item)"
                  >{{ $t("home.table.button.update") }}</button>
                  <button
                    type="button"
                    class="btn atv-action atv-action--refresh"
                    v-if="!item.source.latest_build_id"
                    :title="isExternalApp(item) ? $t('home.table.tips.reinstall_external') : undefined"
                    @click="refreshApp(item)"
                  >{{
                    isExternalApp(item)
                      ? $t("home.table.button.reinstall")
                      : $t("home.table.button.refresh")
                  }}</button>
                  <div
                    class="dropdown dropdown-hover dropdown-end"
                    :class="{ 'dropdown-top': index === sortedList.length - 1 }"
                    @mouseleave="resetConfirmDelete"
                  >
                    <div
                      tabindex="0"
                      role="button"
                      class="btn atv-action"
                    >{{ $t("home.table.button.more") }}</div>
                    <ul
                      tabindex="0"
                      class="dropdown-content z-[1] menu p-2 shadow bg-base-100 rounded-box w-48 gap-1"
                    >
                      <li>
                        <a @click="openSourceDialog(item)">
                          <span class="w-4 h-4 shrink-0">
                            <GithubIcon v-if="item.source.kind === 'github'" />
                            <LinkIcon v-else />
                          </span>
                          {{
                            item.source.kind
                              ? $t("home.source.manage")
                              : $t("home.source.track")
                          }}
                        </a>
                      </li>
                      <li v-if="item.source.latest_build_id">
                        <a
                          :title="isExternalApp(item) ? $t('home.table.tips.reinstall_external') : undefined"
                          @click="refreshApp(item)"
                        >
                          <span class="w-4 h-4 shrink-0">
                            <RefreshIcon />
                          </span>
                          {{
                            isExternalApp(item)
                              ? $t("home.table.button.reinstall")
                              : $t("home.table.button.refresh")
                          }}
                        </a>
                      </li>
                      <li v-if="!isExternalApp(item)">
                        <a @click="reinstallApp(item)">
                          <span class="w-4 h-4 shrink-0">
                            <DownloadIcon />
                          </span>
                          {{ $t("home.table.button.reinstall") }}
                        </a>
                      </li>
                      <li v-if="confirmDeleteId !== item.ID">
                        <a class="atv-menu-danger" @click="confirmDelete(item)">
                          <span class="w-4 h-4 shrink-0">
                            <DismissIcon />
                          </span>
                          {{ $t("home.table.button.delete") }}
                        </a>
                      </li>
                      <li v-else>
                        <a class="atv-menu-danger" @click="deleteApp(item)">
                          <span class="w-4 h-4 shrink-0">
                            <DismissIcon />
                          </span>
                          {{ $t("home.table.button.confirm_delete") }}
                        </a>
                      </li>
                    </ul>
                  </div>
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
      <div
        v-if="hasExternalApps"
        class="stat-title text-sm whitespace-normal atv-refresh-footer-note"
      >
        {{ $t("home.table.tips.footer_external") }}
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
import { updateFailed } from "@/utils/source.mjs";

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
      identities: null,
      checkingUpdates: false,
      // IDs of apps whose update request is pending. The installing list
      // polled from the server does not include them yet.
      pendingUpdates: {},
      // ID of the app armed for deletion in the row dropdown menu. Clicking
      // delete once arms it, clicking again confirms.
      confirmDeleteId: null,
      // ID of a manual device armed for "forget" confirmation.
      forgetConfirmId: null,
    };
  },
  computed: {
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
    hasExternalApps: function () {
      return this.list.some((item) => this.isExternalApp(item));
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

      api.getSigningIdentities().then((res) => {
        _this.identities = res.data || [];
      }).catch(() => {
        // Unknown identities: an app must not be labelled with a deleted identity.
        _this.identities = null;
      });
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
    confirmDelete(item) {
      this.confirmDeleteId = item.ID;
    },
    resetConfirmDelete() {
      this.confirmDeleteId = null;
    },
    deleteApp(item, closePopper) {
      let _this = this;

      _this.confirmDeleteId = null;
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
        if (_this.isExternalApp(item)) {
          toast.info(
            this.$t("home.toast.reinstall_app_started", {
              name: item.ipa_name,
            })
          );
          return;
        }
        toast.info(
          this.$t("home.toast.refresh_app_started", {
            name: item.ipa_name,
          })
        );
      });
    },
    reinstallApp(item) {
      let _this = this;

      _this.installingApps.push(item);

      api.reinstallApp(item.ID).then((res) => {
        _this.checkInstallingAppDelay();
        toast.info(
          this.$t("home.toast.reinstall_app_started", {
            name: item.ipa_name,
          })
        );
      });
    },
    isExternalApp(item) {
      return item.signing_mode === "external_certificate";
    },
    identityName(item) {
      if (!this.identities) {
        // Neutral placeholder while the identities are loading or unavailable.
        return "—";
      }
      const identity = this.identities.find((entry) => entry.id === item.signing_identity_id);
      return identity ? identity.name : this.$t("home.table.signing.identity_missing");
    },
    // externalErrorText explains the last failure of an external-certificate app.
    externalErrorText(item) {
      switch (item.refreshed_error) {
        case 2:
          return this.$t("home.table.tips.signing_identity_error");
        case 3:
          return this.$t("home.table.tips.signing_error");
        case 4:
          return this.$t("home.table.tips.transport_error");
        default:
          return "";
      }
    },
    formatDate(value) {
      return dayjs(value).format("YYYY-MM-DD");
    },
    updateApp(item) {
      // The request fetches the source before it queues the update: the app
      // shows as installing meanwhile, so another click cannot queue it twice.
      this.pendingUpdates[item.ID] = true;
      api
        .updateAppFromSource(item.ID, {})
        .then((res) => {
          this.onUpdateStarted(item, res.data);
        })
        .catch((err) => {
          // request.js already shows the error.
          console.error(err);
        })
        .finally(() => {
          delete this.pendingUpdates[item.ID];
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
    // sourceWarning explains why an app needs attention: the last check
    // failed, or its latest build failed to install and is not retried.
    sourceWarning(item) {
      const source = item.source;
      if (source.check_error) return source.check_error;
      return updateFailed(source) ? this.$t("home.source.update_failed", { version: source.latest_version }) : "";
    },
    // refreshErrorTip maps the refreshed_error code to its message.
    refreshErrorTip(item) {
      if (this.isExternalApp(item)) return this.externalErrorText(item);
      if (item.refreshed_error == 1) {
        return this.$t("home.table.tips.account_invalid");
      }
      if (item.refreshed_error == 5) {
        return this.$t("home.table.tips.certificate_limit");
      }
      if (!item.refreshed_result && item.refreshed_date) {
        return this.$t("home.table.tips.refresh_failed");
      }
      return "";
    },
    // indicatorTip explains the corner badge: the refresh failure reason
    // first, then any source warning.
    indicatorTip(item) {
      const tips = [];
      const refreshTip = this.refreshErrorTip(item);
      if (refreshTip) tips.push(refreshTip);
      const warning = this.sourceWarning(item);
      if (warning) tips.push(warning);
      return tips.join("\n");
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
      // Manual (pairing-file) devices are "added", not discovered as connected.
      const status = item.manual
        ? this.$t("home.sidebar.device_status.added")
        : this.formatStatus(item.status);
      const connection = this.formatConnection(item.connection);

      if (!connection) {
        return status;
      }

      return `${status} · ${connection}`;
    },
    async forgetDevice(item) {
      // First click arms the confirmation, second click deletes.
      if (this.forgetConfirmId !== item.id) {
        this.forgetConfirmId = item.id;
        return;
      }
      this.forgetConfirmId = null;
      try {
        await api.deleteManualDevice(item.id);
        toast.success(this.$t("home.sidebar.device_status.forget_done"));
        this.fetchData();
      } catch (error) {
        console.error(error);
      }
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
      if (this.pendingUpdates[item.ID]) return true;
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
import DownloadIcon from "@/assets/icons/download.svg";
import PlusIcon from "@/assets/icons/plus.svg";
import ChevronRightIcon from "@/assets/icons/chevron-right.svg";
import TrashIcon from "@/assets/icons/trash.svg";
</script>

  
<style lang="postcss" scoped>
.headline {
  @apply prose mb-2;
}

/* Persistent entry that opens the add-device page. It reuses the device
   panel/row so its hover tint is inset by the panel padding, exactly like the
   connected-device rows. */
.atv-add-device-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 50px;
  height: 50px;
  border-radius: 9999px;
  background: var(--atv-accent-soft);
  color: var(--atv-accent);
  flex-shrink: 0;
  transition: background-color .15s ease;
}

/* The row hover tint is almost the same shade as the badge, so the badge
   would blend into the background on hover. Deepen it to keep the icon
   readable while the row is highlighted. */
.atv-device-row:hover .atv-add-device-icon {
  background: var(--atv-accent-soft-hover);
}

.atv-add-device-icon svg {
  width: 22px;
  height: 22px;
}

.atv-add-device-text {
  flex: 1;
  min-width: 0;
}

.atv-add-device-text h4 {
  margin: 0;
  font-size: 0.88rem;
  line-height: 1.35;
  font-weight: 700;
  color: var(--atv-ink);
}

.atv-add-device-text p {
  margin: 0;
  font-size: 0.8rem;
  color: var(--atv-muted);
  line-height: 1.35;
}

.atv-add-device-chevron {
  width: 18px;
  height: 18px;
  color: var(--atv-muted);
  flex-shrink: 0;
}

/* Manual (pairing-file) device: "IP" badge on the avatar's top-right corner
   instead of the online dot. It sits on .avatar (not .avatar > div, which
   daisyUI clips with overflow:hidden) so it is never cut off. */
.atv-ip-badge {
  position: absolute;
  top: -6px;
  right: -6px;
  z-index: 11;
  padding: 2px 5px;
  border-radius: 7px;
  background: var(--atv-accent);
  color: #fff;
  font-size: 9px;
  font-weight: 800;
  line-height: 1;
  letter-spacing: 0.03em;
  box-shadow: 0 0 0 2px var(--atv-surface);
}

/* "Forget" action for a manual device: a trash icon that floats over the far
   right edge of the row without taking any horizontal space from the title
   (absolute, out of flow). Hidden at rest on desktop and revealed on row hover
   or keyboard focus, where it grows a left-to-right fade panel that covers the
   title text beneath it. First click arms the confirmation (the icon turns red
   and shows the confirm label), the second click deletes. Always visible on
   phones, which have no hover. */
.atv-device-subline {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

/* Positioning context for the floating forget panel; overflow keeps the
   panel's fill inside the row's rounded corners. */
.atv-device-row {
  position: relative;
  overflow: hidden;
}

.atv-device-forget {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 5px;
  padding: 0 16px 0 52px;
  border: 0;
  border-radius: 0 12px 12px 0;
  background: linear-gradient(
    to right,
    transparent 0%,
    var(--atv-surface-hover) 46%
  );
  color: var(--atv-muted);
  white-space: nowrap;
  opacity: 0;
  pointer-events: none;
  transition: opacity 140ms ease, color 140ms ease;
}

.atv-device-forget-icon {
  display: inline-flex;
  width: 17px;
  height: 17px;
}

.atv-device-forget-icon svg {
  width: 100%;
  height: 100%;
}

.atv-device-forget-label {
  font-size: 0.78rem;
  font-weight: 650;
  color: var(--atv-danger);
}

.atv-device-row--manual:hover .atv-device-forget,
.atv-device-forget:focus-visible,
.atv-device-forget--armed {
  opacity: 1;
  pointer-events: auto;
}

.atv-device-forget:hover,
.atv-device-forget--armed {
  color: var(--atv-danger);
}

/* Touch devices (phones and iPads) have no hover, so the forget icon is
   always shown. With no hovered row to blend into, the gradient fades to the
   card's own background instead of the hover tint, and the panel is kept
   compact so it covers as little of the title as possible. */
@media (hover: none) {
  .atv-device-forget {
    padding: 0 12px 0 16px;
    background: linear-gradient(
      to right,
      transparent 0%,
      var(--atv-surface) 46%
    );
    opacity: 1;
    pointer-events: auto;
  }
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

/* Danger menu item: keep the original danger red in normal, hover and
   focus states. Beats daisyUI's menu hover rule that resets the color. */
.menu li > .atv-menu-danger {
  color: var(--atv-danger);
}

.menu li > .atv-menu-danger:hover,
.menu li > .atv-menu-danger:focus-visible {
  color: var(--atv-danger-hover);
  background-color: var(--atv-danger-soft);
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
  white-space: nowrap;
  transition: border-color 150ms ease, color 150ms ease;
}

.atv-source-chip:hover,
.atv-source-chip:focus-visible {
  border-color: var(--atv-accent);
  color: var(--atv-accent);
}

/* Keep the tooltip bubble anchored to the corner badge: daisyUI's
   .tooltip sets position relative, which would override the
   .indicator-item absolute corner positioning, so pin it back. */
.atv-indicator-tip {
  position: absolute;
}

/* Undo the white-space: nowrap inherited from .indicator-item so the
   tooltip bubble wraps long error messages instead of one long line,
   while pre-line keeps the line break between combined messages. */
.atv-indicator-tip::before {
  white-space: pre-line;
  text-align: left;
  /* Source errors carry unbreakable URLs: break them so the text stays
     inside the bubble instead of running over it and the table behind. */
  overflow-wrap: anywhere;
}

.atv-source-line .badge-info {
  white-space: nowrap;
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