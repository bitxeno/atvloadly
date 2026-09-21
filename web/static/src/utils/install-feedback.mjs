const knownAccountStatuses = new Set(["valid", "invalid"]);

export function accountStatusLabel(status, translate) {
  const value = String(status ?? "").trim();
  const key = value.toLowerCase();
  if (knownAccountStatuses.has(key)) {
    return translate(`account.status_labels.${key}`);
  }
  return value || "—";
}
