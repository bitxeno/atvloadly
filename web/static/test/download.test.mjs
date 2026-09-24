import assert from "node:assert/strict";
import test from "node:test";

import {
  base64ToUint8Array,
  buildScreenshotFilename,
  dataUrlToBlob,
  downloadBlob,
  downloadDataUrl,
  downloadViaFormPost,
  needsServerDownload,
  splitDataUrl,
} from "../src/utils/download.mjs";

function fakeTimers() {
  const callbacks = [];
  const original = globalThis.setTimeout;
  globalThis.setTimeout = (fn) => {
    callbacks.push(fn);
    return callbacks.length;
  };
  return {
    callbacks,
    restore() {
      globalThis.setTimeout = original;
    },
  };
}

test("builds a timestamped screenshot filename", () => {
  const now = new Date(2026, 8, 24, 9, 5, 7);
  assert.equal(buildScreenshotFilename(now), "screenshot-20260924-090507.jpg");
});

test("parses only base64 image data URLs", () => {
  assert.deepEqual(splitDataUrl("data:image/jpeg;base64,aGk="), {
    mime: "image/jpeg",
    base64: "aGk=",
  });
  assert.equal(splitDataUrl("data:text/plain,hi"), null);
  assert.equal(splitDataUrl("data:image/png;base64,"), null);
  assert.equal(splitDataUrl("not-a-data-url"), null);
  assert.equal(splitDataUrl(null), null);
});

test("decodes base64 payloads without depending on atob", () => {
  const expected = [104, 105];
  const originalAtob = globalThis.atob;
  delete globalThis.atob;
  try {
    assert.deepEqual(Array.from(base64ToUint8Array("aGk=")), expected);
  } finally {
    if (originalAtob !== undefined) {
      globalThis.atob = originalAtob;
    }
  }
});

test("converts a data URL into a Blob with the embedded MIME type", () => {
  const timers = fakeTimers();
  const originalBlob = globalThis.Blob;
  try {
    const blob = dataUrlToBlob("data:image/jpeg;base64,aGk=");
    assert.ok(blob instanceof originalBlob);
    assert.equal(blob.type, "image/jpeg");
    assert.ok(blob.size > 0);
  } finally {
    timers.restore();
  }
});

test("rejects invalid data URLs before touching the DOM", () => {
  assert.throws(() => dataUrlToBlob("data:text/plain,hi"), /Invalid data URL/);
  assert.throws(() => downloadDataUrl("data:text/plain,hi", "x.jpg"), /Invalid data URL/);
});

test("downloads through an object URL and revokes it afterwards", () => {
  const timers = fakeTimers();
  const created = [];
  const revoked = [];
  const clicks = [];
  const link = {
    download: "",
    click: () => clicks.push(link.href),
    remove: () => {},
  };
  const document = {
    createElement: () => link,
    body: { appendChild: () => {}, removeChild: () => {} },
  };
  const URLImpl = {
    createObjectURL: (blob) => {
      created.push(blob);
      return "blob:fake";
    },
    revokeObjectURL: (url) => revoked.push(url),
  };
  try {
    const blob = new Blob(["hi"], { type: "image/jpeg" });
    assert.equal(downloadBlob(blob, "screenshot.jpg", { document, URL: URLImpl }), "downloaded");
    assert.equal(link.href, "blob:fake");
    assert.equal(link.download, "screenshot.jpg");
    assert.equal(clicks.length, 1);
    assert.deepEqual(created, [blob]);
    assert.equal(revoked.length, 0);
    timers.callbacks.forEach((fn) => fn());
    assert.deepEqual(revoked, ["blob:fake"]);
  } finally {
    timers.restore();
  }
});

test("opens a new tab when the browser has no download attribute", () => {
  const timers = fakeTimers();
  const opened = [];
  const link = {};
  const document = { createElement: () => link };
  const window = { open: (url) => opened.push(url) };
  const revoked = [];
  const URLImpl = {
    createObjectURL: () => "blob:fake",
    revokeObjectURL: (url) => revoked.push(url),
  };
  try {
    const blob = new Blob(["hi"], { type: "image/jpeg" });
    assert.equal(
      downloadBlob(blob, "screenshot.jpg", { document, window, URL: URLImpl }),
      "opened",
    );
    assert.deepEqual(opened, ["blob:fake"]);
  } finally {
    timers.restore();
  }
});

