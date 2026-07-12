// Thin fetch wrapper for the topology JSON API, modeled on the SWAMP/FabAID
// api.ts pattern: credentials included, typed errors, auto-redirect on 401.
export const API_BASE = "/api/v1";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function fetchJSON<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });
  if (res.status === 401 && typeof window !== "undefined") {
    const returnTo = encodeURIComponent(window.location.pathname);
    window.location.href = `/login?return_to=${returnTo}`;
    throw new ApiError(401, "unauthorized");
  }
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) msg = body.error;
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, msg);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

// ---- types ----

export interface SessionInfo {
  user: {
    id: string;
    display_name: string;
    status: string;
    is_provisioned: boolean;
    roles?: string[];
  };
  effective_role: string;
  roles: string[];
}

export interface DashboardResource {
  id: number;
  name: string;
  fqdn: string;
  active: boolean;
  resource_group: string;
}

export interface Proposal {
  id: string;
  entity_kind: string;
  target_name?: string;
  operation: string;
  status: string;
  proposed_state: unknown;
  schema_version: number;
  created_by: string;
  review_note?: string;
  created_at: string;
  updated_at: string;
  revisions?: ProposalRevision[];
}

export interface ProposalRevision {
  revision_no: number;
  proposed_state: unknown;
  edited_by: string;
  note?: string;
  created_at: string;
}

export interface Dashboard {
  my_resources: DashboardResource[];
  pending_registrations: Proposal[];
  pending_approvals: Proposal[];
  can_review: boolean;
}

export interface AuditEntry {
  id: string;
  actor_user_id?: string;
  action: string;
  entity_kind?: string;
  entity_id?: string;
  proposal_id?: string;
  created_at: string;
}

// ---- api surface ----

export const api = {
  auth: {
    me: () => fetchJSON<SessionInfo>("/auth/me"),
    mode: () => fetchJSON<{ mode: string }>("/auth/mode"),
    logout: () => fetchJSON<{ status: string }>("/auth/logout", { method: "POST" }),
    devLogin: (body: { email?: string; display_name?: string; role?: string }) =>
      fetchJSON<{ status: string; user_id: string }>("/auth/dev-login", {
        method: "POST",
        body: JSON.stringify(body),
      }),
  },
  dashboard: () => fetchJSON<Dashboard>("/dashboard"),
  resources: () => fetchJSON<Record<string, DashboardResource>>("/resources"),
  rgsummary: () => fetchJSON<unknown>("/rgsummary"),
  proposals: {
    mine: () => fetchJSON<Proposal[]>("/proposals/mine"),
    pending: () => fetchJSON<Proposal[]>("/proposals/pending"),
    get: (id: string) => fetchJSON<Proposal>(`/proposals/${id}`),
    create: (body: unknown) =>
      fetchJSON<{ id: string; status: string }>("/proposals", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    revise: (id: string, body: unknown) =>
      fetchJSON<{ status: string }>(`/proposals/${id}`, {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    submit: (id: string) =>
      fetchJSON<{ status: string }>(`/proposals/${id}/submit`, { method: "POST" }),
    withdraw: (id: string) =>
      fetchJSON<{ status: string }>(`/proposals/${id}/withdraw`, { method: "POST" }),
    approve: (id: string) =>
      fetchJSON<{ status: string }>(`/proposals/${id}/approve`, { method: "POST" }),
    reject: (id: string, note: string) =>
      fetchJSON<{ status: string }>(`/proposals/${id}/reject`, {
        method: "POST",
        body: JSON.stringify({ note }),
      }),
  },
  invites: {
    get: (token: string) => fetchJSON<InvitePreview>(`/invites/${token}`),
    accept: (token: string) =>
      fetchJSON<{ status: string }>(`/invites/${token}/accept`, { method: "POST" }),
    create: (body: unknown) =>
      fetchJSON<{ invite_url: string; token: string }>("/invites", {
        method: "POST",
        body: JSON.stringify(body),
      }),
  },
  audit: () => fetchJSON<AuditEntry[]>("/audit"),
};

export interface InvitePreview {
  kind: string;
  valid: boolean;
  expires_at: string;
  claim?: {
    entity_kind: string;
    entity_id: string;
    contact_type: string;
    rank: string;
  } | null;
}
