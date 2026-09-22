export function installFailureMessage(output, translate) {
  const log = String(output ?? "");
  if (
    log.includes("An Application Group with Identifier") &&
    log.includes("is not available")
  ) {
    return translate("install.toast.app_group_unavailable");
  }
  return translate("install.toast.install_failed");
}
