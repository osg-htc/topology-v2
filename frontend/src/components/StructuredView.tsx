// StructuredView renders a proposal's proposed_state as readable, nested
// sections — never raw JSON, and never overflowing to the right. Bundles (an
// `operations` array) render as an ordered list of change cards.

import { kindLabel } from "@/lib/proposalSummary";

function labelize(key: string): string {
  if (/^[A-Z]/.test(key)) return key; // already-capitalized legacy keys (FQDN, …)
  return key
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/^\w/, (c) => c.toUpperCase());
}

function isPrimitive(v: unknown): boolean {
  return v === null || typeof v !== "object";
}

function Scalar({ value }: { value: unknown }) {
  if (value === null || value === undefined || value === "")
    return <span className="text-gray-400">—</span>;
  if (typeof value === "boolean")
    return <span className="text-gray-800">{value ? "yes" : "no"}</span>;
  return <span className="whitespace-pre-wrap break-words text-gray-800">{String(value)}</span>;
}

function Node({ value }: { value: unknown }) {
  if (isPrimitive(value)) return <Scalar value={value} />;

  if (Array.isArray(value)) {
    if (value.length === 0) return <span className="text-gray-400">(none)</span>;
    if (value.every(isPrimitive)) return <Scalar value={value.join(", ")} />;
    // Array of objects → stacked mini-cards, indented modestly.
    return (
      <div className="space-y-2">
        {value.map((v, i) => (
          <div key={i} className="rounded border border-gray-100 bg-gray-50/60 p-2">
            <ObjectBody value={v} />
          </div>
        ))}
      </div>
    );
  }

  return <ObjectBody value={value} />;
}

// ObjectBody lays out an object's fields. Scalars sit inline (label · value,
// wrapping); nested objects/arrays drop below the label with a left-border
// indent, so deep nesting never pushes content off-screen.
function ObjectBody({ value }: { value: unknown }) {
  const entries = Object.entries((value ?? {}) as Record<string, unknown>);
  if (entries.length === 0) return <span className="text-gray-400">(empty)</span>;
  return (
    <div className="space-y-1.5">
      {entries.map(([k, v]) => {
        const nested = !isPrimitive(v) && !(Array.isArray(v) && v.every(isPrimitive));
        if (nested) {
          return (
            <div key={k} className="min-w-0">
              <div className="text-xs font-medium uppercase tracking-wide text-gray-500">{labelize(k)}</div>
              <div className="mt-1 border-l-2 border-gray-100 pl-3">
                <Node value={v} />
              </div>
            </div>
          );
        }
        return (
          <div key={k} className="flex flex-wrap items-baseline gap-x-2">
            <span className="shrink-0 text-xs font-medium uppercase tracking-wide text-gray-500">{labelize(k)}</span>
            <span className="min-w-0 text-sm">
              <Node value={v} />
            </span>
          </div>
        );
      })}
    </div>
  );
}

function BundleView({ operations }: { operations: Record<string, unknown>[] }) {
  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        {operations.length} operation{operations.length === 1 ? "" : "s"}, applied in order (all or nothing):
      </p>
      {operations.map((op, i) => {
        const kind = String(op.entity_kind ?? "");
        const operation = String(op.operation ?? "");
        return (
          <div key={i} className="rounded-lg border border-gray-200 p-3">
            <div className="mb-2 flex items-center gap-2">
              <span className="flex h-5 w-5 items-center justify-center rounded-full bg-brand-100 text-xs font-semibold text-brand-800">
                {i + 1}
              </span>
              <span className="text-sm font-medium capitalize text-navy-900">
                {operation} {kindLabel(kind).toLowerCase()}
              </span>
            </div>
            <div className="pl-7">
              {operation === "delete" ? (
                <p className="text-sm text-gray-600">Delete “{String(op.target_name ?? "")}”.</p>
              ) : (
                <Node value={op.proposed_state} />
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function StructuredView({ value }: { value: unknown }) {
  const v = (value ?? {}) as Record<string, unknown>;
  if (Array.isArray(v.operations)) {
    return (
      <div className="min-w-0 text-sm">
        <BundleView operations={v.operations as Record<string, unknown>[]} />
      </div>
    );
  }
  return (
    <div className="min-w-0 text-sm">
      <Node value={value} />
    </div>
  );
}
