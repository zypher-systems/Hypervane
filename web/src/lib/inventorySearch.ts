let query = "";
const listeners = new Set<(q: string) => void>();

export function getInventorySearch() {
  return query;
}

export function setInventorySearch(next: string) {
  query = next;
  listeners.forEach((fn) => fn(query));
}

export function onInventorySearch(fn: (q: string) => void) {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

export function focusInventorySearch() {
  window.dispatchEvent(new CustomEvent("hypervane:search"));
}
