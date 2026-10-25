<template>
  <div ref="root" class="dropdown w-full atv-combo">
    <div class="join w-full atv-join">
      <input
        v-model="text"
        type="text"
        class="input input-bordered join-item flex-1 min-w-0"
        :placeholder="placeholder"
        :disabled="disabled"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        @focus="open = true"
        @keydown.enter.prevent="open = false"
      />
      <button
        type="button"
        class="btn join-item"
        :disabled="disabled || refreshing"
        :title="refreshLabel"
        :aria-label="refreshLabel"
        @click="onRefresh"
      >
        <span class="w-5 h-5" :class="{ 'animate-spin': refreshing }"><RefreshIcon /></span>
      </button>
    </div>
    <ul
      v-show="open && options.length > 0"
      class="dropdown-content z-[1] menu p-2 shadow bg-base-100 rounded-box w-full gap-1 atv-combo-list"
    >
      <li v-for="option in options" :key="option.value">
        <!-- mousedown.prevent keeps focus on the input: WebKit moves focus on
             mousedown, which would close the list before the click lands. -->
        <button type="button" @mousedown.prevent @click="pick(option)">
          <span class="atv-combo-label">{{ option.label }}</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<script>
import RefreshIcon from "@/assets/icons/refresh.svg";

export default {
  name: "DeviceComboBox",
  components: { RefreshIcon },
  props: {
    modelValue: { type: String, default: "" },
    options: { type: Array, default: () => [] },
    placeholder: { type: String, default: "" },
    refreshLabel: { type: String, default: "Refresh" },
    refreshing: { type: Boolean, default: false },
    disabled: { type: Boolean, default: false },
  },
  emits: ["update:modelValue", "refresh"],
  data() {
    return {
      open: false,
      text: this.modelValue,
    };
  },
  watch: {
    modelValue(value) {
      this.text = value;
    },
    text(value) {
      // Direct typing is a first-class way to enter an IP: keep the parent in
      // sync on every keystroke so it can submit what the user typed.
      this.$emit("update:modelValue", value);
    },
  },
  mounted() {
    // Close the dropdown when a click lands outside the combobox.
    this.outsideClose = (event) => {
      if (this.open && this.$refs.root && !this.$refs.root.contains(event.target)) {
        this.open = false;
      }
    };
    document.addEventListener("click", this.outsideClose);
  },
  beforeUnmount() {
    document.removeEventListener("click", this.outsideClose);
  },
  methods: {
    pick(option) {
      this.text = option.value;
      this.open = false;
    },
    onRefresh() {
      this.$emit("refresh");
    },
  },
};
</script>

<style lang="postcss" scoped>
.atv-combo-list {
  max-height: 260px;
  overflow-y: auto;
}

.atv-combo-label {
  display: block;
  width: 100%;
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Phones: give each option a comfortable tap target and let long device
   names wrap instead of being cut off. */
@media (max-width: 767px) {
  .atv-combo-list {
    max-height: 45vh;
  }

  .atv-combo-list li > button {
    min-height: 44px;
    padding: 10px 12px;
  }

  .atv-combo-label {
    white-space: normal;
    overflow: visible;
    text-overflow: clip;
    word-break: break-word;
    line-height: 1.35;
  }
}
</style>
