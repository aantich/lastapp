import type { ConsoleConfig, Item, PasswordPolicy } from "../../api-types";

// Story data for the Northstar demo Tenant. Shapes follow the management API
// rows the console reads (api-types.ts); values are illustrative only.

// A supplied policy shaped like GET /v1/config. Stories pass their own policy
// so no control depends on a frontend copy of the server limits.
export function policy(min_length: number, max_length: number): PasswordPolicy {
  return {
    min_length,
    max_length,
    length_hint: `${min_length}–${max_length} characters.`,
  };
}

export function consoleConfig(password_policy: PasswordPolicy): ConsoleConfig {
  return {
    development_policy: {
      max_release_microapps: 20,
      deployment_states: {
        steps: ["queued", "preparing", "publishing", "active"],
        failed: "failed",
      },
      workspace_steps: ["storage", "tables", "publishing"],
    },
    oidc_enabled: true,
    password_policy,
    role_catalog: [],
  };
}

export const people: Item[] = [
  ["Ava Chen", "ava.chen", "active"],
  ["Mateo Alvarez", "mateo.alvarez", "active"],
  ["Priya Natarajan", "priya.natarajan", "active"],
  ["Jonas Weber", "jonas.weber", "active"],
  ["Lena Novak", "lena.novak", "suspended"],
  ["Samuel Okafor", "samuel.okafor", "active"],
  ["Hana Sato", "hana.sato", "active"],
  ["Diego Romero", "diego.romero", "active"],
  ["Freya Lindqvist", "freya.lindqvist", "active"],
  ["Omar Haddad", "omar.haddad", "active"],
  ["Chloe Martin", "chloe.martin", "active"],
  ["Ravi Kapoor", "ravi.kapoor", "active"],
].map(([display_name, handle, user_state], i) => ({
  id: `usr_${String(i + 1).padStart(4, "0")}`,
  user_id: `usr_${String(i + 1).padStart(4, "0")}`,
  display_name,
  email: `${handle}@northstar.example`,
  user_state,
}));

export const spaces: Item[] = [
  ["CRM", "crm", "Accounts, contacts and the Deal pipeline", 3],
  ["Field Service", "field-service", "Work orders and technician schedules", 2],
  ["Finance Ops", "finance-ops", "Invoices, approvals and spend", 2],
  ["People", "people", "Onboarding and equipment requests", 1],
  ["Support", "support", "Tickets and service-level tracking", 2],
  ["Partner Portal", "partner-portal", "Reseller registrations and leads", 1],
  ["Inventory", "inventory", "Stock levels across three warehouses", 1],
].map(([display_name, slug, description, apps], i) => ({
  id: `spc_${String(i + 1).padStart(4, "0")}`,
  display_name,
  slug,
  description,
  app_count: apps,
  state: i === 6 ? "archived" : "active",
}));

export const dealStages = [
  { value: "prospecting", label: "Prospecting" },
  { value: "qualified", label: "Qualified" },
  { value: "proposal", label: "Proposal sent" },
  { value: "negotiation", label: "Negotiation" },
  { value: "won", label: "Closed won" },
  { value: "lost", label: "Closed lost", disabled: true },
];
