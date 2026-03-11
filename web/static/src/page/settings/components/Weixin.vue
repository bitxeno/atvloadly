<template>
  <div class="form-item">
    <label class="form-item-label">
      <span class="label-text">{{
        $t("settings.notification.weixin.corp_id")
      }}</span>
    </label>
    <input
      v-model="modelValue.corp_id"
      type="text"
      placeholder=""
      class="input input-bordered grow"
    />
  </div>
  <div class="form-item">
    <label class="form-item-label">
      <span class="label-text">{{
        $t("settings.notification.weixin.corp_secret")
      }}</span>
    </label>
    <input
      v-model="modelValue.corp_secret"
      type="text"
      placeholder=""
      class="input input-bordered grow"
    />
  </div>
  <div class="form-item">
    <label class="form-item-label">
      <span class="label-text">{{
        $t("settings.notification.weixin.agent_id")
      }}</span>
    </label>
    <input
      v-model="modelValue.agent_id"
      type="text"
      placeholder=""
      class="input input-bordered grow"
    />
  </div>
  <div class="form-item">
    <label class="form-item-label">
      <span class="label-text">{{
        $t("settings.notification.weixin.touser")
      }}</span>
    </label>
    <input
      v-model="modelValue.to_user"
      type="text"
      placeholder=""
      class="input input-bordered grow"
    />
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: {
    type: Object,
    required: true,
  },
});
</script>
