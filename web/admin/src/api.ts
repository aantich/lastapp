export class Failure extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}
export const sessionExpiredEvent = "lastapp:session-expired";
let sessionGeneration = 0;
export function resetSessionRequests() {
  sessionGeneration++;
}
// Response shapes live in api-types.ts. Callers name the shape they read;
// the default is unknown so an unread response cannot be used untyped.
export async function api<T = unknown>(
  path: string,
  method = "GET",
  body?: unknown,
  revision?: number,
  signal?: AbortSignal,
): Promise<T> {
  const started = sessionGeneration;
  const csrf =
    document.cookie
      .split("; ")
      .find(
        (x) =>
          x.startsWith("lastapp_csrf=") || x.startsWith("__Host-lastapp_csrf="),
      )
      ?.split("=")[1] || "";
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrf,
      ...(revision ? { "If-Match": String(revision) } : {}),
    },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    signal,
  });
  const json: { error?: { code?: string; message?: string } } = await res
    .json()
    .catch(() => ({}));
  if (
    res.status === 401 &&
    json.error?.code === "unauthenticated" &&
    started === sessionGeneration
  ) {
    resetSessionRequests();
    window.dispatchEvent(new Event(sessionExpiredEvent));
  }
  if (res.ok && (path === "/auth/login/password" || path === "/auth/password"))
    resetSessionRequests();
  if (!res.ok)
    throw new Failure(
      res.status,
      json.error?.message || `Request failed (${res.status})`,
    );
  return json as T;
}
