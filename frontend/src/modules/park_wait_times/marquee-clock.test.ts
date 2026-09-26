import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/** The loop's lifecycle, which a render cannot see: a leaked frame loop looks like a stopped
    one. */

interface FakeFrames {
  readonly queued: Map<number, FrameRequestCallback>;
  readonly cancelled: number[];
  runOneFrame(nowMs?: number): void;
}

let frames: FakeFrames;
let realRaf: typeof globalThis.requestAnimationFrame;
let realCaf: typeof globalThis.cancelAnimationFrame;

function installFakeFrames(): FakeFrames {
  const queued = new Map<number, FrameRequestCallback>();
  const cancelled: number[] = [];
  let nextHandle = 1;

  globalThis.requestAnimationFrame = (callback: FrameRequestCallback): number => {
    const handle = nextHandle++;
    queued.set(handle, callback);
    return handle;
  };
  globalThis.cancelAnimationFrame = (handle: number): void => {
    cancelled.push(handle);
    queued.delete(handle);
  };

  return {
    queued,
    cancelled,
    runOneFrame(nowMs = performance.now()) {
      // One in flight at a time by construction: `step` requests the next only as it returns.
      const [handle, callback] = [...queued.entries()][0] ?? [];
      if (handle === undefined || callback === undefined) {
        throw new Error('no animation frame in flight');
      }
      queued.delete(handle);
      callback(nowMs);
    },
  };
}

/** A column stands for its own `scrollLeft` and nothing else — the one property the loop writes. */
function aColumn(): HTMLElement {
  return { scrollLeft: 0 } as HTMLElement;
}

async function freshClock(): Promise<typeof import('./marquee-clock')> {
  vi.resetModules();
  return import('./marquee-clock');
}

beforeEach(() => {
  realRaf = globalThis.requestAnimationFrame;
  realCaf = globalThis.cancelAnimationFrame;
  frames = installFakeFrames();
});

afterEach(() => {
  globalThis.requestAnimationFrame = realRaf;
  globalThis.cancelAnimationFrame = realCaf;
});

describe('registerMarquee', () => {
  it('starts the loop on the first column, and does not start a second for the next', async () => {
    const { registerMarquee } = await freshClock();

    registerMarquee(aColumn(), 100);
    expect(frames.queued.size, 'the first registration puts one frame in flight').toBe(1);

    registerMarquee(aColumn(), 100);
    expect(frames.queued.size, 'the second joins the loop already running rather than starting its own').toBe(1);
  });
});

describe('unregisterMarquee', () => {
  it('cancels the loop with the last column, leaving no frame callback running', async () => {
    const { registerMarquee, unregisterMarquee } = await freshClock();
    const column = aColumn();

    registerMarquee(column, 100);
    const inFlight = [...frames.queued.keys()][0];

    unregisterMarquee(column);

    expect(frames.cancelled, 'the frame the loop held is the one cancelled').toEqual([inFlight]);
    expect(frames.queued.size, 'and nothing is left in flight behind a torn-down placement').toBe(0);
  });

  it('leaves the loop running while any other column is still registered', async () => {
    const { registerMarquee, unregisterMarquee } = await freshClock();
    const staying = aColumn();
    const leaving = aColumn();
    registerMarquee(staying, 100);
    registerMarquee(leaving, 100);

    unregisterMarquee(leaving);

    expect(frames.cancelled, 'no frame is cancelled while a column still needs one').toEqual([]);
    expect(frames.queued.size, 'the loop is still in flight').toBe(1);
  });

  it('returns a dropped column home', async () => {
    const { registerMarquee, unregisterMarquee } = await freshClock();
    const column = aColumn();
    registerMarquee(column, 100);
    frames.runOneFrame(performance.now() + 4000);
    expect(column.scrollLeft, 'the column has left home before it is dropped').toBeGreaterThan(0);

    unregisterMarquee(column);

    expect(column.scrollLeft, 'a dropped column is put back rather than left mid-scroll').toBe(0);
  });

  it('does nothing for a column it never held, the loop included', async () => {
    const { registerMarquee, unregisterMarquee } = await freshClock();
    registerMarquee(aColumn(), 100);

    unregisterMarquee(aColumn());

    expect(frames.cancelled, 'an unheld column cancels nothing').toEqual([]);
    expect(frames.queued.size, 'and leaves the loop in flight for the column that is held').toBe(1);
  });

  it('starts a fresh loop when a column registers after the last one left', async () => {
    const { registerMarquee, unregisterMarquee } = await freshClock();
    const first = aColumn();
    registerMarquee(first, 100);
    unregisterMarquee(first);
    expect(frames.queued.size).toBe(0);

    const second = aColumn();
    registerMarquee(second, 100);

    expect(frames.queued.size, 'the loop runs again for a column registered after the teardown').toBe(1);
  });
});
