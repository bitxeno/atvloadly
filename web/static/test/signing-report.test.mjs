import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  createSigningReportStream,
  groupIssuesBySeverity,
  hasBlockingIssues,
  hasWaivableEntitlementIssues,
  parseSigningReportLine,
  signingCodeText,
  signingErrorOf,
  summarizePlan,
} from "../src/utils/signing-report.mjs";

function loadLocale(name) {
  return JSON.parse(
    readFileSync(new URL(`../../../locales/${name}.json`, import.meta.url), "utf8"),
  );
}

function translate(locale) {
  return (key) => key.split(".").reduce((value, part) => value?.[part], locale);
}

// backendCodes lists the stable codes of internal/signing/errors.go.
function backendCodes() {
  const source = readFileSync(
    new URL("../../../internal/signing/errors.go", import.meta.url),
    "utf8",
  );
  return [...source.matchAll(/^\s*Code\w+\s*=\s*"([a-z0-9_]+)"/gm)].map((match) => match[1]);
}

// Streamed plan of an IPA installed under a custom bundle identifier with
// its extension removed on request.
const planReport = {
  stage: "plan",
  blocking: false,
  issues: [
    { code: "bundle_identifier_rewritten", severity: "warning", message: "rewritten", bundle: "Payload/A.app" },
    { code: "extension_removed", severity: "warning", message: "removed", bundle: "Payload/A.app/PlugIns/W.appex" },
  ],
  main_bundle_id: "com.example.app",
  signed_main_bundle_id: "com.vendor.slot",
  custom_identifier: "com.vendor.slot",
  main_application_identifier: "ABCDE12345.com.vendor.slot",
  removed_bundles: ["Payload/A.app/PlugIns/W.appex"],
};

test("parses every report stage and rejects anything else", () => {
  const cases = [
    { line: `SIGNING_REPORT: ${JSON.stringify(planReport)}\n`, stage: "plan" },
    { line: 'SIGNING_REPORT: {"stage":"failure","class":"identity","code":"profile_expired","message":"expired"}', stage: "failure" },
    { line: '  SIGNING_REPORT: {"stage":"verified"}\r\n', stage: "verified" },
    { line: 'SIGNING_REPORT: {"stage":"unknown"}', stage: null },
    { line: "SIGNING_REPORT: {not json", stage: null },
    { line: 'SIGNING_REPORT: ["stage","plan"]', stage: null },
    { line: 'Installing SIGNING_REPORT: {"stage":"verified"}', stage: null },
    { line: "Installation Succeeded!", stage: null },
  ];
  for (const { line, stage } of cases) {
    assert.equal(parseSigningReportLine(line)?.stage ?? null, stage, line);
  }
});

test("normalizes report lists so consumers can iterate them", () => {
  const failure = parseSigningReportLine('SIGNING_REPORT: {"stage":"failure","code":"engine_failed","issues":null}');
  assert.deepEqual(failure.issues, []);

  const plan = parseSigningReportLine('SIGNING_REPORT: {"stage":"plan","issues":[{"code":"x"}]}');
  assert.deepEqual(plan.removed_bundles, []);
  assert.deepEqual(plan.issues, [{ code: "x" }]);
});

test("separates reports from log text across arbitrary chunk boundaries", () => {
  const verified = 'SIGNING_REPORT: {"stage":"verified"}\n';
  const plan = `SIGNING_REPORT: ${JSON.stringify(planReport)}\n`;
  const stream = [
    "Signing app...\n",
    plan,
    "Installing 50%\n",
    verified,
    "ERROR: SIGNING_REPORT: in the middle stays text\n",
    "Installation Succeeded!",
  ].join("");

  for (const size of [1, 3, 7, 16, 64, stream.length]) {
    const splitter = createSigningReportStream();
    let text = "";
    const stages = [];
    for (let offset = 0; offset < stream.length; offset += size) {
      const output = splitter.push(stream.slice(offset, offset + size));
      text += output.text;
      stages.push(...output.reports.map((report) => report.stage));
    }
    const rest = splitter.flush();
    text += rest.text;
    stages.push(...rest.reports.map((report) => report.stage));

    assert.deepEqual(stages, ["plan", "verified"], `chunk size ${size}`);
    assert.equal(
      text,
      "Signing app...\nInstalling 50%\nERROR: SIGNING_REPORT: in the middle stays text\nInstallation Succeeded!",
      `chunk size ${size}`,
    );
  }
});

