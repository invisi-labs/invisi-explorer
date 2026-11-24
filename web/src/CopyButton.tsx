import { useEffect, useRef, useState } from "react";
import type { Key } from "./i18n";

export function CopyButton({ value, t, className = "icon" }: {
  value: string;
  t: (key: Key) => string;
  className?: string;
}) {
  const [message, setMessage] = useState("");
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  return <>
    <button type="button" className={className} aria-label={t("copy")} title={t("copy")}
      onClick={async (event) => {
        event.stopPropagation();
        try {
          await navigator.clipboard.writeText(value);
          setMessage(t("copied"));
        } catch {
          setMessage(t("copyFailed"));
        }
        clearTimeout(timer.current);
        timer.current = setTimeout(() => setMessage(""), 2000);
      }}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><rect x="8" y="8" width="12" height="13" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/></svg>
    </button>
    <span className="sr" aria-live="polite">{message}</span>
  </>;
}
