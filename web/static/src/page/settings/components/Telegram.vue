<template>
  <div class="form-item">
    <label class="form-item-label">
      <span class="label-text">BotToken</span>
    </label>
    <input
      v-model="modelValue.bot_token"
      type="text"
      placeholder=""
      class="input input-bordered grow"
    />
  </div>
  <div class="form-item">
    <label class="form-item-label">
      <span class="label-text">ChatID</span>
    </label>
    <input
      v-model="modelValue.chat_id"
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
