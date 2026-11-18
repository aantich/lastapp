// The console UI kit: the only source of shared console UI. Screens are built
// from these components and the Signal tokens (themes.css, styles in
// ui/kit.css); stories live in src/ui/controls (`npm run storybook`).

// Primitives
export {
  Button,
  IconButton,
  LinkButton,
  type ButtonProps,
  type ButtonVariant,
  type IconButtonProps,
} from "./button";
export { StatusBadge, Badge } from "./status";
export { statusTone, type Tone } from "./status-tone";
export { cx, initials } from "./utils";
export { Stack, Split, Inline, Columns } from "./layout";
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
export { Dialog, DialogBody, DialogFooter } from "./dialog";
export { Toaster } from "./toast";
export { toast, dismissToast, type ToastOptions } from "./toast-store";
export { Checkbox, Switch, RadioGroup, type RadioOption } from "./checkbox";
export { FormField, TextField, Textarea } from "./field";
export {
  Stepper,
  Progress,
  Timeline,
  type Step,
  type StepState,
  type TimelineEvent,
} from "./stepper";
export { Avatar, type AvatarKind } from "./avatar";
export { Tooltip } from "./tooltip";
export { Spinner, Skeleton, Kbd } from "./feedback";
export {
  FactList,
  Disclosure,
  CodeBlock,
  PickList,
  type Fact,
  type PickItem,
} from "./content";
export { SegmentedControl, type Segment } from "./segmented";

// Composites
export { SidebarNav, SidebarFooter, type NavItem } from "./sidebar";
export { useSidebarCollapse, useCommandPaletteShortcut } from "./hooks";
export { ContextSwitcher, type ContextOption } from "./context-switcher";
export { CommandPalette, type Command } from "./command-palette";
export { Menu, type MenuItem } from "./menu";
export { DataTable, type Column } from "./data-table";
export { RoleMatrix, type MatrixRow, type MatrixColumn } from "./role-matrix";

// Form controls
export { Select, ListboxPopup, type ListOption } from "./select";
export { RemoteSelect } from "./remote-select";
export { NumberInput } from "./number-input";
export { DatePicker } from "./date-picker";
export { isoDay, parseDay, addDays, daysBetween } from "./days";
export { NewPasswordInput } from "./new-password-input";
export { ConfirmDialog, ConfirmHost } from "./confirm-dialog";
export { confirmAction, type ConfirmOptions } from "./confirm";
export { FormModal, type Field, type Modal } from "./form-modal";
export { Collection } from "./collection";
export { ErrorBoundary } from "./error-boundary";
export { useDialogFocus } from "./use-dialog-focus";
export { useAppearance, applyMode, modes, type Mode } from "../appearance";
