import { getActiveConnection, onActiveConnection } from "@/lib/connection";
import { connectionTopic, live } from "@/lib/ws";
import { useEffect, useRef, useSyncExternalStore } from "react";

export function useActiveConnection() {
  return useSyncExternalStore(onActiveConnection, getActiveConnection);
}

/** Follows a detail topic such as `node:pve1` on the active connection. */
export function useLiveTopic(detail: string | null, onData: (data: unknown) => void) {
  const connection = useActiveConnection();
  const handler = useRef(onData);
  useEffect(() => {
    handler.current = onData;
  });
  useEffect(() => {
    if (!detail || !connection) return;
    const topic = connectionTopic(connection, detail);
    live.subscribe([topic]);
    const off = live.on((t, data) => {
      if (t === topic) handler.current(data);
    });
    return () => {
      off();
      live.unsubscribe([topic]);
    };
  }, [detail, connection]);
}
