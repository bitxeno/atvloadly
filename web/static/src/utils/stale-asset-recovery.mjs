const staleAssetErrorPattern =
  /failed to fetch dynamically imported module|error loading dynamically imported module|importing a module script failed|unable to preload css/i;

const recoveryKey = "atvloadly:stale-asset-recovery";

export function currentBundleURL(document, location) {
  return document.querySelector('script[type="module"][src]')?.src || location.href;
}

export function isStaleAssetLoadError(error) {
  const message = error instanceof Error ? error.message : String(error ?? "");
  return staleAssetErrorPattern.test(message);
}

export function recoverFromStaleAssetLoadError(
  error,
  bundleURL,
  storage,
  reload,
) {
  if (!isStaleAssetLoadError(error) || typeof bundleURL !== "string" || !bundleURL) {
    return false;
  }

  try {
    if (storage.getItem(recoveryKey) === bundleURL) {
      return false;
    }
    storage.setItem(recoveryKey, bundleURL);
  } catch {
    return false;
  }

  reload();
  return true;
}
