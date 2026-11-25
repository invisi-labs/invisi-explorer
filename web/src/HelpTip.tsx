import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
export function HelpTip({ label, text }: { label: string; text: string }) {
  const id = useId(),
    trigger = useRef<HTMLButtonElement>(null),
    bubble = useRef<HTMLDivElement>(null),
    pinned = useRef(false);
  const [open, setOpen] = useState(false),
    [position, setPosition] = useState({ left: 12, top: 12 });
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const show = () => {
    clearTimeout(timer.current);
    setOpen(true);
  };
  const leave = () => {
    if (!pinned.current) timer.current = setTimeout(() => setOpen(false), 150);
  };
  useEffect(() => () => clearTimeout(timer.current), []);
  const close = () => {
    clearTimeout(timer.current);
    pinned.current = false;
    setOpen(false);
  };
  useLayoutEffect(() => {
    if (!open) return;
    const place = () => {
      const anchor = trigger.current?.getBoundingClientRect(),
        tip = bubble.current?.getBoundingClientRect();
      if (!anchor || !tip) return;
      setPosition({
        left: Math.max(
          12,
          Math.min(anchor.left, window.innerWidth - tip.width - 12),
        ),
        top:
          anchor.bottom + tip.height + 12 < window.innerHeight
            ? anchor.bottom + 8
            : Math.max(12, anchor.top - tip.height - 8),
      });
    };
    place();
    window.addEventListener("resize", place);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", close, true);
    };
  }, [open, text]);
  useEffect(() => {
    if (!open) return;
    const outside = (event: PointerEvent) => {
      if (
        !trigger.current?.contains(event.target as Node) &&
        !bubble.current?.contains(event.target as Node)
      )
        close();
    };
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        close();
      }
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", key, true);
    };
  }, [open]);
  return (
    <span className="term-help">
      <button
        ref={trigger}
        type="button"
        className="term-help-trigger"
        aria-label={label}
        aria-describedby={open ? id : undefined}
        aria-expanded={open}
        onMouseEnter={show}
        onMouseLeave={leave}
        onFocus={show}
        onBlur={close}
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          pinned.current = !pinned.current;
          setOpen(pinned.current);
        }}
      >
        ?
      </button>
      {open &&
        createPortal(
          <div
            ref={bubble}
            id={id}
            role="tooltip"
            onMouseEnter={show}
            onMouseLeave={leave}
            className="term-help-bubble"
            style={position}
          >
            {text}
          </div>,
          trigger.current?.closest("dialog") ?? document.body,
        )}
    </span>
  );
}
