import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";
import { createSigningReportStream } from "../src/utils/signing-report.mjs";

const source = readFileSync(new URL("../src/page/install/index.vue", import.meta.url), "utf8");
const script = source.match(/<script>([\s\S]*?)<\/script>/)[1]
  .replace(/^import[\s\S]*?;\s*/gm, "")
  .replace("export default", "globalThis.component =");

function page(upload = async () => [{ name: "app.ipa", path: "/tmp/app.ipa" }], checkAfcService) {
  const sockets = [];
  const errors = [];
  class WebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;
    readyState = WebSocket.CONNECTING;
    sent = [];
    constructor() { sockets.push(this); }
    open() { this.readyState = WebSocket.OPEN; this.onopen?.({ currentTarget: this }); }
    close() { this.readyState = WebSocket.CLOSED; this.onclose?.({ currentTarget: this, code: 1006 }); }
    send(message) { this.sent.push(JSON.parse(message)); }
  }
  const context = vm.createContext({
    SigningIssueList: {}, SigningPlan: {}, SourcePicker: {},
    WebSocket, createSigningReportStream, FormData,
    location: { protocol: "http:", host: "localhost" },
    api: { upload, checkAfcService },
    toast: { error: (message) => errors.push(message), success() {} },
    console: { log() {} },
  });
  vm.runInContext(script, context);
  const component = context.component;
  const state = component.data();
  for (const [name, method] of Object.entries(component.methods)) {
    state[name] = method.bind(state);
  }
  Object.defineProperty(state, "canInstall", { configurable: true, get: () => component.computed.canInstall.call(state) });
  Object.assign(state, {
    isExternal: false, installMode: "link", ipaUrl: "https://example.com/app.ipa",
    device: { connection: "RPPairing" }, reportStream: createSigningReportStream(),
    $t: (key) => key, validateForm: () => true, startUpdateLog() {}, stopUpdateLog() {},
  });
  component.mounted?.call(state);
  return { state, sockets, errors };
}

const settle = () => new Promise(setImmediate);

test("waits for the install socket to open before sending", async () => {
  const { state, sockets } = page();
  const pending = state.onSubmit();
  assert.equal(sockets.length, 1);
  assert.equal(sockets[0].sent.length, 0);
  assert.equal(state.loading, true);
  await state.onSubmit();
  assert.equal(sockets.length, 1);
  sockets[0].open();
  await pending;
  assert.equal(sockets[0].sent.length, 1);
  assert.equal(sockets[0].sent[0].t, 1);
  assert.equal(state.loading, true);
});

test("ends a disconnected install and connects only for the next submission", async () => {
  const { state, sockets, errors } = page();
  const first = state.onSubmit();
  sockets[0].open();
  await first;
  const oldClose = sockets[0].onclose;
  const oldMessage = sockets[0].onmessage;
  sockets[0].close();
  assert.equal(state.loading, false);
  assert.equal(errors.at(-1), "install.toast.connection_closed");
  assert.equal(sockets.length, 1);

  const second = state.onSubmit();
  assert.equal(sockets.length, 2);
  oldClose({ currentTarget: sockets[0], code: 1006 });
  oldMessage({ currentTarget: sockets[0], data: "Installation Succeeded" });
  assert.equal(state.loading, true);
  sockets[1].open();
  await second;
  assert.equal(sockets[0].sent.length, 1);
  assert.equal(sockets[1].sent.length, 1);
});

test("does not send a late upload on a replacement socket", async () => {
  let finishUpload;
  const { state, sockets } = page(() => new Promise((resolve) => { finishUpload = resolve; }));
  state.installMode = "file";
  const first = state.onSubmit();
  sockets[0].open();
  await settle();
  sockets[0].close();
  state.installMode = "link";
  const second = state.onSubmit();
  sockets[1].open();
  await second;
  finishUpload([{ name: "old.ipa", path: "/tmp/old.ipa" }]);
  await first;
  assert.equal(sockets[0].sent.length, 0);
  assert.equal(sockets[1].sent.length, 1);
  assert.equal(state.ipa.name, "app.ipa");
  assert.equal(state.loading, true);
});

test("closing the page during connection does not submit or show a failure", async () => {
  const { state, sockets, errors } = page();
  const pending = state.onSubmit();
  state.closeWebSocket();
  await pending;
  assert.equal(sockets[0].sent.length, 0);
  assert.equal(errors.length, 0);
});

test("a completed install can close without reporting a failure", async () => {
  const { state, sockets, errors } = page();
  const pending = state.onSubmit();
  sockets[0].open();
  await pending;
  sockets[0].onmessage({ currentTarget: sockets[0], data: "Installation Succeeded" });
  assert.equal(state.loading, false);
  sockets[0].close();
  assert.equal(errors.length, 0);
});

test("a connection failure releases the prepared external IPA", async () => {
  const { state, sockets, errors } = page();
  Object.defineProperty(state, "canInstall", { configurable: true, value: true });
  state.isExternal = true;
  state.signing.mode = "external_certificate";
  state.signing.uploaded = { name: "app.ipa", path: "/tmp/app.ipa" };
  const pending = state.onSubmit();
  sockets[0].onerror();
  await pending;
  assert.equal(state.loading, false);
  assert.equal(state.signing.uploaded, null);
  assert.equal(errors.length, 1);
});

test("does not upload after a disconnected AFC check finishes", async () => {
  let finishCheck;
  let uploads = 0;
  const { state, sockets } = page(
    async () => { uploads++; return []; },
    () => new Promise((resolve) => { finishCheck = resolve; }),
  );
  state.device.connection = "Lockdown";
  state.installMode = "file";
  const first = state.onSubmit();
  sockets[0].open();
  await settle();
  sockets[0].close();
  state.device.connection = "RPPairing";
  state.installMode = "link";
  const second = state.onSubmit();
  sockets[1].open();
  await second;
  const output = state.log.output;
  finishCheck();
  await first;
  assert.equal(uploads, 0);
  assert.equal(state.log.output, output);
  assert.equal(sockets[1].sent.length, 1);
});
