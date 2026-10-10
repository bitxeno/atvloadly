import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";
import { createSigningReportStream } from "../src/utils/signing-report.mjs";
import {
  installFailureMessage,
  needsCertificateReset,
  pickRevocableCertificates,
} from "../src/utils/install-error-feedback.mjs";

const source = readFileSync(new URL("../src/page/install/index.vue", import.meta.url), "utf8");
const script = source.match(/<script>([\s\S]*?)<\/script>/)[1]
  .replace(/^import[\s\S]*?;\s*/gm, "")
  .replace("export default", "globalThis.component =");

function page(upload = async () => [{ name: "app.ipa", path: "/tmp/app.ipa" }], checkAfcService, certificates = async () => ({ data: [] })) {
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
    formatInstallFailureMessage: installFailureMessage,
    needsCertificateReset, pickRevocableCertificates,
    location: { protocol: "http:", host: "localhost" },
    api: { upload, checkAfcService, getCertificates: certificates },
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
  if (component.unmounted) {
    state.unmounted = component.unmounted.bind(state);
  }
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

test("a stale socket closing during the next upload cannot end it", async () => {
  let finishUpload;
  const { state, sockets, errors } = page(() => new Promise((resolve) => { finishUpload = resolve; }));
  // A previous attempt leaves its idle socket behind after succeeding.
  const first = state.onSubmit();
  await settle();
  sockets[0].open();
  await first;
  sockets[0].onmessage({ currentTarget: sockets[0], data: "Installation Succeeded" });
  assert.equal(state.loading, false);
  assert.equal(state.websock, sockets[0]);

  // The next attempt releases that socket up front, then uploads with no
  // socket open for proxies to drop.
  state.installMode = "file";
  const second = state.onSubmit();
  await settle();
  await settle();
  assert.equal(state.websock, null);
  assert.equal(sockets.length, 1);
  // A late FIN for the previous socket arrives mid-upload: ignored.
  sockets[0].close();
  assert.equal(errors.length, 0);
  assert.equal(state.loading, true);
  finishUpload([{ name: "app.ipa", path: "/tmp/app.ipa" }]);
  await settle();
  await settle();
  assert.equal(sockets.length, 2);
  sockets[1].open();
  await second;
  assert.equal(sockets[1].sent.length, 1);
  assert.equal(sockets[1].sent[0].t, 1);
  assert.equal(state.loading, true);
});

test("does not send a late upload after the page is left", async () => {
  let finishUpload;
  const { state, sockets, errors } = page(() => new Promise((resolve) => { finishUpload = resolve; }));
  state.installMode = "file";
  const first = state.onSubmit();
  await settle();
  // No socket opens before the upload finishes: the idle window with zero
  // traffic that proxies and routers close is gone.
  assert.equal(sockets.length, 0);
  state.unmounted();
  finishUpload([{ name: "old.ipa", path: "/tmp/old.ipa" }]);
  await first;
  assert.equal(sockets.length, 0);
  assert.equal(errors.length, 0);
});

test("opens the install socket only after the file upload finishes", async () => {
  let finishUpload;
  const { state, sockets } = page(() => new Promise((resolve) => { finishUpload = resolve; }));
  state.installMode = "file";
  const pending = state.onSubmit();
  await settle();
  await settle();
  // The large upload is in flight with no idle socket for proxies to drop.
  assert.equal(sockets.length, 0);
  assert.equal(state.loading, true);
  finishUpload([{ name: "app.ipa", path: "/tmp/app.ipa" }]);
  await settle();
  await settle();
  assert.equal(sockets.length, 1);
  assert.equal(sockets[0].sent.length, 0);
  sockets[0].open();
  await pending;
  assert.equal(sockets[0].sent.length, 1);
  assert.equal(sockets[0].sent[0].t, 1);
});

test("closing the page during connection does not submit or show a failure", async () => {
  const { state, sockets, errors } = page();
  const pending = state.onSubmit();
  state.unmounted();
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

test("does not upload after the page is left during the AFC check", async () => {
  let finishCheck;
  let uploads = 0;
  const { state, sockets } = page(
    async () => { uploads++; return []; },
    () => new Promise((resolve) => { finishCheck = resolve; }),
  );
  state.device.connection = "Lockdown";
  state.installMode = "file";
  const first = state.onSubmit();
  await settle();
  // The AFC check runs before any socket opens, so the check itself cannot
  // be "disconnected": release the attempt by navigating away instead.
  assert.equal(sockets.length, 0);
  state.unmounted();
  const output = state.log.output;
  finishCheck();
  await first;
  assert.equal(uploads, 0);
  assert.equal(state.log.output, output);
  assert.equal(sockets.length, 0);
});

// The engine reports a required revocation as a failure line carrying its
// marker; the dialog lists the account certificates and the user answer is a
// resend of the same install with exactly one authorized serial.
const resetRequiredLog = [
  "[INFO plumesign] Developer API error 7460",
  "Error: [certificate_reset_required] The certificate limit of this account is reached.",
].join("\n");

const accountCerts = [
  { serialNumber: "AAAA1111", name: "iOS Development: SideStore", machineName: "SideStore - Ann's iPhone", status: "Issued", expirationDate: "2027-01-01" },
  { serialNumber: "BBBB2222", name: "iOS Development: iloader", machineName: "iloader", status: "Issued", expirationDate: "2027-02-01" },
];

async function failedInstall(withMarker) {
  const { state, sockets, errors } = page(
    undefined,
    undefined,
    async () => ({ data: accountCerts }),
  );
  const pending = state.onSubmit();
  sockets[0].open();
  await pending;
  const line = withMarker ? `${resetRequiredLog}\nInstallation Failed!` : "Installation Failed!";
  sockets[0].onmessage({ currentTarget: sockets[0], data: line });
  await settle();
  return { state, sockets, errors };
}

test("offers the certificate dialog when the engine stops at the limit", async () => {
  const { state, errors } = await failedInstall(true);
  assert.equal(state.certRevoke.visible, true);
  // The generic failure toast waits for the user decision.
  assert.equal(errors.length, 0);
  assert.deepEqual(
    state.certRevoke.certificates.map((cert) => cert.serialNumber),
    ["AAAA1111", "BBBB2222"],
  );
});

test("confirming resends the exact install with the authorized serial", async () => {
  const { state, sockets } = await failedInstall(true);
  state.certRevoke.selectedSerial = "BBBB2222";
  state.confirmCertRevoke();

  assert.equal(state.certRevoke.visible, false);
  assert.equal(state.loading, true);
  const retried = sockets[0].sent.at(-1);
  assert.equal(retried.t, 1);
  const install = JSON.parse(retried.d);
  assert.equal(install.revoke_certificate_serial, "BBBB2222");
  // The same staged install request, not a freshly built one.
  assert.equal(install.ipa_path, "https://example.com/app.ipa");
  assert.equal(install.account, "");
  // The authorization is spent: a later failure cannot reuse it silently.
  assert.equal(state.certRevoke.payload, null);

  sockets[0].onmessage({ currentTarget: sockets[0], data: "Installation Succeeded" });
  assert.equal(state.loading, false);
  assert.equal(state.certRevoke.visible, false);
});

test("cancelling the certificate dialog reports the plain failure", async () => {
  const { state, sockets, errors } = await failedInstall(true);
  state.closeCertRevoke();
  assert.equal(state.certRevoke.visible, false);
  assert.equal(state.certRevoke.payload, null);
  // A cancel never sends another install; the engine already revoked nothing.
  assert.equal(sockets[0].sent.length, 1);

  // A second failure without an offer behaves like any other failure.
  sockets[0].onmessage({ currentTarget: sockets[0], data: "Installation Failed!" });
  assert.equal(errors.length, 1);
  assert.equal(state.certRevoke.visible, false);
});

test("an ordinary failure never opens the certificate dialog", async () => {
  const { state, errors } = await failedInstall(false);
  assert.equal(state.certRevoke.visible, false);
  assert.equal(state.certRevoke.payload, null);
  assert.equal(errors.length, 1);
});

test("an external install has no certificate offer", async () => {
  const { state, sockets, errors } = page();
  Object.defineProperty(state, "canInstall", { configurable: true, value: true });
  state.isExternal = true;
  state.signing.mode = "external_certificate";
  state.signing.uploaded = { name: "app.ipa", path: "/tmp/app.ipa" };
  const pending = state.onSubmit();
  sockets[0].open();
  await pending;
  assert.equal(state.certRevoke.payload, null);

  sockets[0].onmessage({ currentTarget: sockets[0], data: `${resetRequiredLog}\nInstallation Failed!` });
  assert.equal(state.certRevoke.visible, false);
  assert.equal(errors.length, 1);
});

test("revoked and expired certificates are not offered for pick", () => {
  const picked = pickRevocableCertificates([
    ...accountCerts,
    { serialNumber: "CC33", status: "Revoked" },
    { serialNumber: "DD44", status: "Expired" },
    { name: "no serial" },
  ]);
  assert.deepEqual(picked.map((cert) => cert.serialNumber), ["AAAA1111", "BBBB2222"]);
  assert.equal(needsCertificateReset("plain failure"), false);
});
