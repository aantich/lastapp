import { useEffect, useId, useState, type RefObject } from "react";
export type ListOption = { value: string; label: string; disabled?: boolean };
// Shared open/active/keyboard state for every popup list in the console
// (Select and RemoteSelect). The popup itself is rendered by ListboxPopup
// inside `root`; pointer presses outside `root` close it.
export function useListbox<T extends ListOption>(
  options: T[],
  choose: (option: T) => void,
  root: RefObject<HTMLElement | null>,
) {
  const id = useId();
  const [open, setOpen] = useState(false),
    [active, setActive] = useState(0);
  useEffect(() => {
    if (!open) return;
    const close = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [open, root]);
  useEffect(() => {
    if (open)
      document
        .getElementById(`${id}-${active}`)
        ?.scrollIntoView({ block: "nearest" });
  }, [id, open, active]);
  // Next enabled option from `from` in direction `delta`; stays put at the end.
  function step(from: number, delta: 1 | -1) {
    for (let i = from + delta; i >= 0 && i < options.length; i += delta)
      if (!options[i].disabled) return i;
    return Math.max(0, Math.min(options.length - 1, from));
  }
  // Handles the keys common to every list; returns true when it consumed one.
  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (open) setActive((i) => step(i, e.key === "ArrowDown" ? 1 : -1));
      else setOpen(true);
      return true;
    }
    if (!open) return false;
    if (e.key === "Enter") {
      e.preventDefault();
      const option = options[active];
      if (option && !option.disabled) choose(option);
      return true;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      setOpen(false);
      return true;
    }
    if (e.key === "Tab") setOpen(false);
    return false;
  }
  return {
    id,
    open,
    setOpen,
    active,
    setActive,
    step,
    onKeyDown,
    activeId: open && options[active] ? `${id}-${active}` : undefined,
  };
}
export type Listbox = ReturnType<typeof useListbox>;
