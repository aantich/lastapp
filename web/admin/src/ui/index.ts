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
export { StandalonePage } from "./standalone-page";
export {
  Card,
  CardHeader,
  CardBody,
  CardFooter,
  StatCard,
  StatGrid,
  AttentionCard,
  AttentionGrid,
  AllClear,
  AppCard,
} from "./card";
export { PageHeader, PageIntro, Breadcrumbs, type Crumb } from "./page-header";
export { ActivityList, ActorMark, type ActivityItem } from "./activity-list";
export { Tabs, type TabItem } from "./tabs";
export { EmptyState } from "./empty-state";
export { Alert, InlineError, type AlertTone } from "./alert";
export { Dialog, DialogBody, DialogFooter } from "./dialog";
export { Toaster } from "./toast";
export { toast, dismissToast, type ToastOptions } from "./toast-store";
export { Checkbox, Switch, RadioGroup, type RadioOption } from "./checkbox";
export { FormField, TextField, Textarea } from "./field";
export { FieldGroup } from "./field-group";
export {
  Stepper,
  Progress,
  Timeline,
  type Step,
  type StepState,
  type TimelineEvent,
} from "./stepper";
export { Avatar, type AvatarKind, type AvatarSize } from "./avatar";
export { Icon, type IconSize, type IconStroke } from "./icon";
export type { Measure } from "./measure";
export { Tooltip } from "./tooltip";
export { Spinner, Skeleton, Kbd } from "./feedback";
export { LoadingScreen } from "./loading-screen";
export { UploadButton } from "./upload-button";
export {
  FactList,
  Disclosure,
  CodeBlock,
  PickList,
  type Fact,
  type PickItem,
} from "./content";
export { SegmentedControl, type Segment } from "./segmented";
export { AuthLayout } from "./auth-layout";
export { Section, Lead } from "./section";
export { NavList, NavListNote, type NavListItem } from "./nav-list";
export { GridTable, type GridColumn, type GridRow } from "./grid-table";
export {
  RecordHeader,
  Panel,
  PathCard,
  Callout,
  ChipRow,
  ResultTile,
  ResultTiles,
  DesignerPage,
  DesignerBody,
  DesignerNote,
} from "./designer-parts";

// Composites
export {
  SidebarNav,
  SidebarLinks,
  type NavItem,
  type SidebarLink,
} from "./sidebar";
export { AppShell, ContextLabel } from "./app-shell";
export { SearchField } from "./search-field";
export { AccountMenu } from "./account-menu";
export { useSidebarCollapse, useCommandPaletteShortcut } from "./hooks";
export { ContextSwitcher, type ContextOption } from "./context-switcher";
export { CommandPalette, type Command } from "./command-palette";
export { Menu, type MenuItem } from "./menu";
export { DataTable, type Column } from "./data-table";
export { RecordList, type RecordRow } from "./record-list";
export {
  RowTable,
  CellTitle,
  CellMono,
  CellActions,
  type RowTableRow,
} from "./row-table";
export {
  AppHeader,
  AppTile,
  CardRow,
  PageStack,
  TitledCard,
  ValueBox,
} from "./app-header";
export { RoleMatrix, type MatrixRow, type MatrixColumn } from "./role-matrix";
export {
  AccessMatrix,
  AssignmentList,
  type AccessCell,
  type AccessMatrixRow,
} from "./access-matrix";
export {
  ConnectionCard,
  ConnectionGrid,
  ChoiceCards,
  FlowCard,
  FlowSection,
  FlowStack,
  type Choice,
} from "./connection-card";

// Form controls
export { Select, type ListOption } from "./select";
export { ListboxPopup } from "./listbox-popup";
export { RemoteSelect } from "./remote-select";
export { NumberInput } from "./number-input";
export { DatePicker } from "./date-picker";
export { isoDay, parseDay, addDays, daysBetween } from "./days";
export { NewPasswordInput } from "./new-password-input";
export { ConfirmDialog, ConfirmHost } from "./confirm-dialog";
export { confirmAction, type ConfirmOptions } from "./confirm";
export { FormModal } from "./form-modal";
export type { Field, Modal } from "./form-modal-types";
export { Collection } from "./collection";
export { ErrorBoundary } from "./error-boundary";
export { useDialogFocus } from "./use-dialog-focus";
export { useAppearance, applyMode, modes, type Mode } from "../appearance";
