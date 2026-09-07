import { useEffect, useState } from "react";

import { Events } from "@wailsio/runtime";

/** Subscribes to the `time` event that main.go emits every second. */
export function useGoClock(): string {
  const [time, setTime] = useState("waiting for Go…");

  useEffect(() => {
    // Events.On returns its own unsubscribe function; returning it stops a
    // second listener stacking up on every hot reload.
    const off = Events.On("time", (event: { data: string }) => {
      setTime(event.data);
    });

    return off;
  }, []);

  return time;
}
