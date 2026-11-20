<template>
  <div class="app-sidebar" :class="`app-sidebar--${mode}`">
    <!-- Brand + collapse: shown by the overlay panel (phone drawer and the
         expanded tablet rail). The desktop column has no header, the top
         navbar keeps the logo there. -->
    <div class="app-sidebar__head" v-if="mode === 'panel'">
      <router-link :to="{ name: 'home' }" class="app-sidebar__brand" @click="$emit('navigate')">
        <AppIcon class="app-sidebar__brand-mark" />
        <span class="app-sidebar__brand-name">atvloadly</span>
      </router-link>
      <button
        type="button"
        class="app-sidebar__collapse"
        :aria-label="$t('nav.collapse_sidebar')"
        @click="$emit('collapse')"
      >
        <span class="app-sidebar__collapse-icon"><CloseIcon /></span>
      </button>
    </div>

    <!-- Add device -->
    <router-link
      :to="{ name: 'add-device' }"
      class="app-sidebar__add atv-add-device"
      @click="$emit('navigate')"
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

    <!-- Paired devices -->
    <section class="app-sidebar__group">
      <h4 class="app-sidebar__group-title">{{ $t("home.heading.paired_devices") }}</h4>
      <div class="card atv-device-panel" v-show="pairedDevices.length > 0">
        <div
          class="atv-device-row flex flex-row gap-x-3 cursor-pointer"
          v-for="item in pairedDevices"
          v-bind:key="item.id"
          :title="deviceLabel(item)"
          @click="$emit('select-device', item)"
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

          <div class="flex flex-col justify-top flex-1 min-w-0 atv-device-info">
            <h4>{{ item.name }} ({{ truncateIP(item.ip) }})</h4>
            <p class="atv-device-subline">
              <span>{{ formatDeviceStatus(item) }}</span>
            </p>
          </div>

          <!-- Device actions: a dots trigger opens a menu with the delete
               action. Anchored to the trigger so the menu opens down-left,
               clear of the row's right edge. The trigger is a tabindex div
               (not a button) so WebKit focuses it on click and daisyUI opens
               the menu, matching the app-actions menu on the home page. -->
          <div class="dropdown dropdown-end atv-device-menu" @click.stop @mouseleave="$emit('forget-reset')">
            <div
              tabindex="0"
              role="button"
              class="atv-device-more"
              :aria-label="$t('home.sidebar.device_status.more')"
              :title="$t('home.sidebar.device_status.more')"
            >
              <span class="atv-device-more-icon"><DotsIcon /></span>
            </div>
            <ul
              tabindex="0"
              class="dropdown-content z-[2] menu p-1 shadow bg-base-100 rounded-box w-40 gap-1"
            >
              <li>
                <a class="atv-menu-danger" @click.stop.prevent="$emit('forget-device', item)">
                  <span class="w-4 h-4 shrink-0"><TrashIcon /></span>
                  {{
                    forgetConfirmId === item.id
                      ? $t("home.sidebar.device_status.forget_confirm")
                      : $t("home.sidebar.device_status.forget")
                  }}
                </a>
              </li>
            </ul>
          </div>
        </div>
      </div>

      <div
        class="card atv-device-panel h-36 overflow-hidden atv-device-empty"
        v-show="pairedDevices.length == 0"
      >
        <h4 class="flex justify-center items-center">
          {{ $t("home.sidebar.tips.no_paired_devices") }}
        </h4>
        <div class="stat-title whitespace-normal text-sm content-center">
          {{ $t("home.sidebar.tips.how_pair_device") }}
        </div>
      </div>
    </section>

    <!-- Service status -->
    <section class="app-sidebar__group atv-service-status" v-if="services.length > 0">
      <h4 class="app-sidebar__group-title">{{ $t("home.heading.service_status") }}</h4>
      <div class="card atv-device-panel min-h-24 overflow-hidden">
        <ul class="flex flex-col gap-y-2">
          <li
            class="flex items-center gap-x-1"
            v-for="item in services"
            v-bind:key="item.name"
            :title="item.name"
          >
            <div class="w-6 text-green-500" v-show="item.running">
              <CheckMarkIcon />
            </div>
            <div class="w-6 text-red-500" v-show="!item.running">
              <DismissIcon />
            </div>
            <span class="atv-service-name">{{ item.name }}</span>
          </li>
        </ul>
      </div>
    </section>
  </div>
</template>

<script>
import { truncateIP } from "@/utils/utils";