test("releases ordinary text immediately and only holds possible report prefixes", () => {
  const splitter = createSigningReportStream();
  assert.deepEqual(splitter.push("Installation Failed!"), { text: "Installation Failed!", reports: [] });

  assert.deepEqual(splitter.push("SIGN"), { text: "", reports: [] });
  assert.deepEqual(splitter.push("ED IPA written\n"), { text: "SIGNED IPA written\n", reports: [] });

  assert.deepEqual(splitter.push("SIGNING_REPORT: {broken"), { text: "", reports: [] });
  assert.deepEqual(splitter.flush(), { text: "SIGNING_REPORT: {broken", reports: [] });
  assert.deepEqual(splitter.flush(), { text: "", reports: [] });
});

test("a report received as a whole message is emitted at once without a stray blank line", () => {
  const splitter = createSigningReportStream();
  const output = splitter.push('SIGNING_REPORT: {"stage":"verified"}');
  assert.deepEqual(output.reports.map((report) => report.stage), ["verified"]);
  assert.equal(output.text, "");

  assert.deepEqual(splitter.push("\n"), { text: "", reports: [] });
  assert.deepEqual(splitter.push("\n"), { text: "\n", reports: [] });
});

test("every backend signing code is translated in every locale", () => {
  const codes = backendCodes();
  assert.ok(codes.includes("revocation_not_checked"));
  for (const localeName of ["en", "sv", "zh_cn"]) {
    const message = translate(loadLocale(localeName));
    for (const code of codes) {
      const text = signingCodeText(code, "english fallback", message);
      assert.notEqual(text, "english fallback", `${localeName}: ${code}`);
      assert.equal(text, message(`signing.codes.${code}`), `${localeName}: ${code}`);
    }
  }
});

test("unknown codes fall back to the backend message, then to the code", () => {
  const message = translate(loadLocale("en"));
  const identity = (key) => key;
  assert.equal(signingCodeText("future_code", "Something new happened", message), "Something new happened");
  assert.equal(signingCodeText("future_code", "  ", identity), "future_code");
  assert.equal(signingCodeText("", "Plain API error", message), "Plain API error");
});

test("groups issues by severity, most severe first, keeping backend order", () => {
  const issues = [
    { code: "a", severity: "info" },
    { code: "b", severity: "warning" },
    { code: "c", severity: "error" },
    { code: "d", severity: "bogus" },
    { code: "e", severity: "error" },
  ];
  assert.deepEqual(
    groupIssuesBySeverity(issues).map((group) => [group.severity, group.issues.map((issue) => issue.code)]),
    [
      ["error", ["c", "e"]],
      ["warning", ["b", "d"]],
      ["info", ["a"]],
    ],
  );
  assert.deepEqual(groupIssuesBySeverity(null), []);
  assert.equal(hasBlockingIssues([{ severity: "bogus" }, { severity: "warning" }]), false);
  assert.equal(hasBlockingIssues([{ severity: "info" }, { severity: "error" }]), true);
});

test("offers the missing-entitlements waiver only for entitlement findings", () => {
  const cases = [
    { codes: ["entitlements_missing"], want: true },
    { codes: ["bundle_identifier_rewritten", "entitlement_value_changed"], want: true },
    { codes: ["wildcard_requires_binary_entitlements"], want: false },
    { codes: ["device_not_provisioned", "extension_removed"], want: false },
    { codes: [], want: false },
  ];
  for (const { codes, want } of cases) {
    const issues = codes.map((code) => ({ code, severity: "error" }));
    assert.equal(hasWaivableEntitlementIssues(issues), want, codes.join(","));
  }
});

