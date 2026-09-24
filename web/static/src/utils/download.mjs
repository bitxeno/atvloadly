function getGlobal(name) {
  return typeof globalThis !== "undefined" ? globalThis[name] : undefined;
}

export function buildScreenshotFilename(now = new Date()) {
  const pad = (n) => String(n).padStart(2, "0");
  return (
    `screenshot-${now.getFullYear()}` +
    `${pad(now.getMonth() + 1)}${pad(now.getDate())}` +
    `-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}.jpg`
  );
}

export function splitDataUrl(dataUrl) {
  if (typeof dataUrl !== "string") {
    return null;
  }
  const comma = dataUrl.indexOf(",");
  if (!dataUrl.startsWith("data:") || comma === -1) {
    return null;
  }
  const meta = dataUrl.slice(5, comma);
  const base64 = dataUrl.slice(comma + 1);
  if (!/;base64$/i.test(meta) || !base64) {
    return null;
  }
  const mime = meta.slice(0, meta.length - ";base64".length) || "application/octet-stream";
  return { mime, base64 };
}

export function base64ToUint8Array(base64) {
  const clean = String(base64 ?? "").replace(/\s/g, "");
  if (!clean) {
    return new Uint8Array(0);
  }
  const decoder = getGlobal("atob");
  if (typeof decoder === "function") {
    const binary = decoder(clean);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  }
  const BufferImpl = getGlobal("Buffer");
  if (typeof BufferImpl?.from === "function") {
    return new Uint8Array(BufferImpl.from(clean, "base64"));
  }
  throw new Error("No base64 decoder available");
}

export function dataUrlToBlob(dataUrl) {
  const parsed = splitDataUrl(dataUrl);
  if (!parsed) {
    throw new Error("Invalid data URL");
  }
  const BlobImpl = getGlobal("Blob");
  if (typeof BlobImpl !== "function") {
    throw new Error("Blob is not supported");
  }
  return new BlobImpl([base64ToUint8Array(parsed.base64)], { type: parsed.mime });
}

function scheduleRevoke(URLImpl, url, delay) {
  const revoke = () => {
    try {
      URLImpl.revokeObjectURL(url);
    } catch {
      // Ignore revoke failures: the download has already started.
    }
  };
  if (typeof setTimeout === "function") {
    setTimeout(revoke, delay);
  } else {
    revoke();
  }
}

// Download a Blob via an object URL instead of a data URL. Data URLs combined
// with the anchor `download` attribute are ignored by Safari/iOS and hit URL
// length limits in other browsers, so they often fail without any prompt.
// Blob object URLs are same-origin and trigger the download prompt reliably.
// Note: blob: URLs still fail inside some iOS in-app WebViews, which only
// download from real http(s) responses. Prefer downloadViaFormPost() there.
export function downloadBlob(blob, filename, adapters = {}) {
  const doc = adapters.document ?? getGlobal("document");
  const win = adapters.window ?? getGlobal("window") ?? getGlobal("globalThis");
  const URLImpl = adapters.URL ?? getGlobal("URL");

  if (!doc || typeof doc.createElement !== "function" || !URLImpl?.createObjectURL) {
    return "unsupported";
  }

  const url = URLImpl.createObjectURL(blob);
  const openNewTab = () => {
    if (!win || typeof win.open !== "function") {
      return null;
    }
    try {
      return win.open(url, "_blank", "noopener");
    } catch {
      return null;
    }
  };

  let link;
  try {
    link = doc.createElement("a");
  } catch {
    return "unsupported";
  }

  // Browsers without `download` support (notably iOS Safari) ignore the
  // attribute and would navigate away, so open the image for manual saving.
  if (!("download" in link)) {
    const opened = openNewTab();
    scheduleRevoke(URLImpl, url, opened ? 60000 : 4000);
    return opened ? "opened" : "blocked";
  }

  try {
    link.href = url;
    link.download = filename;
    link.rel = "noopener";
    if (doc.body?.appendChild) {
      doc.body.appendChild(link);
    }
    link.click();
    if (typeof link.remove === "function") {
      link.remove();
    } else if (link.parentNode?.removeChild) {
      link.parentNode.removeChild(link);
    }
    scheduleRevoke(URLImpl, url, 4000);
    return "downloaded";
  } catch {
    const opened = openNewTab();
    scheduleRevoke(URLImpl, url, opened ? 60000 : 4000);
    return opened ? "opened" : "blocked";
  }
}

export function downloadDataUrl(dataUrl, filename, adapters = {}) {
  return downloadBlob(dataUrlToBlob(dataUrl), filename, adapters);
}

// Embedded browsers hand blob:/data: URL downloads to the host app, which
// merely offers to "open an external app" instead of saving the file. These
// markers cover the common in-app WebViews (iOS WKWebView and Android WebView
// plus the usual social apps) that cannot save a client-side download.
const embeddedBrowserPatterns = [
  /\bwv\b/, // Android WebView
  /FBAN|FBAV/,
  /Instagram/,
  /MicroMessenger/, // WeChat
  /Line\//,
  /Twitter/,
  /QQ\/|QQBrowser/,
  /Weibo/,
  /AlipayClient/,
  /DingTalk/,
  /Electron/,
  /GSA\//, // Google app
];

// needsServerDownload reports whether the download must go through the server.
// iOS ignores client-side downloads in most embedded browsers (and in-app
// WebViews cannot save them at all), so those clients upload the preview and
// navigate to a real attachment URL instead.
export function needsServerDownload({
  userAgent = "",
  maxTouchPoints = 0,
} = {}) {
  const ua = String(userAgent);
  if (embeddedBrowserPatterns.some((pattern) => pattern.test(ua))) {
    return true;
  }
  if (/iPad|iPhone|iPod/.test(ua)) {
    return true;
  }
  // iPadOS reports a desktop Safari user agent, distinguishable only by touch.
  return /Macintosh/.test(ua) && Number(maxTouchPoints) > 1;
}

// Post form fields to a URL and let the browser save the response. A form
// submit is a real top-level navigation, so the server can answer with
// Content-Disposition: attachment and the browser downloads the file without
// leaving the page. This is the only shape in-app WebViews accept, since they
// cannot save a client-side blob:/data: download.
const downloadFormMarker = "data-atv-download-form";

export function downloadViaFormPost(url, fields = {}, adapters = {}) {
  if (typeof url !== "string" || !url) {
    return "unsupported";
  }
  const doc = adapters.document ?? getGlobal("document");
  if (!doc || typeof doc.createElement !== "function" || !doc.body?.appendChild) {
    return "unsupported";
  }

  // Remove forms from earlier downloads. The form that just submitted is left
  // in place: detaching it immediately can abort the in-flight request.
  try {
    const stale = doc.querySelectorAll?.(`[${downloadFormMarker}]`) ?? [];
    for (const element of stale) {
      element.remove?.();
    }
  } catch {
    // Cleanup is best effort; a leftover hidden form is harmless.
  }

  let form;
  try {
    form = doc.createElement("form");
  } catch {
    return "unsupported";
  }

  try {
    form.method = "post";
    form.action = url;
    form.style.display = "none";
    form.setAttribute?.(downloadFormMarker, "");
    for (const [name, value] of Object.entries(fields)) {
      const input = doc.createElement("input");
      input.type = "hidden";
      input.name = name;
      input.value = String(value ?? "");
      form.appendChild(input);
    }
    doc.body.appendChild(form);
    form.submit();
    return "downloaded";
  } catch {
    try {
      form.parentNode?.removeChild?.(form);
    } catch {
      // Ignore cleanup failures; the submission already failed.
    }
    return "blocked";
  }
}
