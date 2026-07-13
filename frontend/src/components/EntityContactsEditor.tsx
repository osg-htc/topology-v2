"use client";

import { useQuery } from "@tanstack/react-query";
import { api, EntityContact } from "@/lib/api";
import { input, Card } from "./ui";
import { ContactPersonInput } from "./ContactPersonInput";

export const CONTACT_TYPES = [
  "Administrative Contact",
  "Security Contact",
  "Executive Contact",
  "Local Operational Contact",
  "Local Security Contact",
];
export const RANKS = ["Primary", "Secondary", "Tertiary"];

export type ContactRow = { type: string; name: string; id: string };

// toEntityContacts converts editor rows into the proposal `contacts` array,
// deriving rank from each contact's order within its type.
export function toEntityContacts(rows: ContactRow[]) {
  const perType: Record<string, number> = {};
  const out: { contact_type: string; rank: string; name: string; id: string }[] = [];
  for (const c of rows) {
    if (!c.name && !c.id) continue;
    const n = perType[c.type] ?? 0;
    perType[c.type] = n + 1;
    out.push({ contact_type: c.type, rank: RANKS[Math.min(n, RANKS.length - 1)], name: c.name, id: c.id });
  }
  return out;
}

// fromEntityContacts converts stored entity contacts (with rank) into editor
// rows ordered by type then rank.
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
  const { data: session } = useQuery({ queryKey: ["me"], queryFn: api.auth.me, retry: false });
  const isAdmin = session?.effective_role === "administrator";
  const { data: knownContacts } = useQuery({ queryKey: ["contacts"], queryFn: api.contacts });

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
  const rankLabel = (i: number) => {
    const t = rows[i].type;
    const n = rows.slice(0, i + 1).filter((c) => c.type === t).length - 1;
    return RANKS[Math.min(n, RANKS.length - 1)];
  };

  return (
    <Card>
      <h3 className="mb-3 text-sm font-semibold text-gray-700">Contacts</h3>
      <div className="space-y-2">
        {rows.map((c, i) => (
          <div key={i} className="flex items-center gap-2">
            <div className="flex flex-col text-gray-400">
              <button type="button" className="leading-none hover:text-gray-700 disabled:opacity-30" disabled={i === 0} onClick={() => move(i, -1)} aria-label="Move up">▲</button>
              <button type="button" className="leading-none hover:text-gray-700 disabled:opacity-30" disabled={i === rows.length - 1} onClick={() => move(i, 1)} aria-label="Move down">▼</button>
            </div>
            <span className="w-16 shrink-0 text-xs text-gray-400">{rankLabel(i)}</span>
            <div className="grid flex-1 grid-cols-3 gap-2">
              <select className={input} value={c.type} onChange={(e) => set(i, { type: e.target.value })}>
                {CONTACT_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
              <ContactPersonInput name={c.name} id={c.id} isAdmin={isAdmin} fallback={knownContacts ?? []} onChange={(nm, cid) => set(i, { name: nm, id: cid })} />
              <input className={input} placeholder="ID" value={c.id} onChange={(e) => set(i, { id: e.target.value })} />
            </div>
            <button type="button" className="text-gray-300 hover:text-red-600" onClick={() => remove(i)} aria-label="Remove">×</button>
          </div>
        ))}
        {rows.length === 0 && <p className="text-sm text-gray-400">No contacts at this level.</p>}
      </div>
      <button
        className="mt-2 text-xs text-brand-600 hover:underline"
        onClick={() => onChange([...rows, { type: "Administrative Contact", name: "", id: "" }])}
      >
        + Add contact
      </button>
      <p className="mt-1 text-xs text-gray-400">
        Contacts here are inherited by resources in this scope (per type, unless a resource overrides it).
        Order within a type sets its rank.
      </p>
    </Card>
  );
}
