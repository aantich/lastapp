import { useState } from "react";
import { Check } from "lucide-react";
import { Dialog, DialogFooter } from "./dialog";
import { Button } from "./button";
import { Alert } from "./alert";
import { toast } from "./toast-store";
import { Disclosure } from "./content";
import { FormModalField } from "./form-modal-field";
import { usePasswordPolicy } from "../console-config";
import { useAction } from "../use-action";
import type { Item } from "../api-types";
import type { Modal } from "./form-modal-types";
import { Icon } from "./icon";

function savedMessage(title: string) {
  const [verb, ...rest] = title.split(" ");
  const subject = rest.join(" ");
  const done = { Create: "created", Add: "added", Edit: "saved" }[verb];
  return done && subject
    ? `${subject[0].toUpperCase()}${subject.slice(1)} ${done}.`
    : "Saved.";
}
export function FormModal({
  modal,
  close,
  done,
  lookups,
}: {
  modal: Modal;
  close: () => void;
  done: () => void;
  lookups: Record<string, string>;
}) {
  const policy = usePasswordPolicy();
  const [values, setValues] = useState<Item>(modal.initial);
  const [error, setError] = useState("");
  const [visible, setVisible] = useState<string[]>([]);
  const { busy, run } = useAction(modal, {
    onSuccess: () => {
      toast(modal.success || savedMessage(modal.title), { tone: "success" });
      done();
    },
    onError: setError,
  });
  function submit(e: React.FormEvent) {
    e.preventDefault();
    void run(() => modal.save(values));
  }
  return (
    <Dialog
      title={modal.title}
      description={modal.description}
      titleId="modal-title"
      className="form-modal"
      dismissable={!busy}
      onClose={close}
    >
      <form onSubmit={submit}>
        <div className="fields">
          {[false, true].map((advanced) => {
            const fields = modal.fields.filter(
              (f) => !!f.advanced === advanced && (!f.when || f.when(values)),
            );
            const content = fields.map((f, i) => (
              <FormModalField
                key={f.key}
                f={f}
                fields={modal.fields}
                autoFocus={i === 0 && !advanced}
                values={values}
                setValues={setValues}
                visible={visible}
                setVisible={setVisible}
                policy={policy}
                lookups={lookups}
              />
            ));
            return advanced ? (
              fields.length > 0 && (
                <Disclosure
                  summary="Advanced"
                  className="advanced-fields wide"
                  key="advanced"
                >
                  <div className="fields">{content}</div>
                </Disclosure>
              )
            ) : (
              <span className="basic-fields" key="basic">
                {content}
              </span>
            );
          })}
        </div>
        {error && <Alert>{error}</Alert>}
        <DialogFooter>
          <Button disabled={busy} onClick={close}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={busy}
            icon={<Icon icon={Check} size="lg" aria-hidden />}
          >
            {busy ? "Saving…" : modal.submit || "Save changes"}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}
