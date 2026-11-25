// Shared interaction and visual pattern with the Broadcaster console.
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
export type SelectOption = { value: string; label: string; icon?: ReactNode };
export function Select({
  value,
  options,
  onChange,
  label,
  disabled = false,
}: {
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  label: string;
  disabled?: boolean;
}) {
  const id = useId(),
    root = useRef<HTMLDivElement>(null),
    trigger = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false),
    [active, setActive] = useState(0);
  const selected = options.findIndex((o) => o.value === value),
    item = options[selected];
  useEffect(() => {
    if (!open) return;
    const outside = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, [open]);
  useEffect(() => {
    if (open)
      document
        .getElementById(id + "-" + active)
        ?.scrollIntoView?.({ block: "nearest" });
  }, [open, active, id]);
  useEffect(() => {
    if (disabled) setOpen(false);
  }, [disabled]);
  const choose = (index: number) => {
    if (options[index]) onChange(options[index].value);
    setOpen(false);
    trigger.current?.focus();
  };
  return (
    <div
      className="select"
      ref={root}
      onKeyDown={(e) => {
        if (disabled) return;
        if (e.key === "Tab") {
          setOpen(false);
          return;
        }
        if (e.key === "Escape") {
          e.preventDefault();
          setOpen(false);
          trigger.current?.focus();
          return;
        }
        if (
          !["ArrowDown", "ArrowUp", "Home", "End", "Enter", " "].includes(e.key)
        )
          return;
        e.preventDefault();
        if (!options.length) return;
        if (e.key === "Enter" || e.key === " ") {
          if (open) choose(active);
          else {
            setActive(Math.max(selected, 0));
            setOpen(true);
          }
          return;
        }
        let next = active;
        if (e.key === "Home") next = 0;
        else if (e.key === "End") next = options.length - 1;
        else if (!open) next = Math.max(selected, 0);
        else
          next =
            (active + (e.key === "ArrowDown" ? 1 : -1) + options.length) %
            options.length;
        setActive(next);
        setOpen(true);
      }}
    >
      <button
        type="button"
        disabled={disabled}
        role="combobox"
        className="select-trigger"
        aria-label={label}
        aria-haspopup="listbox"
        aria-controls={id}
        aria-expanded={open}
        aria-activedescendant={open ? id + "-" + active : undefined}
        ref={trigger}
        onClick={() => {
          setActive(Math.max(selected, 0));
          setOpen(!open);
        }}
      >
        {item?.icon}
        <span>{item?.label ?? label}</span>
        <svg
          className="select-chevron"
          aria-hidden="true"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.6"
        >
          <path d="m7 10 5 5 5-5" />
        </svg>
      </button>
      {open && (
        <div className="select-menu" id={id} role="listbox" aria-label={label}>
          {options.map((o, i) => (
            <button
              key={o.value}
              type="button"
              role="option"
              aria-selected={o.value === value}
              id={id + "-" + i}
              tabIndex={-1}
              className={"select-option" + (active === i ? " is-active" : "")}
              onMouseMove={() => setActive(i)}
              onClick={() => choose(i)}
            >
              {o.icon}
              <span>{o.label}</span>
              {o.value === value && (
                <svg
                  className="select-check"
                  aria-hidden="true"
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.6"
                >
                  <path d="m5 12 4 4L19 6" />
                </svg>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
