import assert from "node:assert/strict";
import test from "node:test";

import {
  UploadStalledError,
  createUploadGuard,
  defaultUploadStallMs,
  uploadRequestConfig,
  uploadStallMessage,
} from "../src/utils/upload-guard.mjs";

// A controllable stand-in for setTimeout/clearTimeout so the tests drive the
// stall clock instead of waiting on it.
function fakeClock() {
  let nextId = 1;
  const timers = new Map();
  return {
    setTimeout: (fn, ms) => {
      const id = nextId++;
      timers.set(id, { fn, ms });
      return { id, unref() {} };
    },
    clearTimeout: (timer) => {
      timers.delete(timer.id);
    },
    // fire runs the pending timers, as if their delay elapsed.
    fire() {
      const pending = [...timers.values()];
      timers.clear();
      for (const timer of pending) {
        timer.fn();
      }
    },
    get pending() {
      return timers.size;
    },
  };
}

test("a slow upload that keeps reporting progress is never aborted", () => {
  const clock = fakeClock();
  const guard = createUploadGuard(60_000, null, clock.setTimeout, clock.clearTimeout);

  // Far more elapsed time than the stall budget, but bytes keep moving.
  for (let i = 0; i < 100; i++) {
    guard.onUploadProgress({ loaded: i * 1024, total: 100 * 1024 });
  }
  assert.equal(guard.signal.aborted, false);

  // The timer was re-armed by each progress event, so nothing fires early.
  assert.equal(clock.pending, 1);
  guard.release();
  assert.equal(clock.pending, 0);
  assert.equal(guard.signal.aborted, false);
});

test("an upload that stops delivering bytes is aborted with a stall", () => {
  const clock = fakeClock();
  const guard = createUploadGuard(60_000, null, clock.setTimeout, clock.clearTimeout);

  guard.onUploadProgress({ loaded: 1, total: 2 });
  clock.fire();

  assert.equal(guard.signal.aborted, true);
  assert.equal(guard.stalled(), true);
  assert.ok(guard.signal.reason instanceof UploadStalledError);
  assert.equal(guard.signal.reason.stallMs, 60_000);
});

test("release stops the stall timer so a finished upload cannot abort later", () => {
  const clock = fakeClock();
  const guard = createUploadGuard(60_000, null, clock.setTimeout, clock.clearTimeout);

  guard.release();
  assert.equal(clock.pending, 0);
  clock.fire();
  assert.equal(guard.signal.aborted, false);
  assert.equal(guard.stalled(), false);
});

test("progress after the stall was detected does not resurrect the request", () => {
  const clock = fakeClock();
  const guard = createUploadGuard(60_000, null, clock.setTimeout, clock.clearTimeout);

  clock.fire();
  assert.equal(guard.stalled(), true);
  guard.onUploadProgress({ loaded: 5, total: 10 });
  assert.equal(guard.stalled(), true);
  assert.equal(clock.pending, 0);
});

test("the default stall budget is long enough for a large healthy upload", () => {
  assert.equal(defaultUploadStallMs, 5 * 60 * 1000);
});

test("describes a stall in seconds for the install log", () => {
  assert.equal(
    uploadStallMessage(new UploadStalledError(300_000)),
    "the upload stopped making progress for 300s",
  );
  assert.equal(uploadStallMessage({ reason: new UploadStalledError(60_000) }),
    "the upload stopped making progress for 60s");
  assert.equal(uploadStallMessage(new Error("other")), "the upload stopped making progress");
});

test("the upload request has no fixed timeout and carries the guard", () => {
  const clock = fakeClock();
  const guard = createUploadGuard(60_000, null, clock.setTimeout, clock.clearTimeout);
  const config = uploadRequestConfig(guard);

  // A fixed axios timeout would cap a 23 GB upload at a fixed duration; 0
  // makes axios create no timer at all, leaving the guard to abort a stall.
  assert.equal(config.timeout, 0);
  assert.equal(config.signal, guard.signal);
  assert.equal(typeof config.onUploadProgress, "function");
  guard.release();
});

test("progress events reach the caller so a long upload can be reported", () => {
  const clock = fakeClock();
  const seen = [];
  const guard = createUploadGuard(60_000, (event) => seen.push(event.loaded), clock.setTimeout, clock.clearTimeout);

  guard.onUploadProgress({ loaded: 10, total: 100 });
  guard.onUploadProgress({ loaded: 20, total: 100 });
  assert.deepEqual(seen, [10, 20]);

  // After the stall was detected the caller hears nothing more: the request
  // is already being aborted.
  clock.fire();
  guard.onUploadProgress({ loaded: 30, total: 100 });
  assert.deepEqual(seen, [10, 20]);
});
