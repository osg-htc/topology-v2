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
    // Don't bounce when already on a public page (avoids a /login redirect loop).
    const path = window.location.pathname;
    const onPublic = path.startsWith("/login") || path.startsWith("/invites/accept");
    if (!onPublic) {
      const returnTo = encodeURIComponent(path);
      window.location.href = `/login?return_to=${returnTo}`;
    }
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
  summary: () => fetchJSON<Summary>("/summary"),
  resources: () => fetchJSON<Record<string, DashboardResource>>("/resources"),
  resourceDetail: (name: string) =>
    fetchJSON<ResourceDetail>(`/resources/${encodeURIComponent(name)}`),
  resourceGroupDetail: (name: string) =>
    fetchJSON<RGDetail>(`/resource-groups/${encodeURIComponent(name)}`),
  siteDetail: (name: string) => fetchJSON<SiteDetail>(`/sites/${encodeURIComponent(name)}`),
  facilityDetail: (name: string) =>
    fetchJSON<FacilityDetail>(`/facilities/${encodeURIComponent(name)}`),
  rgsummary: () => fetchJSON<unknown>("/rgsummary"),
  serviceNames: () => fetchJSON<string[]>("/service-names"),
  voNames: () => fetchJSON<string[]>("/vo-names"),
  tagNames: () => fetchJSON<string[]>("/tag-names"),
  contacts: () => fetchJSON<{ name: string; id: string }[]>("/contacts"),
  resourceGroups: (includeInactive = false) =>
    fetchJSON<ResourceGroup[]>(`/resource-groups${includeInactive ? "?include_inactive=1" : ""}`),
  sites: (includeInactive = false) =>
    fetchJSON<Site[]>(`/sites${includeInactive ? "?include_inactive=1" : ""}`),
  facilities: (includeInactive = false) =>
    fetchJSON<Facility[]>(`/facilities${includeInactive ? "?include_inactive=1" : ""}`),
  institutions: () => fetchJSON<Institution[]>("/institutions"),
  downtimes: (filter?: { resource?: string; rg?: string }) => {
    const p = new URLSearchParams();
    if (filter?.resource) p.set("resource", filter.resource);
    if (filter?.rg) p.set("rg", filter.rg);
    const qs = p.toString();
    return fetchJSON<Downtime[]>(`/downtimes${qs ? `?${qs}` : ""}`);
  },
  projects: (includeInactive = false) =>
    fetchJSON<Project[]>(`/projects${includeInactive ? "?include_inactive=1" : ""}`),
  project: (name: string) => fetchJSON<ProjectDetail>(`/projects/${encodeURIComponent(name)}`),
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
  userLabels: (ids: string[]) =>
    fetchJSON<UserLabelT[]>(`/user-labels?ids=${encodeURIComponent(ids.join(","))}`),
  admin: {
    syncInstitutions: () =>
      fetchJSON<{ synced: number }>("/admin/institutions/sync", { method: "POST" }),
    getOIDCConfig: () => fetchJSON<OIDCConfig>("/admin/oidc-config"),
    setOIDCConfig: (body: { issuer: string; client_id: string; client_secret?: string }) =>
      fetchJSON<{ status: string }>("/admin/oidc-config", {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    listUsers: () => fetchJSON<AdminUser[]>("/admin/users"),
    searchUsers: (q: string) =>
      fetchJSON<UserSearchResult[]>(`/admin/users/search?q=${encodeURIComponent(q)}`),
    setUserRole: (id: string, role: string, action: "add" | "remove") =>
      fetchJSON<{ status: string }>(`/admin/users/${id}/roles`, {
        method: "POST",
        body: JSON.stringify({ role, action }),
      }),
    setUsername: (id: string, username: string) =>
      fetchJSON<{ status: string }>(`/admin/users/${id}/username`, {
        method: "POST",
        body: JSON.stringify({ username }),
      }),
    listBackups: () => fetchJSON<{ backups: string[] }>("/admin/backups"),
    createBackup: () => fetchJSON<{ key: string; size: number }>("/admin/backup", { method: "POST" }),
    restore: (key: string) =>
      fetchJSON<{ status: string }>("/admin/restore", {
        method: "POST",
        body: JSON.stringify({ key }),
      }),
    importGitHub: (ref: string) =>
      fetchJSON<{ status: string; repo: string }>("/admin/import-github", {
        method: "POST",
        body: JSON.stringify({ ref }),
      }),
  },
};

export interface Summary {
  resources: number;
  resource_groups: number;
  sites: number;
  facilities: number;
  institutions: number;
  vos: number;
  projects: number;
}

export interface ResourceGroup {
  name: string;
  group_id: number;
  site: string;
  facility: string;
  production: boolean | null;
  support_center: string;
  group_description: string;
  resource_count: number;
  deleted: boolean;
}

export interface Site {
  name: string;
  site_id: number;
  facility: string;
  long_name: string;
  city: string;
  state: string;
  country: string;
  deleted: boolean;
}

export interface Facility {
  name: string;
  facility_id: number;
  institution_id: string;
  site_count: number;
  deleted: boolean;
}

export interface Institution {
  id: string;
  name: string;
  ror_id: string;
}

export interface Downtime {
  id: number;
  resource_group: string;
  resource: string;
  class: string;
  severity: string;
  description: string;
  start_time: string;
  end_time: string;
  created_time: string;
  services: string[];
}

export interface Project {
  name: string;
  project_id: string;
  pi_name: string;
  organization: string;
  field_of_science: string;
  sponsor_type: string;
  sponsor_name: string;
  deleted: boolean;
}

export interface ProjectDetail {
  name: string;
  id: string;
  description: string;
  department: string;
  field_of_science: string;
  field_of_science_id: string;
  organization: string;
  pi_name: string;
  institution_id: string;
  sponsor_type: string;
  sponsor_name: string;
  sponsor?: unknown;
}

export interface ResourceContact {
  contact_type: string;
  rank: string;
  name: string;
  id: string;
}
export interface ResourceService {
  name: string;
  description: string;
  details?: unknown;
}
export interface ResourceDetail {
  name: string;
  id: number;
  resource_group: string;
  site: string;
  facility: string;
  active: boolean;
  description: string;
  fqdn: string;
  dn: string;
  fqdn_aliases: string[];
  tags: string[];
  allowed_vos: string[];
  services: ResourceService[];
  contacts: ResourceContact[];
  downtimes: Downtime[];
  vo_ownership?: unknown;
  wlcg_information?: unknown;
}
export interface RGDetail {
  name: string;
  group_id: number;
  site: string;
  facility: string;
  production: boolean | null;
  support_center: string;
  group_description: string;
  deleted: boolean;
  resources: string[];
}
export interface SiteDetail {
  name: string;
  facility: string;
  long_name: string;
  description: string;
  address_line1: string;
  address_line2: string;
  city: string;
  state: string;
  country: string;
  zipcode: string;
  latitude?: number;
  longitude?: number;
  resource_groups: string[];
}
export interface FacilityDetail {
  name: string;
  facility_id: number;
  institution_id: string;
  deleted: boolean;
  sites: string[];
}

export interface OIDCConfig {
  issuer: string;
  client_id: string;
  has_secret: boolean;
}

export interface UserLabelT {
  id: string;
  display_name: string;
  username: string;
}

export interface UserSearchResult {
  id: string;
  display_name: string;
  legacy_contact_id: string;
  is_provisioned: boolean;
}

export interface AdminUser {
  id: string;
  display_name: string;
  username?: string;
  status: string;
  is_provisioned: boolean;
  roles?: string[];
  identities: {
    id: string;
    issuer: string;
    subject: string;
    email?: string;
    cilogon_id?: string;
  }[];
}

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