test("summarizes the check endpoint plan and the streamed plan alike", () => {
  const checkPlan = {
    issues: planReport.issues,
    main_bundle_id: "com.example.app",
    signed_main_bundle_id: "com.vendor.slot",
    custom_identifier: "com.vendor.slot",
    bundles: [
      {
        path: "Payload/A.app",
        kind: "app",
        original_id: "com.example.app",
        signed_id: "com.vendor.slot",
        removed: false,
        expected_application_identifier: "ABCDE12345.com.vendor.slot",
      },
      {
        path: "Payload/A.app/PlugIns/W.appex",
        kind: "app_extension",
        original_id: "com.example.app.widget",
        signed_id: "com.vendor.slot.widget",
        removed: true,
      },
    ],
  };
  const fromCheck = summarizePlan(checkPlan, false);
  const fromReport = summarizePlan(parseSigningReportLine(`SIGNING_REPORT: ${JSON.stringify(planReport)}`), planReport.blocking);
  assert.deepEqual(fromCheck, fromReport);
  assert.equal(fromCheck.bundleIdRewritten, true);
  assert.equal(fromCheck.signedMainBundleId, "com.vendor.slot");
  assert.equal(fromCheck.mainApplicationIdentifier, "ABCDE12345.com.vendor.slot");
  assert.deepEqual(fromCheck.removedBundles, ["Payload/A.app/PlugIns/W.appex"]);
});

test("keeps the IPA identifiers by default and shows the main app application identifier", () => {
  // Prefix wildcard profile ABCDE12345.com.vendor.*: every bundle keeps its
  // identifier and is signed with the profile derived application identifier.
  const plan = {
    issues: [
      { code: "application_identifier_from_profile", severity: "warning", message: "profile", bundle: "Payload/A.app/PlugIns/W.appex" },
      { code: "application_identifier_from_profile", severity: "warning", message: "profile", bundle: "Payload/A.app" },
    ],
    main_bundle_id: "com.example.app",
    signed_main_bundle_id: "com.example.app",
    bundles: [
      {
        path: "Payload/A.app/PlugIns/W.appex",
        kind: "app_extension",
        original_id: "com.example.app.widget",
        signed_id: "com.example.app.widget",
        removed: false,
        expected_application_identifier: "ABCDE12345.com.vendor.com.example.app.widget",
      },
      {
        path: "Payload/A.app",
        kind: "app",
        original_id: "com.example.app",
        signed_id: "com.example.app",
        removed: false,
        expected_application_identifier: "ABCDE12345.com.vendor.com.example.app",
      },
    ],
  };
  const summary = summarizePlan(plan);
  assert.equal(summary.blocking, false);
  assert.equal(summary.bundleIdRewritten, false);
  assert.equal(summary.signedMainBundleId, "com.example.app");
  assert.equal(summary.mainApplicationIdentifier, "ABCDE12345.com.vendor.com.example.app");
  assert.deepEqual(summary.removedBundles, []);
  assert.deepEqual(
    groupIssuesBySeverity(summary.issues).map((group) => group.severity),
    ["warning"],
  );

  const invalidCustom = summarizePlan({
    issues: [{ code: "custom_identifier_invalid", severity: "error", message: "invalid" }],
    main_bundle_id: "com.example.app",
    signed_main_bundle_id: "com.example.app",
  });
  assert.equal(invalidCustom.blocking, true);
  assert.equal(invalidCustom.bundleIdRewritten, false);
});

test("plan blocking follows the server flag, else the issues", () => {
  const blockingIssue = [{ code: "device_not_provisioned", severity: "error" }];
  assert.equal(summarizePlan({ issues: blockingIssue }).blocking, true);
  assert.equal(summarizePlan({ issues: [] }).blocking, false);
  assert.equal(summarizePlan({ issues: [] }, true).blocking, true);
  assert.equal(summarizePlan({ issues: blockingIssue }, false).blocking, false);

  const unchanged = summarizePlan({ main_bundle_id: "com.example.app", signed_main_bundle_id: "" });
  assert.equal(unchanged.bundleIdRewritten, false);
  assert.equal(unchanged.signedMainBundleId, "com.example.app");
  assert.equal(unchanged.mainApplicationIdentifier, "");
});

test("extracts the signing error payload of a rejected API call", () => {
  const rejected = {
    result: {
      code: -1,
      msg: "signing identity 3 is used by 2 installed app(s)",
      data: { code: "identity_referenced", class: "identity", app_count: 2 },
    },
  };
  assert.deepEqual(signingErrorOf(rejected), {
    code: "identity_referenced",
    class: "identity",
    message: "signing identity 3 is used by 2 installed app(s)",
    issues: [],
    appCount: 2,
  });

  assert.equal(signingErrorOf({ result: { code: -1, msg: "Invalid argument", data: null } }), null);
  assert.equal(signingErrorOf(new Error("timeout of 5000ms exceeded")), null);
});
