export class Failure extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}
export async function api(
  path: string,
  method = "GET",
  body?: any,
  revision?: number,
) {
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
  });
  const json = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Failure(
      res.status,
      json.error?.message || `Request failed (${res.status})`,
    );
  return json;
}
