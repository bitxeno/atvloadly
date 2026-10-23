import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/page/pair/index.vue", import.meta.url), "utf8");
const script = source.match(/<script>([\s\S]*?)<\/script>/)[1]
  .replace(/^import .*;\s*$/gm, "").replace("export default", "globalThis.component =");

function page() {
  const sockets = [], errors = [], consoleMessages = [], timers = new Map();
  let nextTimer = 0;
  class WebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSED = 3;
    readyState = 0;
    sent = [];
    constructor() { sockets.push(this); }
    open() { this.readyState = 1; this.onopen?.(); }
    send(message) { this.sent.push(JSON.parse(message)); }
    close() { this.readyState = 3; this.onclose?.(); }
    message(data) { this.onmessage?.({ data }); }
  }
  const context = vm.createContext({
    WebSocket, location: { protocol: "http:", host: "localhost" },
    console: { log: (...args) => consoleMessages.push(args.join(" ")) },
    toast: { error: (message) => errors.push(message) },
    setTimeout: (fn) => { timers.set(++nextTimer, fn); return nextTimer; },
    clearTimeout: (id) => timers.delete(id),
  });
  vm.runInContext(script, context);
  const state = context.component.data();
  for (const [name, method] of Object.entries(context.component.methods)) state[name] = method.bind(state);
  Object.assign(state, { device: { id: "tv" }, $t: (key) => key, $nextTick: (fn) => fn(), $refs: {} });
  state.beforeUnmount = context.component.beforeUnmount.bind(state);
  state.initWebSocket();
  return { state, socket: sockets[0], timers, errors, consoleMessages };
}

test("waits for the socket before starting and ignores duplicate clicks", () => {
  const { state, socket } = page();
  state.start(); state.start();
  assert.equal(socket.sent.length, 0);
  socket.open();
  assert.deepEqual(socket.sent, [{ t: 1, d: "tv" }]);
});

test("pairing PIN reaches the WebSocket without being logged or retained", () => {
  const { state, socket, consoleMessages } = page();
  socket.open(); state.start();
  socket.message("Enter "); socket.message("PIN:");
  assert.equal(state.loading, false);
  const pin = "314159";
  state.pin = pin; state.confirmPin(); state.confirmPin();
  assert.deepEqual(socket.sent[1], { t: 2, d: pin });
  assert.equal(socket.sent.length, 2);
  assert.equal(state.pin, "");
  assert.equal(consoleMessages.some((message) => message.includes(pin)), false);
});

test("uses the server completion message even when the engine logs no success", () => {
  const { state, socket, timers, errors } = page();
  socket.open(); state.start(); state.status.confirm = true;
  socket.message("Pairing Succeeded!");
  assert.equal(state.active, 2);
  assert.equal(state.success, true);
  assert.equal(state.status.confirm, false);
  assert.equal(timers.size, 0);
  assert.equal(errors.length, 0);
});

test("an engine SUCCESS log alone cannot complete pairing", () => {
  const { state, socket } = page();
  socket.open(); state.start(); socket.message("INFO: SUCCESS: intermediate operation");
  assert.equal(state.active, 1);
  assert.equal(state.success, false);
});

test("failed commands stop the spinner and recognize split incorrect PIN output", () => {
  const { state, socket, errors } = page();
  socket.open(); state.start(); state.status.confirm = true;
  socket.message("Invalid "); socket.message("PIN"); socket.message("Pairing Failed!");
  assert.equal(state.active, 2);
  assert.equal(state.success, false);
  assert.equal(state.status.confirm, false);
  assert.deepEqual(errors, ["pair.toast.pin_incorrect"]);
});

for (const failure of ["close", "error", "timeout"]) {
  test(`${failure} stops a pending pairing and releases the socket`, () => {
    const { state, socket, timers, errors } = page();
    socket.open(); state.start(); state.status.confirm = true;
    if (failure === "timeout") [...timers.values()][0]();
    else if (failure === "error") socket.onerror();
    else socket.close();
    assert.equal(state.active, 2);
    assert.equal(state.status.confirm, false);
    assert.equal(state.success, false);
    assert.equal(socket.readyState, 3);
    assert.equal(timers.size, 0);
    assert.equal(errors.length, 1);
  });
}

test("leaving the page closes the socket and cancels the timer without a failure toast", () => {
  const { state, socket, timers, errors } = page();
  socket.open(); state.start(); state.pin = "123456"; state.beforeUnmount();
  assert.equal(socket.readyState, 3);
  assert.equal(timers.size, 0);
  assert.equal(state.pin, "");
  assert.equal(errors.length, 0);
});
