// Guard against an IPA upload that stops making progress.
//
// Axios applies `timeout` to the whole request, not to a period without
// traffic, so a fixed value caps the upload at a fixed duration regardless of
// how fast the file transfers: a 23 GB IPA needs far longer than any timeout
// short enough to catch a dead connection. This guard watches upload progress
// instead. The request is aborted only when no byte moved for stallMs, so a
// slow but healthy upload of any size runs to completion while a connection
// that stopped delivering still fails.
//
// Free of Vue and DOM dependencies so node:test exercises it directly.

// Default time without upload progress after which the upload is abandoned.
// Generous enough for a browser to finish a buffered chunk and for a relayed
// connection to recover from a burst, short enough to fail a dead one before
// the user gives up on it.
export const defaultUploadStallMs = 5 * 60 * 1000;

// uploadRequestConfig returns the axios config of an IPA upload guarded by
// guard. timeout is 0 (axios creates no timer) so the file size alone decides
// how long the request may take; the guard's signal is what aborts it.
export function uploadRequestConfig(guard) {
  return {
    timeout: 0,
    signal: guard.signal,
    onUploadProgress: guard.onUploadProgress,
  };
}

// createUploadGuard returns the axios `signal` and `onUploadProgress` of one
// guarded upload, plus release() to stop the timer when the request is over.
// The returned signal aborts with an UploadStalledError so callers can tell a
// stalled upload apart from an abort caused by the user leaving the page.
// onProgress, when given, receives each progress event after the stall budget
// was re-armed, so a caller can report how far a long upload got.
export function createUploadGuard(stallMs = defaultUploadStallMs, onProgress = null, setTimeoutFn = setTimeout, clearTimeoutFn = clearTimeout) {
  const controller = new AbortController();
  let timer = null;
  let done = false;

  const arm = () => {
    if (timer !== null) {
      clearTimeoutFn(timer);
    }
    timer = setTimeoutFn(() => {
      timer = null;
      if (done) {
        return;
      }
      done = true;
      controller.abort(new UploadStalledError(stallMs));
    }, stallMs);
    // Node keeps the process alive for a pending timer; a page does not care.
    timer?.unref?.();
  };

  arm();

  return {
    signal: controller.signal,
    // onUploadProgress re-arms the guard: a byte reached the server, so the
    // upload is alive and the stall budget starts again.
    onUploadProgress: (event) => {
      if (done) {
        return;
      }
      arm();
      onProgress?.(event);
    },
    // release stops the timer; a finished request must not abort later.
    release: () => {
      done = true;
      if (timer !== null) {
        clearTimeoutFn(timer);
        timer = null;
      }
    },
    // stalled reports whether the guard aborted the request.
    stalled: () => controller.signal.aborted && controller.signal.reason instanceof UploadStalledError,
  };
}

// UploadStalledError marks the abort of an upload that stopped progressing.
export class UploadStalledError extends Error {
  constructor(stallMs) {
    super(`no upload progress for ${Math.round(stallMs / 1000)}s`);
    this.name = "UploadStalledError";
    this.stallMs = stallMs;
  }
}

// uploadStallMessage describes a stalled upload for the install log.
export function uploadStallMessage(err) {
  const stallMs = err?.stallMs ?? err?.reason?.stallMs ?? err?.cause?.stallMs;
  if (!stallMs) {
    return "the upload stopped making progress";
  }
  return `the upload stopped making progress for ${Math.round(stallMs / 1000)}s`;
}
