export type Expression = {
  op?: string;
  args?: Expression[];
  field?: string;
  literal?: unknown;
  decimal?: string;
};
export type Query = {
  select?: string[];
  filter?: Expression;
  sort?: { field: string; direction: "asc" | "desc" }[];
  limit?: number;
  cursor?: string;
  include_count?: boolean;
};
export type MutationResult = { items: { id: string; revision: number }[] };
export type ShareOptions = {
  permissions: {
    id: string;
    label: string;
    operation: string;
    fields?: string[];
  }[];
  recipients: { id: string; display_name: string }[];
};
export type RuntimeContract = {
  id: string;
  version: string;
  entity: {
    key: string;
    label: string;
    fields: {
      key: string;
      label: string;
      type: string;
      read_only: boolean;
      has_default: boolean;
      [key: string]: unknown;
    }[];
    default_projection?: string[];
  };
  json_schema: Record<string, unknown>;
  create_schema: Record<string, unknown>;
  update_schema: Record<string, unknown>;
  typescript: string;
};
export class LastappError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public requestID?: string,
    public field?: string,
    public rule?: string,
    public retryable = false,
    public path?: string,
  ) {
    super(message);
  }
}
const encode = (a: Uint8Array) =>
  btoa(String.fromCharCode(...a))
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replace(/=+$/, "");
export class Lastapp {
  private bearer = "";
  private expires = 0;
  constructor(
    readonly apiOrigin: string,
    readonly appID: string,
  ) {
    const u = new URL(apiOrigin);
    if (
      u.origin !== apiOrigin ||
      !(
        u.protocol === "https:" ||
        ["localhost", "127.0.0.1"].includes(u.hostname)
      )
    )
      throw new Error("Use an HTTPS API origin (or loopback in development).");
  }
  async connect() {
    const verifier = encode(crypto.getRandomValues(new Uint8Array(32))),
      nonce = encode(crypto.getRandomValues(new Uint8Array(32)));
    const popup = window.open(
      "about:blank",
      "lastapp-" + this.appID,
      "popup,width=560,height=700",
    );
    if (!popup) throw new Error("Allow the Lastapp connection popup.");
    const challenge = encode(
      new Uint8Array(
        await crypto.subtle.digest(
          "SHA-256",
          new TextEncoder().encode(verifier),
        ),
      ),
    );
    const q = new URLSearchParams({ app: this.appID, nonce, challenge });
    const code = await new Promise<string>((resolve, reject) => {
      const finish = () => {
        window.removeEventListener("message", receive);
        clearTimeout(timer);
        clearInterval(closed);
      };
      const receive = (event: MessageEvent) => {
        if (
          event.origin !== this.apiOrigin ||
          event.source !== popup ||
          event.data?.type !== "lastapp:launch" ||
          event.data?.nonce !== nonce ||
          typeof event.data.code !== "string"
        )
          return;
        finish();
        resolve(event.data.code);
      };
      const timer = setTimeout(() => {
        finish();
        popup.close();
        reject(new Error("Connection window timed out. Try again."));
      }, 300000);
      const closed = setInterval(() => {
        if (popup.closed) {
          finish();
          reject(new Error("Connection window closed."));
        }
      }, 500);
      window.addEventListener("message", receive);
      popup.location.href = this.apiOrigin + "/console/launch?" + q;
    });
    await this.connectDevelopment({code,verifier,nonce});
    popup.close();
  }
  /** Exchange a one-use workspace launch code. Never persist these credentials. */
  async connectDevelopment({code,verifier,nonce}:{code:string;verifier:string;nonce:string}) {
    const result = await this.request<{
      access_token: string;
      expires_in: number;
    }>(
      "/session/exchange",
      "POST",
      { code, verifier, nonce },
      undefined,
      false,
    );
    this.bearer = result.access_token;
    this.expires = Date.now() + result.expires_in * 1000;
  }
  disconnect() {
    this.bearer = "";
    this.expires = 0;
  }
  async request<T>(
    path: string,
    method = "GET",
    body?: unknown,
    revision?: number,
    auth = true,
  ): Promise<T> {
    if (auth && (!this.bearer || Date.now() >= this.expires))
      throw new LastappError(401, "unauthenticated", "Connect the App first.");
    const response = await fetch(this.apiOrigin + "/data/v1" + path, {
      method,
      credentials: "omit",
      headers: {
        "Content-Type": "application/json",
        ...(auth ? { Authorization: "Bearer " + this.bearer } : {}),
        ...(revision ? { "If-Match": String(revision) } : {}),
      },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    });
    const data = await response.json();
    if (!response.ok) {
      if (response.status === 401) this.disconnect();
      throw new LastappError(
        response.status,
        data.error?.code || "request_failed",
        data.error?.message || "Request failed",
        data.error?.request_id,
        data.error?.field,
        data.error?.rule,
        !!data.error?.retryable,
        data.error?.field_or_rule,
      );
    }
    return data;
  }
  schema() {
    return this.request<{
      entities: RuntimeContract[];
      capabilities: Record<string, unknown>;
    }>("/schema");
  }
  query<T>(entity: string, query: Query) {
    return this.request<{
      items: T[];
      version: string;
      next_cursor?: string;
      count?: number;
    }>("/entities/" + encodeURIComponent(entity) + "/query", "POST", query);
  }
  get<T>(entity: string, id: string, select?: string[]) {
    return this.request<T>(
      "/entities/" +
        encodeURIComponent(entity) +
        "/records/" +
        encodeURIComponent(id) +
        (select ? "?select=" + encodeURIComponent(select.join(",")) : ""),
    );
  }
  create(entity: string, values: unknown, id = crypto.randomUUID()) {
    return this.request<MutationResult>(
      "/entities/" + encodeURIComponent(entity) + "/records",
      "POST",
      { id, values },
    );
  }
  update(entity: string, id: string, revision: number, values: unknown) {
    return this.request<MutationResult>(
      "/entities/" +
        encodeURIComponent(entity) +
        "/records/" +
        encodeURIComponent(id),
      "PATCH",
      { values },
      revision,
    );
  }
  delete(entity: string, id: string, revision: number) {
    return this.request<MutationResult>(
      "/entities/" +
        encodeURIComponent(entity) +
        "/records/" +
        encodeURIComponent(id),
      "DELETE",
      undefined,
      revision,
    );
  }
  bulk(
    entity: string,
    items: {
      operation: "create" | "update" | "delete";
      id: string;
      revision?: number;
      values?: unknown;
    }[],
  ) {
    return this.request<MutationResult>(
      "/entities/" + encodeURIComponent(entity) + "/bulk",
      "POST",
      { items },
    );
  }
  referenceOptions(entity: string, field: string, query = "") {
    return this.request<{ items: { id: string; display_name: string }[] }>(
      "/entities/" +
        encodeURIComponent(entity) +
        "/reference-options?" +
        new URLSearchParams({ field, q: query }),
    );
  }
  capabilities(entity: string, ids: string[]) {
    return this.request(
      "/entities/" + encodeURIComponent(entity) + "/capabilities",
      "POST",
      { ids },
    );
  }
  shareOptions(
    entity: string,
    id: string,
    kind: "user" | "team" = "user",
    query = "",
  ) {
    return this.request<ShareOptions>(
      "/entities/" +
        encodeURIComponent(entity) +
        "/records/" +
        encodeURIComponent(id) +
        "/shares?" +
        new URLSearchParams({
          view: "options",
          recipient_kind: kind,
          q: query,
        }),
    );
  }
  shares(entity: string, id: string) {
    return this.request(
      "/entities/" +
        encodeURIComponent(entity) +
        "/records/" +
        encodeURIComponent(id) +
        "/shares",
    );
  }
  share(
    entity: string,
    id: string,
    grant: {
      permission_id: string;
      recipient_kind: "user" | "team";
      recipient_id: string;
    },
    revoke = false,
  ) {
    return this.request(
      "/entities/" +
        encodeURIComponent(entity) +
        "/records/" +
        encodeURIComponent(id) +
        "/shares",
      revoke ? "DELETE" : "POST",
      grant,
    );
  }
}
