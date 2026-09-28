// Presentation helpers of the external certificate signing mode: parsing of
// the structured SIGNING_REPORT lines streamed by /ws/install, translation of
// the stable signing codes and grouping of compatibility issues. Free of Vue
// and DOM dependencies so node:test exercises them directly.

export const signingReportPrefix = "SIGNING_REPORT:";

const reportStages = new Set(["plan", "failure", "verified"]);
const severityOrder = ["error", "warning", "info"];

// Codes the user may accept with allow_missing_entitlements; the backend
// downgrades them from error to warning when the option is set.
const waivableEntitlementCodes = new Set([
  "entitlements_missing",
  "entitlement_value_changed",
]);

// Codes describing whether the target device and the IPA platform are
// covered by the provisioning profile.
const deviceCompatibilityCodes = new Set([
  "device_platform_unknown",
  "device_unreachable",
  "device_not_provisioned",
  "profile_platform_mismatch",
  "ipa_platform_mismatch",
  "ipa_platform_unknown",
]);

// A line without newline longer than this is not a report being received in
// pieces; it is released to the log instead of being buffered further.
const maxPendingLength = 1 << 20;

// parseSigningReportLine returns the report carried by one stream line, or
// null when the line is not a well-formed SIGNING_REPORT of a known stage.
export function parseSigningReportLine(line) {
  const text = String(line ?? "").trim();
  if (!text.startsWith(signingReportPrefix)) {
    return null;
  }

  let value;
  try {
    value = JSON.parse(text.slice(signingReportPrefix.length));
  } catch {
    return null;
  }
  if (!value || typeof value !== "object" || !reportStages.has(value.stage)) {
    return null;
  }

  const report = { ...value, issues: Array.isArray(value.issues) ? value.issues : [] };
  if (report.stage === "plan") {
    report.removed_bundles = Array.isArray(value.removed_bundles) ? value.removed_bundles : [];
  }
  return report;
}

// createSigningReportStream splits the install stream into log text and
// structured reports. push(chunk) returns the text to append to the log and
// the reports completed by the chunk; a report line may span several chunks.
// A report is emitted as soon as its JSON is complete, and the line end that
// follows it in a later chunk is dropped. flush() releases whatever is still
// buffered once the stream is over.
export function createSigningReportStream() {
  let pending = "";
  // Set when a report was emitted before its line end arrived.
  let awaitingLineEnd = false;

  function mayBecomeReport(tail) {
    const head = tail.trimStart();
    if (head === "") {
      return false;
    }
    return head.startsWith(signingReportPrefix) || signingReportPrefix.startsWith(head);
  }

  return {
    push(chunk) {
      let incoming = String(chunk ?? "");
      if (awaitingLineEnd && incoming !== "") {
        if (incoming === "\r") {
          return { text: "", reports: [] };
        }
        incoming = incoming.replace(/^\r?\n/, "");
        awaitingLineEnd = false;
      }

      const data = pending + incoming;
      pending = "";
      const reports = [];
      let text = "";
      let start = 0;

      for (let end = data.indexOf("\n", start); end !== -1; end = data.indexOf("\n", start)) {
        const line = data.slice(start, end + 1);
        const report = parseSigningReportLine(line);
        if (report) {
          reports.push(report);
        } else {
          text += line;
        }
        start = end + 1;
      }

      const tail = data.slice(start);
      if (tail !== "") {
        const report = parseSigningReportLine(tail);
        if (report) {
          reports.push(report);
          awaitingLineEnd = true;
        } else if (mayBecomeReport(tail) && tail.length < maxPendingLength) {
          pending = tail;
        } else {
          text += tail;
        }
      }

      return { text, reports };
    },

    flush() {
      const tail = pending;
      pending = "";
      awaitingLineEnd = false;
      const report = parseSigningReportLine(tail);
      return report ? { text: "", reports: [report] } : { text: tail, reports: [] };
    },
  };
}

// signingCodeText returns the translated text of a signing code, falling
// back to the English backend message (then the code itself) when the code
// has no translation. translate(key) may return undefined or the key itself
// for missing keys.
export function signingCodeText(code, message, translate) {
  if (code) {
    const key = `signing.codes.${code}`;
    const text = translate(key);
    if (typeof text === "string" && text !== "" && text !== key) {
      return text;
    }
  }
  return String(message ?? "").trim() || String(code ?? "");
}

export function issueText(issue, translate) {
  return signingCodeText(issue?.code, issue?.message, translate);
}

