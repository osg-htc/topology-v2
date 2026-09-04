"use client";

import { EntityContact } from "@/lib/api";
import { input, Card } from "./ui";
import { ContactPicker } from "./ContactPicker";

export const CONTACT_TYPES = [
  "Administrative Contact",
  "Security Contact",
  "Executive Contact",
  "Local Operational Contact",
  "Local Security Contact",
];
// Kept to order rows loaded from stored data and for slot-targeted invites; the
// contact editor no longer shows these labels.
export const RANKS = ["Primary", "Secondary", "Tertiary"];

// Rank is derived from a row's order within its type (1st = Primary, …), by
// position in the submitted list, not a stored field (see rankForOrder on the
// backend). Capped at 3 rows per type: past the 3rd, the backend clamps
// everything to "Tertiary", so a 4th same-type row would collide with the
// 3rd -- two live rows claiming the same slot -- rather than erroring.
const MAX_PER_TYPE = RANKS.length;

export type ContactRow = {
  type: string;
  name: string;
  id: string;
  inviteId?: string;
  invitePending?: boolean;
  inviteUrl?: string;
};

// toEntityContacts converts editor rows into the proposal `contacts` array, in
// order. Rank is intentionally omitted — the backend derives it from order.
export function toEntityContacts(rows: ContactRow[]) {
  return rows.filter((c) => c.name || c.id).map((c) => ({ contact_type: c.type, name: c.name, id: c.id }));
}

// pendingInviteIds collects the invite ids for any not-yet-onboarded contacts,
// so the proposal can be blocked from approval until they are accepted.
export function pendingInviteIds(rows: ContactRow[]): string[] {
  return rows.filter((c) => c.invitePending && c.inviteId).map((c) => c.inviteId!);
}

// allContactsResolved reports whether every non-blank row is either linked to
// a real person (a real id) or has a pending invite -- the gate a page
// should apply before allowing submission (matches what the backend enforces
// at apply time, so a submission that passes this never gets rejected on
// that ground later).
export function allContactsResolved(rows: ContactRow[]): boolean {
  return rows.every((c) => {
    if (!c.name && !c.id) return true;
    return !!c.id || !!c.invitePending;
  });
}

// fromEntityContacts converts stored entity contacts into editor rows ordered by
// type then rank (rank drives order but is not shown).
export function fromEntityContacts(cs?: EntityContact[]): ContactRow[] {
  if (!cs?.length) return [];
  const rankIdx = (r: string) => Math.max(0, RANKS.indexOf(r));
  return [...cs]
    .sort((a, b) => a.contact_type.localeCompare(b.contact_type) || rankIdx(a.rank) - rankIdx(b.rank))
    .map((c) => ({ type: c.contact_type, name: c.name, id: c.id }));
}

export function EntityContactsEditor({
  rows,
  onChange,
}: {
  rows: ContactRow[];
  onChange: (rows: ContactRow[]) => void;
}) {
  const set = (i: number, patch: Partial<ContactRow>) =>
    onChange(rows.map((c, j) => (j === i ? { ...c, ...patch } : c)));
  const remove = (i: number) => onChange(rows.filter((_, j) => j !== i));
  const move = (i: number, dir: -1 | 1) => {
    const j = i + dir;
    if (j < 0 || j >= rows.length) return;
    const next = [...rows];
    [next[i], next[j]] = [next[j], next[i]];
    onChange(next);
  };
  const countsByType = rows.reduce<Record<string, number>>((acc, c) => {
    acc[c.type] = (acc[c.type] ?? 0) + 1;
    return acc;
  }, {});
  return (
    <Card>
      <h3 className="mb-3 text-sm font-semibold text-gray-700">Contacts</h3>
      <div className="space-y-2">
        {rows.map((c, i) => (
          // See the resource form's identical comment: index-only keys leave
          // a row's ContactPersonInput mounted (and its text state stuck at
          // whatever it was on first mount) across the async prefill that
          // replaces blank default rows with real data. id changes exactly
          // when a row's identity actually changes, never mid-keystroke.
          <div key={`${i}-${c.id}`} className="flex items-start gap-2">
            <div className="flex flex-col pt-2 text-gray-400">
              <button type="button" className="leading-none hover:text-gray-700 disabled:opacity-30" disabled={i === 0} onClick={() => move(i, -1)} aria-label="Move up">▲</button>
              <button type="button" className="leading-none hover:text-gray-700 disabled:opacity-30" disabled={i === rows.length - 1} onClick={() => move(i, 1)} aria-label="Move down">▼</button>
            </div>
            <div className="grid flex-1 grid-cols-2 gap-2">
              <select className={input} value={c.type} onChange={(e) => set(i, { type: e.target.value })}>
                {CONTACT_TYPES.map((t) => (
                  <option key={t} value={t} disabled={t !== c.type && (countsByType[t] ?? 0) >= MAX_PER_TYPE}>
                    {t}
                  </option>
                ))}
              </select>
              <ContactPicker value={c} onChange={(patch) => set(i, patch)} />
            </div>
            <button type="button" className="pt-2 text-gray-300 hover:text-red-600" onClick={() => remove(i)} aria-label="Remove">×</button>
          </div>
        ))}
        {rows.length === 0 && <p className="text-sm text-gray-400">No contacts at this level.</p>}
      </div>
      <button
        className="mt-2 text-xs text-brand-600 hover:underline disabled:opacity-30"
        disabled={CONTACT_TYPES.every((t) => (countsByType[t] ?? 0) >= MAX_PER_TYPE)}
        onClick={() => {
          const openType = CONTACT_TYPES.find((t) => (countsByType[t] ?? 0) < MAX_PER_TYPE) ?? CONTACT_TYPES[0];
          onChange([...rows, { type: openType, name: "", id: "" }]);
        }}
      >
        + Add contact
      </button>
      <p className="mt-1 text-xs text-gray-400">
        Contacts here are inherited by resources in this scope (per type, unless a resource overrides it).
        Use ▲▼ to set the order within a type.
      </p>
    </Card>
  );
}
