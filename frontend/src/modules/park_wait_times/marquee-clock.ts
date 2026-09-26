/**
 * The placement's one marquee loop and cycle clock (./README.md § The grid — constant card,
 * configured shape). A frame writes `scrollLeft` and reads no geometry; each column's distance is
 * cached at registration.
 */

/** px/s, shared by every row. */
export const MARQUEE_PX_PER_S = 30;

/** Seconds held at home before the scroll (./README.md § The grid — constant card, configured
    shape). */
export const HOLD_HOME_S = 2;

/** Seconds held at the end before returning home. */
export const HOLD_END_S = 2;

/** Column → scroll distance (`scrollWidth - clientWidth`), measured at registration. */
const columns = new Map<HTMLElement, number>();

let frame: number | null = null;

let cycleStartMs = 0;

/**
 * One frame: hold home, scroll at `MARQUEE_PX_PER_S`, hold at the end, then home until
 * `startMarqueeCycle`. Reach, the distance travelled before the rotation tick, is
 * `(rotation_interval_seconds - HOLD_HOME_S) × MARQUEE_PX_PER_S`; a longer name is cut short.
 */
function step(nowMs: number): void {
  // Floored at zero: a cycle reset timestamped inside an interval callback can sit later than the
  // frame timestamp of the frame already in flight, which would otherwise read as negative elapsed.
  const elapsed = Math.max(0, (nowMs - cycleStartMs) / 1000);
  for (const [column, distance] of columns) {
    const moveTime = distance / MARQUEE_PX_PER_S;
    column.scrollLeft =
      elapsed < HOLD_HOME_S || elapsed >= HOLD_HOME_S + moveTime + HOLD_END_S
        ? 0
        : Math.min((elapsed - HOLD_HOME_S) * MARQUEE_PX_PER_S, distance);
  }
  frame = requestAnimationFrame(step);
}

export function startMarqueeCycle(): void {
  cycleStartMs = performance.now();
}

/** Registers or re-measures a column; one joining mid-loop takes the cycle in flight. */
export function registerMarquee(column: HTMLElement, distance: number): void {
  columns.set(column, distance);
  if (frame !== null) return;
  // A loop starting from empty starts a cycle, not a stale one left by the last teardown.
  cycleStartMs = performance.now();
  frame = requestAnimationFrame(step);
}

export function unregisterMarquee(column: HTMLElement): void {
  if (!columns.delete(column)) return;
  column.scrollLeft = 0;
  if (columns.size === 0 && frame !== null) {
    cancelAnimationFrame(frame);
    frame = null;
  }
}
