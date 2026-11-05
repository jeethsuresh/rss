/** Browsing destinations only: restoring one never invokes an application command. */
export class DestinationHistory<T> {
  private past: T[] = [];
  private future: T[] = [];
  constructor(public current: T, private readonly limit = 64) {}
  get canBack() { return this.past.length > 0; }
  get canForward() { return this.future.length > 0; }
  visit(next: T) {
    if (JSON.stringify(next) === JSON.stringify(this.current)) return;
    this.past.push(this.current);
    if (this.past.length > this.limit) this.past.shift();
    this.current = next;
    this.future = [];
  }
  refresh(next: T) { this.current = next; }
  back(): T | undefined {
    if (!this.canBack) return;
    this.future.push(this.current);
    return this.current = this.past.pop()!;
  }
  forward(): T | undefined {
    if (!this.canForward) return;
    this.past.push(this.current);
    return this.current = this.future.pop()!;
  }
  reset(next: T) { this.past = []; this.future = []; this.current = next; }
}
