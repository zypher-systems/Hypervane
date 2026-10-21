import type { GuestRef } from "@/types";

const selected = new Map<string, GuestRef>();
const listeners = new Set<() => void>();

export function guestKey(g: GuestRef) {
  return `${g.node}:${g.type}:${g.vmid}`;
}

export function toggleGuest(g: GuestRef) {
  const k = guestKey(g);
  if (selected.has(k)) selected.delete(k);
  else selected.set(k, g);
  emit();
}

export function isSelected(g: GuestRef) {
  return selected.has(guestKey(g));
}

export function getSelection(): GuestRef[] {
  return [...selected.values()];
}

export function clearSelection() {
  selected.clear();
  emit();
}

export function subscribe(fn: () => void) {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

function emit() {
  listeners.forEach((fn) => fn());
}
