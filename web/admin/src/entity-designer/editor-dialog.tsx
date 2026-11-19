import type { FormEvent, ReactNode } from "react";
import { Pencil, Trash2 } from "lucide-react";
import {
  Alert,
  Button,
  Dialog,
  DialogBody,
  DialogFooter,
  IconButton,
  Inline,
  Stack,
  Icon,
} from "../ui";

// Edit and Remove for one row of the designer's field, access and rule
// tables. The names carry the row ("Edit Website") for assistive technology.
export function RowActions({
  title,
  edit,
  remove,
}: {
  title: string;
  edit: () => void;
  remove?: () => void;
}) {
  return (
    <Inline gap="sm">
      <IconButton
        label={"Edit " + title}
        tooltip
        icon={<Icon icon={Pencil} size="md" aria-hidden />}
        onClick={edit}
      />
      {remove && (
        <IconButton
          label={"Remove " + title}
          tone="danger"
          tooltip
          icon={<Icon icon={Trash2} size="md" aria-hidden />}
          onClick={remove}
        />
      )}
    </Inline>
  );
}

// The designer's field, access and rule editors: the kit Dialog. A stray
// press outside keeps it open; Escape and "Close editor" discard the edit.
export function EditorDialog({
  title,
  description,
  close,
  children,
}: {
  title: string;
  description?: ReactNode;
  close: () => void;
  children: ReactNode;
}) {
  return (
    <Dialog
      title={title}
      description={description}
      onClose={close}
      closeLabel="Close editor"
      size="lg"
      closeOnBackdrop={false}
    >
      {children}
    </Dialog>
  );
}

// An editor's form: its controls in the dialog body and the submit button
// in the footer, so Enter in a text field saves.
export function EditorForm({
  submit,
  submitLabel,
  error,
  children,
}: {
  submit: () => void;
  submitLabel: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <form
      onSubmit={(e: FormEvent) => {
        e.preventDefault();
        submit();
      }}
    >
      <DialogBody>
        <Stack gap="md">
          {error && <Alert as="p">{error}</Alert>}
          {children}
        </Stack>
      </DialogBody>
      <DialogFooter>
        <Button type="submit" variant="primary">
          {submitLabel}
        </Button>
      </DialogFooter>
    </form>
  );
}
