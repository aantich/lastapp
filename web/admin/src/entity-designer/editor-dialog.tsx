import { Pencil, Trash2 } from "lucide-react";
import { Dialog } from "../ui";

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
// The designer's field, access, rule and review editors: the kit Dialog.
export function EditorDialog({
  title,
  close,
  children,
}: {
  title: string;
  close: () => void;
  children: React.ReactNode;
}) {
  return (
    <Dialog
      title={title}
      onClose={close}
      closeLabel="Close editor"
      className="entity-dialog"
      closeOnBackdrop={false}
    >
      <div className="entity-dialog-body">{children}</div>
    </Dialog>
  );
}
