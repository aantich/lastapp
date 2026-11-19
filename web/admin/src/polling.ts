// How often screens re-read the server (ms). refetchInterval never overlaps
// requests and pauses in a hidden tab.
export const poll = {
  // Server work in progress: a deployment, an entity publication.
  pending: 1500,
  // The longest wait between retries after a failed read of pending work.
  maxBackoff: 10000,
  // Development workspaces while one is starting or stopping.
  workspaces: 2000,
  // Everything the Tenant workspace shows, while no form is open.
  tenant: 5000,
} as const;
