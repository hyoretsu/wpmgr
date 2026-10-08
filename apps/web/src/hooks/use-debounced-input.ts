import { useCallback, useEffect, useRef, useState } from "react";

/** Keep typing local; notify the form after a quiet interval. */
export function useDebouncedInput(value: string, onChange: (value: string) => void, delay = 300) {
  const [local, setLocal] = useState(value);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const callback = useRef(onChange);
  useEffect(() => { callback.current = onChange; }, [onChange]);
  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);
  const update = useCallback((next: string) => {
    setLocal(next);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => callback.current(next), delay);
  }, [delay]);
  return [local, update] as const;
}