// normalizeSeverity maps unknown severities to "warning" so that an
// unexpected value is never presented as purely informational.
export function normalizeSeverity(severity) {
  return severityOrder.includes(severity) ? severity : "warning";
}

// groupIssuesBySeverity returns the non-empty severity groups ordered
// error, warning, info, keeping the backend order inside each group.
export function groupIssuesBySeverity(issues) {
  const groups = new Map(severityOrder.map((severity) => [severity, []]));
  for (const issue of issues ?? []) {
    groups.get(normalizeSeverity(issue?.severity)).push(issue);
  }
  return severityOrder
    .filter((severity) => groups.get(severity).length > 0)
    .map((severity) => ({ severity, issues: groups.get(severity) }));
}

export function hasBlockingIssues(issues) {
  return (issues ?? []).some((issue) => normalizeSeverity(issue?.severity) === "error");
}

// hasWaivableEntitlementIssues reports whether the "allow missing
// entitlements" choice applies to the issues.
export function hasWaivableEntitlementIssues(issues) {
  return (issues ?? []).some((issue) => waivableEntitlementCodes.has(issue?.code));
}

export function deviceCompatibilityIssues(issues) {
  return (issues ?? []).filter((issue) => deviceCompatibilityCodes.has(issue?.code));
}

// mainApplicationIdentifierOf returns the application identifier the main
// app will be signed with: the main_application_identifier of the streamed
// plan report, else the expected_application_identifier of the check plan
// bundle whose original identifier is the IPA main bundle identifier.
function mainApplicationIdentifierOf(plan, mainBundleId) {
  if (typeof plan?.main_application_identifier === "string") {
    return plan.main_application_identifier;
  }
  if (!Array.isArray(plan?.bundles) || mainBundleId === "") {
    return "";
  }
  const main = plan.bundles.find((bundle) => bundle?.original_id === mainBundleId && !bundle?.removed);
  return main?.expected_application_identifier || "";
}

// summarizePlan normalizes both the check endpoint plan ({issues, bundles,
// ...}) and the "plan" SIGNING_REPORT ({issues, removed_bundles, ...}).
// blocking overrides the value derived from the issues when it is a boolean.
// The bundle identifiers are kept unless the user set a custom identifier,
// in which case signedMainBundleId differs from mainBundleId.
export function summarizePlan(plan, blocking) {
  const issues = Array.isArray(plan?.issues) ? plan.issues : [];
  let removedBundles = [];
  if (Array.isArray(plan?.removed_bundles)) {
    removedBundles = plan.removed_bundles;
  } else if (Array.isArray(plan?.bundles)) {
    removedBundles = plan.bundles.filter((bundle) => bundle?.removed).map((bundle) => bundle.path);
  }
  const mainBundleId = plan?.main_bundle_id || "";
  const signedMainBundleId = plan?.signed_main_bundle_id || mainBundleId;

  return {
    issues,
    blocking: typeof blocking === "boolean" ? blocking : hasBlockingIssues(issues),
    mainBundleId,
    signedMainBundleId,
    bundleIdRewritten: signedMainBundleId !== "" && signedMainBundleId !== mainBundleId,
    mainApplicationIdentifier: mainApplicationIdentifierOf(plan, mainBundleId),
    removedBundles,
  };
}

// signingErrorOf extracts the signing error payload of a rejected API call
// (see utils/request.js), or returns null for other failures.
export function signingErrorOf(error) {
  const result = error?.result;
  const data = result?.data;
  if (!data || typeof data !== "object" || typeof data.code !== "string" || data.code === "") {
    return null;
  }
  return {
    code: data.code,
    class: typeof data.class === "string" ? data.class : "",
    message: String(result.msg ?? ""),
    issues: Array.isArray(data.issues) ? data.issues : [],
    appCount: Number(data.app_count) || 0,
  };
}

// requestErrorOf normalizes any rejected API call for display: the signing
// error payload when present, else the API or transport message.
export function requestErrorOf(error) {
  return signingErrorOf(error) || {
    code: "",
    class: "",
    message: String(error?.result?.msg || error?.message || ""),
    issues: [],
    appCount: 0,
  };
}

// shortFingerprint abbreviates a hex certificate fingerprint for display.
export function shortFingerprint(fingerprint) {
  const value = String(fingerprint ?? "").toUpperCase();
  return value.length > 16 ? `${value.slice(0, 8)}…${value.slice(-8)}` : value;
}
