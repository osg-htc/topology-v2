"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api, VODoc } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";
import { MultiSelect } from "@/components/MultiSelect";
import { ContactPicker } from "@/components/ContactPicker";

// The contact types real VO files use (all 122 of them).
const CONTACT_TYPES = [
  "Administrative Contact",
  "Security Contact",
  "Registration Authority",
  "VO Manager",
  "Sponsors",
  "Miscellaneous Contact",
];
const NAME_RE = /^[A-Za-z0-9][A-Za-z0-9_.-]*$/;

type Row = { type: string; name: string; id: string; inviteId?: string; invitePending?: boolean; inviteUrl?: string };

const asStrings = (v: unknown): string[] | null =>
  Array.isArray(v) && v.every((x) => typeof x === "string") ? (v as string[]) : null;

function NewVOForm() {
  const router = useRouter();
  const editName = useSearchParams().get("edit");
  const [name, setName] = useState(editName ?? "");
  const [f, setF] = useState({
    LongName: "", Community: "", AppDescription: "",
    PrimaryURL: "", PurposeURL: "", SupportURL: "", MembershipServicesURL: "",
  });
  const [certificateOnly, setCertificateOnly] = useState(false);
  const [active, setActive] = useState(true);
  const [disable, setDisable] = useState(false);
  const [primary, setPrimary] = useState<string[]>([]);
  const [secondary, setSecondary] = useState<string[]>([]);
  const [parent, setParent] = useState("");
  const [groups, setGroups] = useState<string[]>([]);
  const [rows, setRows] = useState<Row[]>([]);
  // The original document: used to decide whether an emptied field means
  // "remove it" (it had a value) or "nothing to say" (it never did).
  const [orig, setOrig] = useState<VODoc>({});
  // Values the form can't model faithfully (e.g. ReportingGroups stored in a
  // legacy map shape on a few VOs) are left out of the submission entirely, so
  // they are preserved rather than rewritten.
  const [groupsOpaque, setGroupsOpaque] = useState(false);
  const [advanced, setAdvanced] = useState(false);
  const [advancedJSON, setAdvancedJSON] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [showErrors, setShowErrors] = useState(false);

  const { data: editData } = useQuery({
    queryKey: ["vo-document", editName],
    queryFn: () => api.voDocument(editName!),
    enabled: !!editName,
  });
  const { data: voNames } = useQuery({ queryKey: ["vo-names"], queryFn: api.voNames });
  const { data: groupNames } = useQuery({ queryKey: ["reporting-group-names"], queryFn: api.reportingGroupNames });

  useEffect(() => {
    if (!editData) return;
    const d = editData.vo;
    setOrig(d);
    setName(editData.name);
    setF({
      LongName: d.LongName ?? "", Community: d.Community ?? "", AppDescription: d.AppDescription ?? "",
      PrimaryURL: d.PrimaryURL ?? "", PurposeURL: d.PurposeURL ?? "", SupportURL: d.SupportURL ?? "",
      MembershipServicesURL: d.MembershipServicesURL ?? "",
    });
    setCertificateOnly(d.CertificateOnly === true);
    setActive(d.Active !== false); // absent means active, as in v1
    setDisable(d.Disable === true || editData.disable);
    setPrimary(asStrings(d.FieldsOfScience?.PrimaryFields) ?? []);
    setSecondary(asStrings(d.FieldsOfScience?.SecondaryFields) ?? []);
    setParent(d.ParentVO?.Name ?? "");
    const g = asStrings(d.ReportingGroups);
    setGroups(g ?? []);
    setGroupsOpaque(d.ReportingGroups != null && g === null);
    setRows(
      Object.entries(d.Contacts ?? {}).flatMap(([type, people]) =>
        (people ?? []).map((p) => ({ type, name: p.Name ?? "", id: p.ID ?? "" })),
      ),
    );
  }, [editData]);

  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setF({ ...f, [k]: e.target.value });

  const typeOptions = Array.from(new Set([...CONTACT_TYPES, ...rows.map((r) => r.type)]));
  const contactsResolved = rows.every((r) => (!r.name && !r.id) || !!r.id || !!r.invitePending);
  const bad = {
    name: !editName && !NAME_RE.test(name),
    longName: !f.LongName.trim(),
    community: !f.Community.trim(),
    contacts: !contactsResolved,
  };
  const errCls = (b: boolean) => (showErrors && b ? " border-red-500 ring-1 ring-red-400" : "");

  // A null removes a key; an absent key is left exactly as it is. Optional text
  // fields therefore send their value, or null only if the VO had one to remove.
  const buildVO = (): Record<string, unknown> => {
    const vo: Record<string, unknown> = {};
    const text = (k: keyof typeof f) => {
      const v = f[k].trim();
      if (v) vo[k] = v;
      else if (orig[k] != null) vo[k] = null;
    };
    (Object.keys(f) as (keyof typeof f)[]).forEach(text);
    // A boolean is written only if the file already has it or it differs from
    // its default (Active true, Disable false, CertificateOnly false), so
    // submitting the form never adds a redundant key to a VO's file.
    const bool = (k: "CertificateOnly" | "Active" | "Disable", v: boolean, def: boolean) => {
      if (orig[k] != null || v !== def) vo[k] = v;
    };
    bool("CertificateOnly", certificateOnly, false);
    bool("Active", active, true);
    bool("Disable", disable, false);

    if (primary.length || secondary.length) {
      const fos: Record<string, string[]> = {};
      if (primary.length) fos.PrimaryFields = primary;
      if (secondary.length) fos.SecondaryFields = secondary;
      vo.FieldsOfScience = fos;
    } else if (orig.FieldsOfScience != null) vo.FieldsOfScience = null;

    if (!groupsOpaque) {
      if (groups.length) vo.ReportingGroups = groups;
      else if (orig.ReportingGroups != null) vo.ReportingGroups = null;
    }

    if (parent) vo.ParentVO = { Name: parent };
    else if (orig.ParentVO != null) vo.ParentVO = null;

    const contacts: Record<string, { ID: string; Name: string }[]> = {};
    for (const r of rows) {
      if (!r.name && !r.id) continue;
      (contacts[r.type] = contacts[r.type] || []).push({ ID: r.id, Name: r.name });
    }
    if (Object.keys(contacts).length) vo.Contacts = contacts;
    else if (orig.Contacts != null) vo.Contacts = null;
    return vo;
  };

  const toggleAdvanced = () => {
    if (!advanced) setAdvancedJSON(JSON.stringify(editName ? { ...orig, ...buildVO() } : buildVO(), null, 2));
    setAdvanced(!advanced);
  };

  const submit = async (asDraft: boolean) => {
    setErr("");
    if (!asDraft && !advanced && (bad.name || bad.longName || bad.community || bad.contacts)) {
      setShowErrors(true);
      setErr("Please fix the highlighted fields.");
      return;
    }
    setBusy(true);
    try {
      const vo = advanced ? JSON.parse(advancedJSON) : buildVO();
      const res = await api.proposals.create({
        entity_kind: "vo",
        operation: editName ? "update" : "create",
        target_name: editName ?? undefined,
        submit: !asDraft,
        proposed_state: { name, vo },
        pending_invite_ids: rows.filter((r) => r.invitePending && r.inviteId).map((r) => r.inviteId!),
      });
      router.push(`/proposals/view?id=${res.id}`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const setRow = (i: number, patch: Partial<Row>) => setRows(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  const move = (i: number, dir: -1 | 1) => {
    const j = i + dir;
    if (j < 0 || j >= rows.length) return;
    const next = [...rows];
    [next[i], next[j]] = [next[j], next[i]];
    setRows(next);
  };
  const field = (k: keyof typeof f, title: string, extra = "", placeholder = "") => (
    <div>
      <label className={label}>{title}</label>
      <input className={input + extra} value={f[k]} onChange={set(k)} placeholder={placeholder} />
    </div>
  );

  return (
    <div className="p-8">
      <PageHeader title={editName ? `Edit VO: ${editName}` : "New VO"} />
      <div className="max-w-2xl space-y-6">
        <Card>
          <h3 className="mb-3 text-sm font-semibold text-gray-700">Basics</h3>
          <div className="space-y-4">
            <div>
              <label className={label}>Name</label>
              {editName ? (
                <>
                  <input className={`${input} bg-gray-50 text-gray-500`} value={name} disabled />
                  <p className="mt-1 text-xs text-gray-400">A VO&apos;s name is its file name and can&apos;t be changed. Its ID never changes either.</p>
                </>
              ) : (
                <>
                  <input className={input + errCls(bad.name)} value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. MyExperiment" />
                  {name && !NAME_RE.test(name) && <p className="mt-1 text-xs text-red-600">Use letters, digits, “_”, “-” or “.”, starting with a letter or digit.</p>}
                </>
              )}
            </div>
            {field("LongName", "Long name", errCls(bad.longName))}
            {field("Community", "Community", errCls(bad.community))}
            <div>
              <label className={label}>Application description</label>
              <textarea className={input} rows={3} value={f.AppDescription} onChange={set("AppDescription")} />
            </div>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
              Active
            </label>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={disable} onChange={(e) => setDisable(e.target.checked)} />
              Disabled (independent of Active)
            </label>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={certificateOnly} onChange={(e) => setCertificateOnly(e.target.checked)} />
              Certificate only
            </label>
          </div>
        </Card>

        <Card>
          <h3 className="mb-3 text-sm font-semibold text-gray-700">URLs</h3>
          <div className="space-y-4">
            {field("PrimaryURL", "Primary URL", "", "https://…")}
            {field("PurposeURL", "Purpose URL")}
            {field("SupportURL", "Support URL")}
            {field("MembershipServicesURL", "Membership services URL")}
          </div>
        </Card>

        <Card>
          <h3 className="mb-3 text-sm font-semibold text-gray-700">Science &amp; relationships</h3>
          <div className="space-y-4">
            <div>
              <label className={label}>Primary fields of science</label>
              <MultiSelect options={[]} value={primary} onChange={setPrimary} placeholder="Add a field and press Enter…" />
            </div>
            <div>
              <label className={label}>Secondary fields of science</label>
              <MultiSelect options={[]} value={secondary} onChange={setSecondary} placeholder="Add a field and press Enter…" />
            </div>
            <div>
              <label className={label}>Parent VO</label>
              <select className={input} value={parent} onChange={(e) => setParent(e.target.value)}>
                <option value="">None</option>
                {parent && !(voNames ?? []).includes(parent) && <option value={parent}>{parent} (not in registry)</option>}
                {(voNames ?? []).filter((n) => n !== name).map((n) => (
                  <option key={n} value={n}>{n}</option>
                ))}
              </select>
            </div>
            <div>
              <label className={label}>Reporting groups</label>
              {groupsOpaque ? (
                <p className="text-xs text-gray-500">This VO stores its reporting groups in a legacy format. It is kept as-is; change it with the advanced editor below.</p>
              ) : (
                <MultiSelect options={groupNames ?? []} value={groups} onChange={setGroups} allowCustom={false} placeholder="Pick a reporting group…" />
              )}
            </div>
          </div>
        </Card>

        <Card>
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-700">Contacts</h3>
            <button
              className="text-xs text-brand-600 hover:underline"
              onClick={() => setRows([...rows, { type: CONTACT_TYPES[0], name: "", id: "" }])}
            >
              + Add contact
            </button>
          </div>
          {showErrors && bad.contacts && (
            <p className="mb-2 text-xs text-red-600">Every contact must be linked to a person — search and select, or invite a new one.</p>
          )}
          <div className="space-y-2">
            {rows.map((r, i) => (
              <div key={`${i}-${r.id}`} className="flex items-start gap-2">
                <div className="flex flex-col pt-2 text-gray-400">
                  <button type="button" className="leading-none hover:text-gray-700 disabled:opacity-30" disabled={i === 0} onClick={() => move(i, -1)} aria-label="Move up">▲</button>
                  <button type="button" className="leading-none hover:text-gray-700 disabled:opacity-30" disabled={i === rows.length - 1} onClick={() => move(i, 1)} aria-label="Move down">▼</button>
                </div>
                <div className="grid flex-1 grid-cols-2 gap-2">
                  <select className={input} value={r.type} onChange={(e) => setRow(i, { type: e.target.value })}>
                    {typeOptions.map((t) => (<option key={t} value={t}>{t}</option>))}
                  </select>
                  <ContactPicker value={r} onChange={(patch) => setRow(i, patch)} />
                </div>
                <button type="button" className="pt-2 text-gray-300 hover:text-red-600" onClick={() => setRows(rows.filter((_, j) => j !== i))} aria-label="Remove contact">×</button>
              </div>
            ))}
            {rows.length === 0 && <p className="text-xs text-gray-400">No contacts.</p>}
          </div>
          <p className="mt-2 text-xs text-gray-400">
            Contacts already on this VO stay as they are, even if they aren&apos;t linked to a user yet. Anyone you add must be a person you pick or invite.
          </p>
        </Card>

        <Card>
          <button className="text-xs text-gray-500 hover:text-gray-800" onClick={toggleAdvanced}>
            {advanced ? "▾ Hide advanced (raw JSON)" : "▸ Advanced edit (raw JSON — OASIS, credentials, data federations)"}
          </button>
          {advanced && (
            <div className="mt-3">
              <p className="mb-2 text-xs text-gray-400">
                This is the VO&apos;s whole document and overrides the form on submit. A key you leave out is kept as it is;
                set a key to <code>null</code> to remove it.
              </p>
              <textarea className={`${input} font-mono text-xs`} rows={16} value={advancedJSON} onChange={(e) => setAdvancedJSON(e.target.value)} />
            </div>
          )}
        </Card>

        {err && <p className="text-sm text-red-600">{err}</p>}
        <div className="flex gap-3">
          <button className={btn} disabled={busy || !name} onClick={() => submit(false)}>Submit for review</button>
          <button className={btnSecondary} disabled={busy || !name} onClick={() => submit(true)}>Save draft</button>
        </div>
      </div>
    </div>
  );
}

export default function NewVOPage() {
  return (
    <Suspense fallback={null}>
      <NewVOForm />
    </Suspense>
  );
}
