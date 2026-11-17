import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

test("a second login challenge accepts a new 2FA code on the same socket", () => {
  const source = readFileSync(new URL("../src/components/Login.vue", import.meta.url), "utf8");
  const script = source.match(/<script>([\s\S]*?)<\/script>/)[1]
    .replace(/^import .*;$/gm, "")
    .replace("export default", "globalThis.component =");
  const context = vm.createContext({ HelpIcon: {} });
  vm.runInContext(script, context);

  const sent = [];
  const login = {
    ...context.component.data(),
    ...context.component.methods,
    $refs: { authForm: { checkValidity: () => true } },
    loginWebsock: { send: (message) => sent.push(JSON.parse(message)) },
  };

  login.loginWebsocketonmessage({ data: "Enter 2FA code" });
  login.authForm.authcode = "111111";
  login.onSubmit2FA();
  assert.equal(login.authLoading, true);

  login.loginWebsocketonmessage({ data: "Enter the code sent to your device" });
  assert.equal(login.authDialogVisible, true);
  assert.equal(login.authForm.authcode, "");
  assert.equal(login.authLoading, false, "the submit button must accept the second code");

  login.authForm.authcode = "222222";
  login.onSubmit2FA();
  assert.equal(login.authLoading, true);
  assert.deepEqual(sent, [{ t: 2, d: "111111" }, { t: 2, d: "222222" }]);
});
