import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../src/components/Login.vue", import.meta.url), "utf8");
const HELP_URL =
  "https://github.com/bitxeno/atvloadly/wiki/How-to-resolve-the-login-error";

function loadComponent() {
  const script = source
    .match(/<script>([\s\S]*?)<\/script>/)[1]
    .replace(/^import .*;$/gm, "")
    .replace("export default", "globalThis.component =");
  const context = vm.createContext({
    HelpIcon: {},
    toast: { error: () => {}, success: () => {} },
  });
  vm.runInContext(script, context);
  return context.component;
}

function makeLogin(component) {
  return {
    ...component.data(),
    ...component.methods,
    $t: (key) => key,
    $refs: {},
  };
}

test("a failed login surfaces the collected error and the help URL", () => {
  const login = makeLogin(loadComponent());

  login.loginWebsocketonmessage({ data: "ERROR: incorrect password\n" });
  login.loginWebsocketonmessage({ data: "Login Failed!" });

  assert.equal(login.loginFailed, true);
  assert.equal(login.loginLoading, false);
  assert.equal(login.loginErr, "ERROR: incorrect password\n");
  assert.equal(login.loginHelpUrl, HELP_URL);
});

test("retrying or reopening login clears the previous failure", () => {
  const component = loadComponent();
  const login = makeLogin(component);

  login.loginWebsocketonmessage({ data: "ERROR: bad credentials" });
  login.loginWebsocketonmessage({ data: "Login Failed!" });
  assert.equal(login.loginFailed, true);

  login.show();
  assert.equal(login.loginFailed, false);
  assert.equal(login.loginErr, "");
});

test("the help link is gated on login failure and sits at the left of the action row", () => {
  const template = source.match(/<template>([\s\S]*?)<\/template>/)[1];
  const actionRows = [...template.matchAll(/<div class="modal-action">([\s\S]*?)<\/div>/g)].map((m) => m[1]);

  assert.equal(actionRows.length, 2, "both dialogs have an action row");
  for (const row of actionRows) {
    assert.ok(row.includes("v-if=\"loginFailed\""), "the help link is hidden until failure");
    assert.ok(row.indexOf("v-if=\"loginFailed\"") < row.indexOf("<button"), "the link precedes the buttons");
    assert.ok(row.includes("mr-auto"), "the link is pushed to the far left");
    assert.ok(row.includes("self-center"), "the link is vertically centered with the buttons");
    assert.ok(row.includes(":href=\"loginHelpUrl\""), "the link is bound to the help URL");
    assert.ok(row.includes("target=\"_blank\""), "the link opens in a new tab");
  }
});

test("no inline error alert remains in the dialogs", () => {
  const template = source.match(/<template>([\s\S]*?)<\/template>/)[1];

  assert.ok(!template.includes("alert-error"), "the pre-existing toast handles error display");
  assert.ok(!template.includes("loginErr"), "the error text is not rendered inline");
});