export default {
  name: "AppSidebar",
  props: {
    // "desktop": the static left column; "panel": the overlay drawer body that
    // slides over the content.
    mode: { type: String, default: "desktop" },
    devices: { type: Array, default: () => [] },
    services: { type: Array, default: () => [] },
    // Id of a device armed for the "delete" confirmation.
    forgetConfirmId: { type: [String, Number], default: null },
  },
  emits: ["select-device", "forget-device", "forget-reset", "collapse", "navigate"],
  computed: {
    pairedDevices() {
      return this.devices.filter((item) => item.status == "paired");
    },
  },
  methods: {
    truncateIP,
    deviceLabel(item) {
      return `${item.name} (${truncateIP(item.ip)})`;
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
    formatDeviceStatus(item) {
      // Manual (pairing-file) devices are "added", not discovered as connected.
      const status = item.manual
        ? this.$t("home.sidebar.device_status.added")
        : this.formatStatus(item.status);
      const connection = item.connection;

      if (!connection) {
        return status;
      }

      return `${status} · ${connection}`;
    },
  },
};
</script>

<script setup>
import AppIcon from "@/assets/icons/app.svg";
import PlusIcon from "@/assets/icons/plus.svg";
import ChevronRightIcon from "@/assets/icons/chevron-right.svg";
import IPhoneIcon from "@/assets/icons/iphone.svg";
import AppleTVIcon from "@/assets/icons/appletv.svg";
import DotsIcon from "@/assets/icons/dots.svg";
import TrashIcon from "@/assets/icons/trash.svg";
import CheckMarkIcon from "@/assets/icons/checkmark.svg";
import DismissIcon from "@/assets/icons/dismiss.svg";
import CloseIcon from "@/assets/icons/close.svg";
</script>

<style lang="postcss" scoped>
/* ---- Shell layout: desktop column / tablet rail / phone drawer ---- */
.app-sidebar {
  display: flex;
  flex-direction: column;
  gap: 22px;
  min-width: 0;
}

/* Desktop keeps the original left column: no chrome of its own, the cards
   below sit directly on the page background. */
.app-sidebar--desktop {
  gap: 24px;
}

/* Overlay panel: the phone drawer and the expanded tablet rail share the same
   full-height surface that slides in over the content from the left. */
.app-sidebar--panel {
  height: 100%;
  padding: 18px 16px 40px;
  background: var(--atv-surface);
  box-shadow: var(--atv-menu-shadow);
  overflow-y: auto;
  overscroll-behavior: contain;
}

/* Head row: brand on the left, collapse button on the right. */
.app-sidebar__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 44px;
}

.app-sidebar__brand {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
  padding: 4px 6px;
  margin: -4px -6px;
  border-radius: 10px;
  color: var(--atv-ink);
  text-decoration: none;
  transition: background-color 150ms ease;
}

.app-sidebar__brand:hover {
  background: var(--atv-surface-hover);
}

.app-sidebar__brand-mark {
  width: 28px;
  height: 28px;
  flex-shrink: 0;
}

.app-sidebar__brand-name {
  font-size: 1.12rem;
  font-weight: 760;
  letter-spacing: -0.035em;
  white-space: nowrap;
  overflow: hidden;
}

.app-sidebar__collapse {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--atv-muted);
  cursor: pointer;
  transition: background-color 150ms ease, color 150ms ease;
}

.app-sidebar__collapse:hover,
.app-sidebar__collapse:focus-visible {
  background: var(--atv-surface-hover);
  color: var(--atv-accent);
}

.app-sidebar__collapse-icon {
  display: inline-flex;
  width: 20px;
  height: 20px;
}

.app-sidebar__collapse-icon svg {
  width: 100%;
  height: 100%;
}

.app-sidebar__group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.app-sidebar__group-title {
  margin: 0;
  padding-inline: 2px;
  color: var(--atv-ink);
  font-size: 0.93rem;
  font-weight: 675;
  letter-spacing: -0.012em;
  white-space: nowrap;
  overflow: hidden;
}

/* ---- Add-device entry (shared with the old desktop column) ---- */
.atv-add-device {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 12px;
  border-radius: 16px;
  background: var(--atv-surface);
  border: 1px solid var(--atv-border);
  box-shadow: var(--atv-shadow);
  text-decoration: none;
  transition: background-color 150ms ease;
}

.atv-add-device:hover {
  background: var(--atv-surface-hover);
}

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
.atv-add-device:hover .atv-add-device-icon {
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

/* Align the add-device, connected-device and service-status icons on one
   vertical line. The device/service panels carry no padding (see app.css) and
   each device row / service item adds 12px, matching the add-device card's
   natural 12px horizontal padding — so every icon starts at 12px from the
   column/panel edge. */

/* Service-status items share the device panel but have no row of their own;
   give them the same 12px horizontal padding as a device row. */
.atv-service-status .atv-device-panel > ul > li {
  padding-left: 12px;
  padding-right: 12px;
}

/* ---- Device rows (shared with the old desktop column) ---- */
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

.atv-device-subline {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

/* Positioning context for the row's action menu. */
.atv-device-row {
  position: relative;
}

/* Device action menu: the dots trigger sits inline at the row's right edge
   (takes its own space, no overlay). Hidden at rest on desktop and revealed on
   row hover or keyboard focus; always visible on phones, which have no hover.
   daisyUI's .dropdown keeps the menu anchored to this trigger. */
.atv-device-menu {
  flex-shrink: 0;
}

.atv-device-more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--atv-muted);
  cursor: pointer;
  opacity: 0;
  pointer-events: none;
  transition: opacity 140ms ease, background-color 150ms ease, color 150ms ease;
}

.atv-device-more-icon {
  display: inline-flex;
  width: 18px;
  height: 18px;
}

.atv-device-more-icon svg {
  width: 100%;
  height: 100%;
}

.atv-device-row:hover .atv-device-more,
.atv-device-menu:focus-within .atv-device-more,
.atv-device-more:focus-visible {
  opacity: 1;
  pointer-events: auto;
}

.atv-device-more:hover,
.atv-device-more:focus-visible {
  background: var(--atv-surface-hover);
  color: var(--atv-ink);
}

/* Touch devices (phones and iPads) have no hover, so the dots trigger is
   always shown. */
@media (hover: none) {
  .atv-device-more {
    opacity: 1;
    pointer-events: auto;
  }
}

/* Danger menu item: keep the danger red in normal, hover and focus states,
   beating daisyUI's menu hover rule that resets the color. */
.atv-menu-danger {
  color: var(--atv-danger) !important;
}

.atv-menu-danger:hover,
.atv-menu-danger:focus-visible {
  background: var(--atv-surface-hover);
  color: var(--atv-danger) !important;
}

/* Empty-state card height parity with the old column. */
.atv-device-empty {
  display: grid;
  align-content: center;
  gap: 10px;
  padding: 14px;
}
</style>