test("reports blocked when neither download nor a new tab is possible", () => {
  const link = {};
  const document = { createElement: () => link };
  const window = {
    open: () => {
      throw new Error("popup blocked");
    },
  };
  const created = [];
  const URLImpl = {
    createObjectURL: () => {
      created.push(true);
      return "blob:fake";
    },
    revokeObjectURL: () => {},
  };
  assert.equal(
    downloadBlob(new Blob(["hi"]), "screenshot.jpg", { document, window, URL: URLImpl }),
    "blocked",
  );
  assert.equal(created.length, 1);
});

test("reports unsupported without DOM object URL support", () => {
  assert.equal(
    downloadBlob(new Blob(["hi"]), "screenshot.jpg", { document: undefined, URL: undefined }),
    "unsupported",
  );
});

test("posts the preview and lets the browser save the attachment response", () => {
  const submitted = [];
  const appended = [];
  const document = {
    createElement: (tag) => {
      const element = {
        tag,
        style: {},
        attributes: {},
        children: [],
        appendChild: (child) => element.children.push(child),
        setAttribute: (name, value) => {
          element.attributes[name] = value;
        },
        remove: () => appended.splice(appended.indexOf(element), 1),
        submit: () => submitted.push(element),
      };
      return element;
    },
    body: { appendChild: (element) => appended.push(element) },
  };

  assert.equal(
    downloadViaFormPost(
      "/api/devices/screenshot/download",
      { data: "aGk=" },
      { document },
    ),
    "downloaded",
  );

  const form = submitted[0];
  assert.equal(form.method, "post");
  assert.equal(form.action, "/api/devices/screenshot/download");
  const input = form.children[0];
  assert.equal(input.type, "hidden");
  assert.equal(input.name, "data");
  assert.equal(input.value, "aGk=");
  // The submitted form stays attached: detaching it can abort the request.
  assert.deepEqual(appended, [form]);
});

test("removes stale download forms from earlier submissions", () => {
  const removed = [];
  const stale = [
    { remove: () => removed.push("stale-1") },
    { remove: () => removed.push("stale-2") },
  ];
  const document = {
    querySelectorAll: () => stale,
    createElement: () => ({
      style: {},
      appendChild: () => {},
      setAttribute: () => {},
      submit: () => {},
    }),
    body: { appendChild: () => {} },
  };

  assert.equal(downloadViaFormPost("/x", { data: "aGk=" }, { document }), "downloaded");
  assert.deepEqual(removed, ["stale-1", "stale-2"]);
});

test("reports unsupported or blocked when a form post is impossible", () => {
  assert.equal(downloadViaFormPost("", {}, {}), "unsupported");
  assert.equal(downloadViaFormPost("/x", {}, { document: {} }), "unsupported");
  assert.equal(
    downloadViaFormPost(
      "/x",
      { data: "aGk=" },
      {
        document: {
          createElement: () => ({
            style: {},
            appendChild: () => {},
            setAttribute: () => {},
            submit: () => {
              throw new Error("navigation denied");
            },
            parentNode: { removeChild: () => {} },
          }),
          body: { appendChild: () => {} },
        },
      },
    ),
    "blocked",
  );
});

test("keeps client-side downloads in regular desktop and mobile browsers", () => {
  const chromeDesktop =
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36";
  const safariDesktop =
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15";
  const androidChrome =
    "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Mobile Safari/537.36";

  assert.equal(needsServerDownload({ userAgent: chromeDesktop }), false);
  assert.equal(needsServerDownload({ userAgent: safariDesktop, maxTouchPoints: 0 }), false);
  assert.equal(needsServerDownload({ userAgent: androidChrome }), false);
  assert.equal(needsServerDownload({}), false);
});

test("routes iOS and embedded WebView downloads through the server", () => {
  const iphoneSafari =
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1";
  const ipadDesktopMode =
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15";
  const androidWebView =
    "Mozilla/5.0 (Linux; Android 14; Pixel 8 Build/AP1A; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/126.0 Mobile Safari/537.36";
  const wechat =
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 MicroMessenger/8.0.49";

  assert.equal(needsServerDownload({ userAgent: iphoneSafari }), true);
  assert.equal(needsServerDownload({ userAgent: ipadDesktopMode, maxTouchPoints: 5 }), true);
  assert.equal(needsServerDownload({ userAgent: androidWebView }), true);
  assert.equal(needsServerDownload({ userAgent: wechat }), true);
});
