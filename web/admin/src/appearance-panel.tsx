import { Check, RotateCcw } from "lucide-react";
import { Alert, Button, Dialog, DialogBody, RadioGroup, Stack } from "./ui";
import { modes, type Appearance, type Mode } from "./appearance";

const modeLabels: Record<Mode, string> = {
  light: "Light",
  dark: "Dark",
  system: "System",
};

// The appearance dialog: the Signal theme's color mode (light, dark or the
// device setting), applied at once and saved on this browser.
export function AppearancePanel({
  value,
  onChange,
  reset,
  close,
}: {
  value: Appearance;
  onChange: (value: Appearance) => void;
  reset: () => void;
  close: () => void;
}) {
  return (
    <Dialog
      title="Appearance"
      description="Make the workspace your own. Changes apply instantly and are saved on this browser."
      closeLabel="Close appearance"
      size="sm"
      onClose={close}
      footer={
        <>
          <Button icon={<RotateCcw size={15} aria-hidden />} onClick={reset}>
            Reset to default
          </Button>
          <Button variant="primary" onClick={close}>
            Done
            <Check size={15} aria-hidden />
          </Button>
        </>
      }
    >
      <DialogBody>
        <Stack gap="md">
          <RadioGroup
            legend="Color mode"
            name="color-mode"
            horizontal
            options={modes.map((mode) => ({
              value: mode,
              label: modeLabels[mode],
            }))}
            value={value.mode}
            onChange={(mode) => onChange({ ...value, mode: mode as Mode })}
          />
          <Alert tone="info" live={false}>
            The Signal theme follows you across all console screens, forms and
            resource tables. System mode follows your device setting.
          </Alert>
        </Stack>
      </DialogBody>
    </Dialog>
  );
}
