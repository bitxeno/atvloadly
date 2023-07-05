<template>
  <div class="lg:flex lg:flex-row lg:gap-x-12">
    <div class="lg:basis-3/12 flex gap-x-16 flex-col gap-y-8 mb-8">
      <div v-show="pairableDevices.length > 0">
        <h4 class="mb-2">待配对设备</h4>
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
        <h4 class="mb-2">已连接设备</h4>
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
            <h4 class="flex justify-center items-center">没有连接设备</h4>

            <div class="stat-title whitespace-normal text-sm content-center">
              AppleTV 请转到『<span class="font-medium italic"
                >设置 -> 遥控器与设备 -> 遥控器App与设备</span
              >』，进入配对模式完成配对
            </div>
          </div>
        </div>
      </div>

      <div>
        <h4 class="mb-2">服务状态</h4>
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
      <h4>已安装 Apps</h4>
      <div class="overflow-x-auto">
        <table class="table table-auto static">
          <!-- head -->
          <thead>
            <tr>
              <th>app</th>
              <th>设备</th>
              <th>帐号</th>
              <th>过期时间</th>
              <th v-show="false">最近刷新时间</th>
              <th>操作</th>
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
                        <img :src="iconUrl(item)" class="rounded-md"/>
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
                        title="点击查看安装日志"
                        target="_blank"
                        >{{ formatRefreshDate(item) }}</a
                      >
                    </div>
                  </div>
                </div>
              </td>
              <td>
                {{ formatDeviceName(item) }}
              </td>
              <td>{{ item.account }}</td>
              <td>
                <div class="badge badge-ghost w-16">
                  {{ formatExpiredTime(item) }}
                </div>
              </td>
              <td v-show="false">
                {{ formatRefreshDate(item) }}
              </td>
              <td>
                <div class="flex gap-x-2">
                  <a class="link link-primary" @click="refreshApp(item)"
                    >刷新</a
                  >
                  <Popper placement="top" arrow="true">
                    <template #content="{ close }">
                      <div class="flex flex-col gap-y-2">
                        <div class="py-2">
                          确定要删除 {{ item.ipa_name }} 吗？
                        </div>
                        <div class="flex gap-x-2 justify-end items-center">
                          <a class="link link-primary link-hover" @click="close"
                            >取消</a
                          >
                          <button
                            class="btn btn-primary btn-xs"
                            @click="deleteApp(item, close)"
                          >
                            确定
                          </button>
                        </div>
                      </div>
                    </template>
                    <a class="link link-error">删除</a>
                  </Popper>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div class="empty" v-show="list.length == 0">没有数据</div>
      </div>

      <div
        class="stat-title text-sm flex flex-row items-center gap-x-1 whitespace-break-spaces"
      >
        <div class="w-4"><HelpIcon /></div>
        默认当app过期时间小于1天时，会在凌晨3～6点钟自动进行刷新操作
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
      installingApp: null,
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
        // 为空的话，等待5秒再获取一次
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

      api.getInstallingApp().then((res) => {
        // res.data返回为空表示已安装执行完成
        if (_this.installingApp && !res.data) {
          _this.fetchAppList();
        }

        _this.installingApp = res.data;
        if (_this.installingApp) {
          // 重复检测直到完成
          _this.checkInstallingAppDelay();
        }
      });
    },
    checkInstallingAppDelay() {
      let _this = this;

      if (this.checkInstallingTimer) {
        clearTimeout(this.checkInstallingTimer);
      }

      this.checkInstallingTimer = setTimeout(function() {
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

      _this.installingApp = item;

      api.refreshApp(item.ID).then((res) => {
        _this.checkInstallingAppDelay();
        toast.success(`已启动刷新${item.ipa_name}`);
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
      if (diff < 0) return "已过期";

      return `${diff}天后`;
    },
    formatStatus(status) {
      if (status == "paired") {
        return "已连接";
      } else if (status == "pairable") {
        return "待配对";
      } else {
        return "未连接";
      }
    },
    formatRefreshDate(item) {
      if (this.installingApp && this.installingApp.ID == item.ID) {
        return "安装中...";
      }

      if (!item.refreshed_date) return "-";

      let seconds = dayjs().diff(dayjs(item.refreshed_date), "second");
      if (seconds < 60) {
        return `${seconds}秒前`;
      }
      let miniutes = parseInt(seconds / 60, 10);
      if (miniutes < 60) {
        return `${miniutes}分钟前`;
      }
      let hours = parseInt(seconds / 3600, 10);
      if (hours < 24) {
        return `${hours}小时前`;
      }
      let days = parseInt(seconds / 24 / 3600, 10);
      return `${days}天前`;
    },
    formatRefreshResult(item) {
      if (!item.refreshed_date) return "";

      return item.refreshed_result ? "成功" : "失败";
    },
    formatDeviceName(item) {
      let _this = this;
      for (let i = 0; i < _this.devices.length; i++) {
        const dev = _this.devices[i];
        if (dev.udid == item.udid && dev.status == "paired") {
          return `${dev.ip}`;
        }
      }
      return "未连接";
    },
    isInstalling(item) {
      if (!this.installingApp) return false;

      return item.ID == this.installingApp.ID;
    },
    iconUrl(app) {
      if (app.icon) {
        return `/apps/${app.ID}/icon`;
      } else {
        return '/img/dummy.jpg';
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