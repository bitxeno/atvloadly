import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/page/pair/index.vue", import.meta.url), "utf8");
const script = source.match(/<script>([\s\S]*?)<\/script>/)?.[1];
assert.ok(script, "pair page script exists");

test("pairing PIN reaches the WebSocket without being logged", () => {
  const consoleMessages = [];
  const sent = [];
  const context = vm.createContext({
    console: { log: (...args) => consoleMessages.push(args.join(" ")) },
  });
  vm.runInContext(
    script.replace(/^import .*;\s*$/gm, "").replace("export default", "globalThis.component ="),
    context,
  );
  const state = context.component.data();
  for (const [name, method] of Object.entries(context.component.methods)) {
    state[name] = method.bind(state);
  }
  state.websock = { send: (message) => sent.push(JSON.parse(message)) };

  const pin = "314159";
  state.pin = pin;
  state.confirmPin();

  assert.equal(sent.length, 1);
  assert.equal(sent[0].t, 2);
  assert.equal(sent[0].d, pin);
  assert.equal(consoleMessages.some((message) => message.includes(pin)), false);
});
