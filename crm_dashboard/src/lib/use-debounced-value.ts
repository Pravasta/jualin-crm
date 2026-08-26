"use client";

import { useEffect, useState } from "react";

// Used for the keyword search box — without this, every keystroke would
// fire a request, and out-of-order responses would race (fixed by the
// AbortController in leads-list.tsx, but there's no reason to send the
// requests in the first place).
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}
