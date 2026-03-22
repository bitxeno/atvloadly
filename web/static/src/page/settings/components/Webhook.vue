<template>
  <div class="form-item">
    <label class="form-item-label">
      <span class="label-text">{{ $t("settings.notification.webhook.url") }}</span>
    </label>
    <input
      v-model="modelValue.url"
      type="url"
      :placeholder="$t('settings.notification.webhook.url_placeholder')"
      class="input input-bordered grow"
    />
  </div>
  <div class="form-item">
    <label class="form-item-label">
      <span class="label-text">{{
        $t("settings.notification.webhook.method")
      }}</span>
    </label>
    <select v-model="modelValue.method" class="select select-bordered grow">
      <option value="GET">GET</option>
      <option value="POST">POST</option>
    </select>
  </div>
  <div v-show="modelValue.method == 'POST'" class="flex flex-col gap-y-4">
    <div class="form-item">
      <label class="form-item-label">
        <span class="label-text">{{
          $t("settings.notification.webhook.content_type")
        }}</span>
      </label>
      <select
        v-model="modelValue.content_type"
        class="select select-bordered grow"
      >
        <option value="text/plain">text/plain</option>
        <option value="application/json">application/json</option>
      </select>
    </div>
    <div class="form-item">
      <label class="form-item-label">
        <span class="label-text">{{
          $t("settings.notification.webhook.header")
        }}</span>
      </label>
      <input
        v-model="modelValue.header"
        type="text"
        :placeholder="$t('settings.notification.webhook.header_placeholder')"
        class="input input-bordered grow"
      />
    </div>
    <div class="form-item">
      <label class="form-item-label">
        <span class="label-text">{{
          $t("settings.notification.webhook.body")
        }}</span>
      </label>
      <textarea
        v-model="modelValue.body"
        :placeholder="$t('settings.notification.webhook.body_placeholder')"
        class="textarea textarea-bordered grow"
        rows="4"
      ></textarea>
    </div>
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
