import { useEffect, useRef } from "react";
import { Pencil, Trash2, X } from "lucide-react";

// A row in the designer's field, access and rule lists.
export function EntityRow({
  title,
  summary,
  edit,
  remove,
}: {
  title: string;
  summary: React.ReactNode;
  edit: () => void;
  remove?: () => void;
}) {
  return (
    <article className="entity-row">
      <div className="entity-row-copy">
        <strong>{title}</strong>
        <small>{summary}</small>
      </div>
      <div className="entity-row-actions">
        <button
          className="secondary"
          aria-label={"Edit " + title}
          onClick={edit}
        >
          <Pencil size={13} /> Edit
        </button>
        {remove && (
          <button
            className="icon danger"
            aria-label={"Remove " + title}
            title={"Remove " + title}
            onClick={remove}
          >
            <Trash2 size={15} />
          </button>
        )}
      </div>
    </article>
  );
}
export function EditorDialog({
  title,
  close,
  children,
}: {
  title: string;
  close: () => void;
  children: React.ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const active = document.activeElement as HTMLElement,
      dialog = ref.current;
    dialog?.showModal();
    return () => {
      dialog?.close();
      active?.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className="modal entity-dialog"
      aria-label={title}
      onCancel={(e) => {
        e.preventDefault();
        close();
      }}
    >
      <header>
        <h2>{title}</h2>
        <button className="icon" aria-label="Close editor" onClick={close}>
          <X size={18} />
        </button>
      </header>
      <div className="entity-dialog-body">{children}</div>
    </dialog>
  );
}
