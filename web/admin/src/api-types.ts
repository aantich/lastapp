// Console response shapes. The backend writes most responses as untyped Go
// maps (control.Row), so there is no generated TypeScript for them: the
// generated TypeScript in internal/entity/contracts.go covers per-entity App
// record data only. Each shape names the Go source it mirrors; change
// both together.
//
// The shapes are split by domain under api-types/; this module re-exports
// them so every screen imports from one place.
export * from "./api-types/loose";
export * from "./api-types/console";
export * from "./api-types/entities";
export * from "./api-types/delivery";
export * from "./api-types/agents";
