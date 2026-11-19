const gap = 4,
  margin = 8;
// The element a popup is portalled into: the open modal dialog that holds
// the control (so its focus trap and aria-modal still include the popup),
// otherwise <body>. The popup never takes part in the control's layout. The
// ink sidebar (a modal drawer on narrow screens) is never the host: a popup
// opened from it uses the page's surface tokens, not the sidebar's.
export function portalHost(anchor: HTMLElement) {
  return (
    anchor.closest<HTMLElement>('[aria-modal="true"]:not(.sidebar)') ||
    document.body
  );
}
// Floats the popup over the page beside its anchor: below it, or above when
// the viewport has more room there ("right": beside a rail tile), at least
// as wide as the anchor and never taller than the space available; the list
// inside it scrolls.
export function place(
  popup: HTMLElement,
  anchor: HTMLElement,
  side: "bottom" | "right",
  minWidth: number,
  cap: number,
) {
  const r = anchor.getBoundingClientRect(),
    width = Math.min(Math.max(r.width, minWidth), innerWidth - 2 * margin);
  popup.style.width = width + "px";
  let left: number,
    top: number,
    up = false;
  if (side === "right") {
    popup.style.maxHeight = Math.min(cap, innerHeight - 2 * margin) + "px";
    left = r.right + margin;
    top = Math.max(
      margin,
      Math.min(r.top, innerHeight - margin - popup.offsetHeight),
    );
  } else {
    const below = innerHeight - r.bottom - gap - margin,
      above = r.top - gap - margin;
    up = below < 200 && above > below;
    popup.style.maxHeight =
      Math.max(96, Math.min(cap, up ? above : below)) + "px";
    left = r.left;
    top = up ? r.top - gap - popup.offsetHeight : r.bottom + gap;
  }
  left = Math.max(margin, Math.min(left, innerWidth - margin - width));
  // A fixed popup inside a transformed host (the narrow-screen drawer) is
  // placed relative to that host.
  const host = popup.parentElement;
  if (
    host &&
    host !== document.body &&
    getComputedStyle(host).transform !== "none"
  ) {
    const h = host.getBoundingClientRect();
    left += host.scrollLeft - h.left;
    top += host.scrollTop - h.top;
  }
  popup.classList.toggle("above", up);
  popup.style.left = left + "px";
  popup.style.top = top + "px";
}
