// Human-readable summaries of a change request, so the list and review pages
// convey *what* is changing without opening each one.

type AnyRec = Record<string, unknown>;

export type ProposalLike = {
  entity_kind: string;
  operation: string;
  target_name?: string;
  proposed_state?: unknown;
};

const KIND_LABEL: Record<string, string> = {
  resource: "Resource",
  resource_group: "Resource group",
  site: "Site",
  facility: "Facility",
  project: "Project",
  downtime: "Downtime",
  bundle: "Bundle",
};

export function kindLabel(kind: string): string {
  return KIND_LABEL[kind] ?? kind;
}

function asRec(v: unknown): AnyRec {
  return v && typeof v === "object" ? (v as AnyRec) : {};
}

function count(v: unknown): number {
  return Array.isArray(v) ? v.length : v && typeof v === "object" ? Object.keys(v).length : 0;
}

// nameOf digs the display name out of a proposed_state for any entity kind.
function nameOf(kind: string, state: AnyRec, fallback?: string): string {
  if (typeof state.name === "string" && state.name) return state.name;
  const res = asRec(state.resource);
  if (typeof res.Name === "string" && res.Name) return res.Name;
  return fallback ?? "—";
}

// oneOp summarizes a single operation (used for both standalone and bundled).
// nameOf already prefers a name found in the payload over the raw target
// (which, for a resource, is now an immutable id rather than a name).
function oneOp(kind: string, operation: string, state: AnyRec, targetName?: string): string {
  const verb = operation === "create" ? "Create" : operation === "delete" ? "Delete" : "Update";
  const name = nameOf(kind, state, targetName);
  return `${verb} ${kindLabel(kind).toLowerCase()} “${name}”`;
}

export type ProposalSummary = {
  kind: string; // entity kind for the chip
  title: string; // primary affected entity name
  changes: string[]; // short bullet phrases
};

export function proposalSummary(p: ProposalLike): ProposalSummary {
  const state = asRec(p.proposed_state);

  if (p.entity_kind === "bundle") {
    const ops = Array.isArray(state.operations) ? (state.operations as AnyRec[]) : [];
    const changes = ops.map((op) =>
      oneOp(String(op.entity_kind), String(op.operation), asRec(op.proposed_state), op.target_name as string),
    );
    // Title = the leaf resource if present, else the last created entity.
    const resourceOp = ops.find((o) => o.entity_kind === "resource");
    const title = resourceOp
      ? nameOf("resource", asRec(resourceOp.proposed_state))
      : ops.length
        ? nameOf(String(ops[ops.length - 1].entity_kind), asRec(ops[ops.length - 1].proposed_state))
        : "bundle";
    return { kind: "bundle", title, changes };
  }

  if (p.operation === "delete") {
    // For a resource, target_name is now the immutable id, not a name -- a
    // delete payload carries an optional {name} purely for display (see
    // DeleteButton in entityActions.tsx), preferred here when present.
    const title = nameOf(p.entity_kind, state, p.target_name);
    return { kind: p.entity_kind, title, changes: [oneOp(p.entity_kind, "delete", state, p.target_name)] };
  }

  const title = nameOf(p.entity_kind, state, p.target_name);
  const changes: string[] = [];
  switch (p.entity_kind) {
    case "resource": {
      const res = asRec(state.resource);
      if (res.FQDN) changes.push(`host ${res.FQDN}`);
      if (state.resource_group) changes.push(`in ${state.resource_group}`);
      if (count(res.Services)) changes.push(`${count(res.Services)} service(s)`);
      if (count(res.ContactLists)) changes.push(`${count(res.ContactLists)} contact type(s)`);
      break;
    }
    case "resource_group":
      if (state.site) changes.push(`site ${state.site}`);
      if (count(state.contacts)) changes.push(`${count(state.contacts)} contact(s)`);
      break;
    case "site":
      if (state.facility) changes.push(`facility ${state.facility}`);
      if (state.city || state.country) changes.push([state.city, state.country].filter(Boolean).join(", "));
      break;
    case "facility":
      if (state.institution_id) changes.push("institution set");
      if (count(state.contacts)) changes.push(`${count(state.contacts)} contact(s)`);
      break;
    case "downtime":
      if (state.resource) changes.push(`on ${state.resource}`);
      if (state.class) changes.push(String(state.class).toLowerCase());
      if (state.severity) changes.push(String(state.severity));
      break;
    case "project":
      if (state.pi_name) changes.push(`PI ${state.pi_name}`);
      break;
  }
  if (changes.length === 0) changes.push(`${p.operation} ${kindLabel(p.entity_kind).toLowerCase()}`);
  return { kind: p.entity_kind, title, changes };
}
