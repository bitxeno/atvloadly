<template>
  <div class="container mx-auto p-4">
    <h1 class="text-2xl font-bold mb-4">{{ $t('tools.ssdp.title') }}</h1>

    <div class="mockup-code overflow-y-auto h-[600px]">
      <pre v-for="(log, index) in logs" :key="index" class="px-4 py-1"><code :style="log.color ? { color: log.color } : {}">{{ typeof log === 'string' ? log : log.text }}</code></pre>
    </div>

        <div
        class="stat-title text-sm flex flex-row items-center gap-x-1 whitespace-break-spaces mt-2"
      >
        <div class="w-4"><HelpIcon /></div>
          {{ $t('tools.ssdp.pairable_label') }}
      </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';

const scanning = ref(false);
const logs = ref([]);
let ws = null;

const startScan = () => {
  scanning.value = true;
  logs.value = [];
  
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(`${protocol}//${window.location.host}/ws/tools/scan`);

  ws.onopen = () => {
    logs.value.push("Connected to scanner...");
  };

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      // Format: [Type] Name - Host (IP:Port)
      const hostPart = data.host ? ` - ${data.host}` : '';
      const formatted = `[${data.type}] ${data.name}${hostPart} (${data.address}:${data.port})`;
      if (data.type && data.type.includes('apple-pairable')) {
        logs.value.push({ text: formatted, color: '#00dfae' });
      } else {
        logs.value.push({ text: formatted });
      }
    } catch (e) {
      logs.value.push({ text: event.data });
    }
  };

  ws.onclose = () => {
    scanning.value = false;
    logs.value.push("Connection closed.");
  };

  ws.onerror = (err) => {
    scanning.value = false;
    logs.value.push(`Error: Connection error`);
  };
};

const stopScan = () => {
  if (ws) {
    ws.close();
    ws = null;
  }
  scanning.value = false;
};

onMounted(() => {
  startScan();
});

onUnmounted(() => {
  stopScan();
});
</script>

<script>
import HelpIcon from "@/assets/icons/help.svg";
</script>
