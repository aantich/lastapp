// The console UI kit: the only source of shared console UI. Screens are built
// from these components and the Signal tokens (themes.css, styles in
// ui/kit.css); stories live in src/ui/controls (`npm run storybook`).

// Primitives
export {
  Button,
  IconButton,
  type ButtonProps,
  type ButtonVariant,
  type IconButtonProps,
} from "./button";
export { StatusBadge, Badge } from "./status";
export { statusTone, type Tone } from "./status-tone";
export { cx, initials } from "./utils";
export {
  Card,
  CardHeader,
  CardBody,
  CardFooter,
  StatCard,
  StatGrid,
  AttentionCard,
  AttentionGrid,
} from "./card";
export { PageHeader, Breadcrumbs, type Crumb } from "./page-header";
export { Tabs, type TabItem } from "./tabs";
export { EmptyState } from "./empty-state";
export { Alert, InlineError, type AlertTone } from "./alert";
export { Checkbox, Switch, RadioGroup, type RadioOption } from "./checkbox";
export { FormField, TextField, Textarea } from "./field";
export { Avatar, type AvatarKind } from "./avatar";
export { Tooltip } from "./tooltip";
export { Spinner, Skeleton, Kbd } from "./feedback";

// Form controls
export { Select, ListboxPopup, type ListOption } from "./select";
export { RemoteSelect } from "./remote-select";
export { NumberInput } from "./number-input";
export { NewPasswordInput } from "./new-password-input";
export { ConfirmDialog, ConfirmHost } from "./confirm-dialog";
export { confirmAction, type ConfirmOptions } from "./confirm";
export { FormModal, type Field, type Modal } from "./form-modal";
export { Collection } from "./collection";
export { ErrorBoundary } from "./error-boundary";
export { useDialogFocus } from "./use-dialog-focus";
export { useAppearance, applyMode, modes, type Mode } from "../appearance";
