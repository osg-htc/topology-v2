// StructuredView renders a proposal's proposed_state as readable, nested
// sections — never raw JSON, and never overflowing to the right. Bundles (an
// `operations` array) render as an ordered list of change cards.
//
// Passing `other` turns on diff highlighting: every field whose value
// differs from the same path in `other` (added, removed, or changed) gets a
// highlighted background, all the way down to the leaf that actually
// differs -- not just the top-level section it lives in. Rendering the
// before/proposed columns of the same proposal with `other` set to each
// other is what lets a reviewer spot the one changed field in an otherwise
// identical object without reading every line.

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

// deepEqual compares by value, not key order -- two JSON objects with the
// same fields in a different order (e.g. from independently-built snapshots)
// must not show up as a false-positive diff.
function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (isPrimitive(a) || isPrimitive(b)) return false;
  const aArr = Array.isArray(a);
  const bArr = Array.isArray(b);
  if (aArr !== bArr) return false;
  if (aArr && bArr) {
    if (a.length !== b.length) return false;
    return a.every((v, i) => deepEqual(v, (b as unknown[])[i]));
  }
  const ao = a as Record<string, unknown>;
  const bo = b as Record<string, unknown>;
  const keys = new Set([...Object.keys(ao), ...Object.keys(bo)]);
  for (const k of keys) {
    if (!deepEqual(ao[k], bo[k])) return false;
  }
  return true;
}

function Scalar({ value, changed }: { value: unknown; changed?: boolean }) {
  const cls = changed ? "rounded bg-amber-100 px-1 text-amber-900" : "";
  if (value === null || value === undefined || value === "")
    return <span className={`text-gray-400 ${cls}`}>null</span>;
  if (typeof value === "boolean")
    return <span className={`text-gray-800 ${cls}`}>{value ? "yes" : "no"}</span>;
  return <span className={`whitespace-pre-wrap break-words text-gray-800 ${cls}`}>{String(value)}</span>;
}

// other is undefined when diffing is off (no highlighting anywhere); it is
// present-but-possibly-undefined once diffing is on, since a field can exist
// on one side and not the other.
function Node({ value, other, diffing }: { value: unknown; other?: unknown; diffing: boolean }) {
  if (isPrimitive(value)) return <Scalar value={value} changed={diffing && !deepEqual(value, other)} />;

  if (Array.isArray(value)) {
    if (value.length === 0) {
      const changed = diffing && !deepEqual(value, other);
      return <span className={`text-gray-400 ${changed ? "rounded bg-amber-100 px-1 text-amber-900" : ""}`}>[]</span>;
    }
    if (value.every(isPrimitive))
      return <Scalar value={`[${value.join(", ")}]`} changed={diffing && !deepEqual(value, other)} />;
    // Array of objects → stacked mini-cards, indented modestly, matched
    // against the other side by index (order isn't otherwise tracked). The
    // card itself never highlights -- only the leaf fields inside it do, so
    // one changed field doesn't paint the whole card as if everything in it
    // changed.
    const otherArr = Array.isArray(other) ? other : [];
    return (
      <div className="space-y-2">
        {value.map((v, i) => (
          <div key={i} className="rounded border border-gray-100 bg-gray-50/60 p-2">
            <ObjectBody value={v} other={otherArr[i]} diffing={diffing} />
          </div>
        ))}
      </div>
    );
  }

  return <ObjectBody value={value} other={other} diffing={diffing} />;
}

// ObjectBody lays out an object's fields. Scalars sit inline (label · value,
// wrapping); nested objects/arrays drop below the label with a left-border
// indent, so deep nesting never pushes content off-screen.
function ObjectBody({ value, other, diffing }: { value: unknown; other?: unknown; diffing: boolean }) {
  const obj = (value ?? {}) as Record<string, unknown>;
  const otherObj = (other ?? {}) as Record<string, unknown>;
  // Union of keys, not just this side's: a key the other side has and this
  // side omits is itself a diff worth surfacing (e.g. a field this snapshot
  // never had, or one an update just added).
  const keys = diffing
    ? Array.from(new Set([...Object.keys(obj), ...Object.keys(otherObj)]))
    : Object.keys(obj);
  const entries = keys.map((k) => [k, obj[k]] as const);
  if (entries.length === 0) return <span className="text-gray-400">{"{}"}</span>;
  return (
    <div className="space-y-1.5">
      {entries.map(([k, v]) => {
        const nested = !isPrimitive(v) && !(Array.isArray(v) && v.every(isPrimitive));
        // A nested section (object, or array of objects) never highlights as
        // a whole -- only the leaf fields inside it do, via the recursive
        // Node/ObjectBody calls below. Only a scalar row (the actual leaf)
        // gets highlighted here.
        if (nested) {
          return (
            <div key={k} className="min-w-0">
              <div className="text-xs font-medium uppercase tracking-wide text-gray-500">{labelize(k)}</div>
              <div className="mt-1 border-l-2 border-gray-100 pl-3">
                <Node value={v} other={otherObj[k]} diffing={diffing} />
              </div>
            </div>
          );
        }
        const changed = diffing && !deepEqual(v, otherObj[k]);
        const rowCls = changed ? "rounded bg-amber-50" : "";
        return (
          <div key={k} className={`flex flex-wrap items-baseline gap-x-2 -mx-1 px-1 ${rowCls}`}>
            <span className="shrink-0 text-xs font-medium uppercase tracking-wide text-gray-500">{labelize(k)}</span>
            <span className="min-w-0 text-sm">
              <Node value={v} other={otherObj[k]} diffing={diffing} />
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
                <p className="text-sm text-gray-600">
                  Delete “
                  {String(
                    (op.proposed_state as { name?: string } | null)?.name ?? op.target_name ?? "",
                  )}
                  ”.
                </p>
              ) : (
                <Node value={op.proposed_state} diffing={false} />
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// `other`, when passed, is the value at the same path on the "opposite side"
// (e.g. Proposed's value when rendering Before, and vice versa) -- pass it to
// turn on highlighting for whatever differs. Omit it to render plain, as
// before.
export function StructuredView({ value, other }: { value: unknown; other?: unknown }) {
  const v = (value ?? {}) as Record<string, unknown>;
  const diffing = other !== undefined;
  if (Array.isArray(v.operations)) {
    return (
      <div className="min-w-0 text-sm">
        <BundleView operations={v.operations as Record<string, unknown>[]} />
      </div>
    );
  }
  return (
    <div className="min-w-0 text-sm">
      <Node value={value} other={other} diffing={diffing} />
    </div>
  );
}
