import type { ApiError } from "./public";

type CursorPage = { nextCursor?: string };
type Notice = "updated" | "restarted";
type PagerState<P> = { data?: P; error?: ApiError; loading: boolean; page: number; fetchedAt?: number; notice?: Notice };
const snapshotError = (error: unknown) => {
  const code = (error as ApiError)?.code;
  return code === "CONFLICT" || code === "CHAIN_REORGED";
};

// Page numbers and cursor history commit together, only after a valid response.
// Snapshot recovery uses new authenticated cursors; it never mixes old boundaries.
export class CursorPager<P extends CursorPage> {
  private state: PagerState<P> = { loading: true, page: 1 };
  private cursors = [""];
  private controller?: AbortController;
  private listeners = new Set<() => void>();
  private retryTarget = 0;
  constructor(private readonly load: (cursor: string, signal: AbortSignal) => Promise<P>) {}
  getSnapshot = () => this.state;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  };
  private update(value: Partial<PagerState<P>>) {
    this.state = { ...this.state, ...value };
    this.listeners.forEach(listener => listener());
  }
  start = () => this.run([""], 0);
  stop = () => { this.controller?.abort(); this.controller = undefined; };
  next = async () => {
    if (!this.state.data?.nextCursor || this.state.error) return;
    await this.run([...this.cursors, this.state.data.nextCursor], this.cursors.length);
  };
  previous = async () => {
    if (this.cursors.length < 2) return;
    await this.run(this.cursors.slice(0, -1), this.cursors.length - 2);
  };
  retry = () => this.run([""], this.retryTarget, true);

  private async rebuild(target: number, signal: AbortSignal, reorg = false) {
    // Avoid unbounded replay on long browsing sessions. The user can continue
    // normally from a fresh first page when more than ten pages would be needed.
    let wanted = reorg || target >= 10 ? 0 : target;
    let notice: Notice = wanted === target && !reorg ? "updated" : "restarted";
    for (let attempt = 0; attempt < 2; attempt++) {
      const cursors = [""];
      try {
        for (let index = 0; index <= wanted; index++) {
          const data = await this.load(cursors[index], signal);
          if (signal.aborted) throw new DOMException("Aborted", "AbortError");
          if (index === wanted || !data.nextCursor) return { data, cursors, notice };
          cursors.push(data.nextCursor);
        }
      } catch (error) {
        if (signal.aborted || !snapshotError(error) || attempt === 1) throw error;
        if ((error as ApiError).code === "CHAIN_REORGED") { wanted = 0; notice = "restarted"; }
      }
    }
    throw new Error("Pagination recovery exhausted");
  }

  private async run(cursors: string[], target: number, fresh = false) {
    if (this.controller) return;
    const controller = new AbortController();
    this.controller = controller;
    this.retryTarget = target;
    this.update({ loading: true, error: undefined, notice: undefined });
    try {
      let result: { data: P; cursors: string[]; notice?: Notice };
      if (fresh) result = await this.rebuild(target, controller.signal);
      else {
        try { result = { data: await this.load(cursors[target], controller.signal), cursors }; }
        catch (error) {
          if (!snapshotError(error) || controller.signal.aborted) throw error;
          result = await this.rebuild(target, controller.signal, (error as ApiError).code === "CHAIN_REORGED");
        }
      }
      if (controller.signal.aborted || this.controller !== controller) return;
      this.cursors = result.cursors;
      this.retryTarget = this.cursors.length - 1;
      this.update({ data: result.data, page: this.cursors.length, loading: false, error: undefined, fetchedAt: Date.now(), notice: result.notice });
    } catch (error) {
      if (!controller.signal.aborted && this.controller === controller) {
        this.update({ loading: false, error: error as ApiError });
      }
    } finally {
      if (this.controller === controller) this.controller = undefined;
    }
  }
}
