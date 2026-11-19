import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";
export type ListOption = { value: string; label: string; disabled?: boolean };
// Shared open/active/keyboard state for every popup list in the console
// (Select, RemoteSelect and ContextSwitcher). The popup itself is rendered
// by ListboxPopup in a portal and registers itself in `popup`; pointer
// presses outside both `root` and the popup close it. A list whose
// options depend on being open (RemoteSelect loads them only then) owns the
// open state and passes it in.
export function useListbox<T extends ListOption>(
  options: T[],
  choose: (option: T) => void,
  root: RefObject<HTMLElement | null>,
  openState?: [boolean, Dispatch<SetStateAction<boolean>>],
) {
  const id = useId();
  const ownOpen = useState(false),
    [open, setOpen] = openState || ownOpen,
    [active, setActive] = useState(0),
    popup = useRef<HTMLElement | null>(null);
  const setPopup = useCallback((el: HTMLElement | null) => {
    popup.current = el;
  }, []);
  useEffect(() => {
    if (!open) return;
    const close = (e: PointerEvent) => {
      const target = e.target as Node;
      if (!root.current?.contains(target) && !popup.current?.contains(target))
        setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [open, root, setOpen]);
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
    setPopup,
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
