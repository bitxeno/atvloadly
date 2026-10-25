import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/page/pair/index.vue", import.meta.url), "utf8");
const script = source.match(/<script>([\s\S]*?)<\/script>/)?.[1];
assert.ok(script, "pair page script exists");

function loadComponent() {
  const context = vm.createContext({ console: { log() {} } });
  vm.runInContext(
    script.replace(/^import .*;\s*$/gm, "").replace("export default", "globalThis.component ="),
    context,
  );
  const state = context.component.data();
  for (const [name, fn] of Object.entries(context.component.methods)) {
    state[name] = fn.bind(state);
  }
  for (const [name, fn] of Object.entries(context.component.computed || {})) {
    state[name] = fn.bind(state);
  }
  state.$t = (key, params) => (params ? `${key} ${JSON.stringify(params)}` : key);
  return state;
}

test("manual discovery message updates discovery state", () => {
  const state = loadComponent();
  state.isManual = true;
  state.manualIp = "100.64.0.9";

  state.websocketonmessage({ data: JSON.stringify({ kind: "discover", stage: "scanning", percent: 40 }) });
  assert.equal(state.discovery.stage, "scanning");
  assert.equal(state.discovery.percent, 40);

  state.websocketonmessage({ data: JSON.stringify({ kind: "discover", stage: "found", port: 61108, name: "AppleTV" }) });
  assert.equal(state.discovery.stage, "found");
  assert.equal(state.discovery.port, 61108);
  assert.equal(state.discovery.name, "AppleTV");
});

test("manual start sends ip and discovered port (t=4)", () => {
  const state = loadComponent();
  state.isManual = true;
  state.manualIp = "100.64.0.9";
  state.discovery = { stage: "found", percent: 100, port: 61108, name: "AppleTV" };

  const sent = [];
  state.websock = { send: (m) => sent.push(JSON.parse(m)) };

  state.start();

  assert.equal(sent.length, 1);
  assert.equal(sent[0].t, 4);
  assert.deepEqual(JSON.parse(sent[0].d), { ip: "100.64.0.9", port: 61108 });
});

test("manual mode does not send t=1 device pairing", () => {
  const state = loadComponent();
  state.isManual = true;
  state.manualIp = "100.64.0.9";
  state.discovery = { stage: "found", percent: 100, port: 61108, name: "" };
  const sent = [];
  state.websock = { send: (m) => sent.push(JSON.parse(m)) };
  state.start();
  assert.equal(sent.some((m) => m.t === 1), false);
});
