import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/page/add-device/index.vue", import.meta.url), "utf8");
const script = source.match(/<script>([\s\S]*?)<\/script>/)?.[1];
assert.ok(script, "add-device page script exists");

// Minimal stubs for the imports used by the component.
function loadComponent({ importPair, discoverTunnelPort }) {
  const context = vm.createContext({
    console: { error() {}, log() {} },
    FormData: class {
      constructor() { this._d = {}; }
      append(k, v) { this._d[k] = v; }
    },
    api: { importPair, discoverTunnelPort },
    toast: { success() {}, error() {} },
    truncateIP: (x) => x,
    DeviceComboBox: {},
  });
  const body = script
    .replace(/^import .*;\s*$/gm, "")
    .replace("export default", "globalThis.component =");
  vm.runInContext(body, context);
  const state = context.component.data();
  for (const [name, fn] of Object.entries(context.component.methods)) {
    state[name] = fn.bind(state);
  }
  state.$t = (k) => k;
  state.$el = { querySelector: () => null };
  state.files = [{ name: "p.plist" }];
  return state;
}

test("iPhone discovered device submits without manual and without probing", async () => {
  let probed = 0;
  let submitted = null;
  const state = loadComponent({
    discoverTunnelPort: async () => { probed++; return { data: { port: 1 } }; },
    importPair: async (fd) => { submitted = fd._d; return {}; },
  });
  state.iphoneIp = "192.168.1.205";
  state.iphoneDevices = [{ ip: "192.168.1.205", port: 49152 }];

  await state.startConnect();

  assert.equal(probed, 0, "discovered device must not trigger tunnel probing");
  assert.ok(submitted, "import should be submitted");
  assert.equal(submitted.ip, "192.168.1.205");
  assert.equal(submitted.port, 49152);
  assert.equal(submitted.manual, undefined, "discovered device must not send manual");
});

test("iPhone LAN manual IP probes tunnel then submits with manual=true", async () => {
  let probedIp = null;
  let submitted = null;
  const state = loadComponent({
    discoverTunnelPort: async (ip) => { probedIp = ip; return { data: { port: 61234 } }; },
    importPair: async (fd) => { submitted = fd._d; return {}; },
  });
  state.iphoneIp = "192.168.1.50";
  state.iphoneDevices = [];

  await state.startConnect();

  assert.equal(probedIp, "192.168.1.50", "manual IP must trigger tunnel probing");
  assert.ok(submitted, "import should be submitted after probing");
  assert.equal(submitted.ip, "192.168.1.50");
  assert.equal(submitted.port, 61234);
  assert.equal(submitted.manual, "true", "manual IP must submit manual=true");
});

test("Tailscale iPhone IP shows the Wi-Fi reminder before connecting", async () => {
  let probed = 0;
  let submitted = null;
  const state = loadComponent({
    discoverTunnelPort: async () => { probed++; return { data: { port: 61234 } }; },
    importPair: async (fd) => { submitted = fd._d; return {}; },
  });
  state.iphoneIp = "100.64.0.9";
  state.iphoneDevices = [];

  await state.startConnect();

  assert.equal(state.tailscaleDialogVisible, true, "Tailscale IP must show the reminder dialog");
  assert.equal(probed, 0, "no probing before the reminder is confirmed");
  assert.equal(submitted, null, "no import before the reminder is confirmed");

  await state.onConfirmTailscale();

  assert.equal(state.tailscaleDialogVisible, false, "confirm closes the reminder");
  assert.equal(probed, 1, "confirm resumes the connect flow");
  assert.ok(submitted, "import should be submitted after confirming");
  assert.equal(submitted.ip, "100.64.0.9");
  assert.equal(submitted.port, 61234);
  assert.equal(submitted.manual, "true");
});

test("cancelling the Tailscale reminder does not connect", async () => {
  let probed = 0;
  let submitted = null;
  const state = loadComponent({
    discoverTunnelPort: async () => { probed++; return { data: { port: 61234 } }; },
    importPair: async (fd) => { submitted = fd._d; return {}; },
  });
  state.iphoneIp = "100.64.0.9";
  state.iphoneDevices = [];

  await state.startConnect();
  state.onCancelTailscale();

  assert.equal(state.tailscaleDialogVisible, false, "cancel closes the reminder");
  assert.equal(state.pendingTailscaleConnect, null, "cancel forgets the pending connect");
  assert.equal(probed, 0, "cancel must not probe");
  assert.equal(submitted, null, "cancel must not import");
});

test("iPhone manual IP with no tunnel port does not submit", async () => {
  let submitted = false;
  const state = loadComponent({
    discoverTunnelPort: async () => ({ data: {} }),
    importPair: async () => { submitted = true; return {}; },
  });
  state.iphoneIp = "192.168.1.50";
  state.iphoneDevices = [];

  await state.startConnect();

  assert.equal(submitted, false, "no tunnel port means no import");
});

test("isTailscaleIP recognises 100.x.x.x addresses only", () => {
  const state = loadComponent({
    discoverTunnelPort: async () => ({}),
    importPair: async () => ({}),
  });

  assert.equal(state.isTailscaleIP("100.64.0.9"), true);
  assert.equal(state.isTailscaleIP("100.101.102.103"), true);
  assert.equal(state.isTailscaleIP(" 100.64.0.9 "), true);
  assert.equal(state.isTailscaleIP("192.168.1.5"), false);
  assert.equal(state.isTailscaleIP("10.0.0.1"), false);
  assert.equal(state.isTailscaleIP("1000.64.0.9"), false);
  assert.equal(state.isTailscaleIP(""), false);
});
