"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { input, label } from "./ui";

// A single operation inside a bundled change request.
export type BundleOp = {
  entity_kind: string;
  operation: "create" | "update" | "delete";
  target_name?: string;
  proposed_state: Record<string, unknown>;
};

// The resolved placement: the resource group name to use, plus the ordered
// parent-creation operations (facility -> site -> resource group) for any parts
// that don't exist yet. `valid` gates submission.
export type Placement = { rg: string; ops: BundleOp[]; valid: boolean };

// ParentChainPicker owns the resource-group selection. The user either picks an
// existing group or creates a new one inline; a new group may in turn create a
// new site, which may create a new facility. Everything created inline is
// returned as ordered bundle operations so the whole placement is registered in
// one atomic change request.
export function ParentChainPicker({
  value,
  onResolve,
  invalid,
}: {
  value: string;
  onResolve: (p: Placement) => void;
  invalid?: boolean;
}) {
  const { data: rgs } = useQuery({ queryKey: ["resource-groups", false], queryFn: () => api.resourceGroups() });
  const { data: sites } = useQuery({ queryKey: ["sites", false], queryFn: () => api.sites() });
  const { data: facilities } = useQuery({ queryKey: ["facilities", false], queryFn: () => api.facilities() });

  const [mode, setMode] = useState<"existing" | "new">("existing");
  const [existingRg, setExistingRg] = useState(value);

  // New RG fields
  const [rgName, setRgName] = useState("");
  const [siteMode, setSiteMode] = useState<"existing" | "new">("existing");
  const [existingSite, setExistingSite] = useState("");

  // New site fields
  const [siteName, setSiteName] = useState("");
  const [facMode, setFacMode] = useState<"existing" | "new">("existing");
  const [existingFac, setExistingFac] = useState("");
  const [facName, setFacName] = useState("");

  const rgNames = new Set((rgs ?? []).map((g) => g.name));
  const siteNames = new Set((sites ?? []).map((s) => s.name));
  const facNames = new Set((facilities ?? []).map((f) => f.name));

  // Recompute the resolved placement whenever inputs change and push it up.
  useEffect(() => {
    let p: Placement;
    if (mode === "existing") {
      p = { rg: existingRg, ops: [], valid: rgNames.has(existingRg) };
    } else {
      const ops: BundleOp[] = [];
      // Resolve the site (and facility) the new RG lives in.
      let siteForRg = "";
      if (siteMode === "existing") {
        siteForRg = existingSite;
      } else {
        siteForRg = siteName;
        let facForSite = "";
        if (facMode === "existing") {
          facForSite = existingFac;
        } else {
          facForSite = facName;
          if (facName) ops.push({ entity_kind: "facility", operation: "create", proposed_state: { name: facName, institution_id: "" } });
        }
        if (siteName) ops.push({ entity_kind: "site", operation: "create", proposed_state: { name: siteName, facility: facForSite, long_name: siteName } });
      }
      if (rgName) ops.push({ entity_kind: "resource_group", operation: "create", proposed_state: { name: rgName, site: siteForRg } });

      // Validity: names present, and any "existing" pick actually exists, and no
      // new name collides with an existing entity.
      const siteOK =
        siteMode === "existing"
          ? siteNames.has(existingSite)
          : !!siteName && !siteNames.has(siteName) && (facMode === "existing" ? facNames.has(existingFac) : !!facName && !facNames.has(facName));
      const valid = !!rgName && !rgNames.has(rgName) && siteOK;
      p = { rg: rgName, ops, valid };
    }
    onResolve(p);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, existingRg, rgName, siteMode, existingSite, siteName, facMode, existingFac, facName, rgs, sites, facilities]);

  const tab = (active: boolean) =>
    `rounded px-2 py-1 text-xs ${active ? "bg-brand-100 text-brand-800" : "text-gray-500 hover:bg-gray-100"}`;

  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <label className={`${label} mb-0`}>Resource group</label>
        <div className="flex gap-1">
          <button type="button" className={tab(mode === "existing")} onClick={() => setMode("existing")}>Existing</button>
          <button type="button" className={tab(mode === "new")} onClick={() => setMode("new")}>Create new</button>
        </div>
      </div>

      {mode === "existing" ? (
        <>
          <input
            className={input + (invalid ? " border-red-500 ring-1 ring-red-400" : "")}
            list="rg-options"
            value={existingRg}
            onChange={(e) => setExistingRg(e.target.value)}
            placeholder="Search resource groups…"
          />
          <datalist id="rg-options">
            {(rgs ?? []).map((g) => (
              <option key={g.name} value={g.name}>{g.site} · {g.facility}</option>
            ))}
          </datalist>
          {existingRg && !rgNames.has(existingRg) && (
            <p className="mt-1 text-xs text-amber-600">“{existingRg}” is not an existing group — pick one or switch to “Create new”.</p>
          )}
        </>
      ) : (
        <div className="space-y-3 rounded-md border border-brand-200 bg-brand-50/40 p-3">
          <div>
            <label className={label}>New resource group name</label>
            <input className={input} value={rgName} onChange={(e) => setRgName(e.target.value)} placeholder="e.g. UChicago_ClusterA" />
            {rgName && rgNames.has(rgName) && <p className="mt-1 text-xs text-red-600">A resource group named “{rgName}” already exists.</p>}
          </div>

          <div>
            <div className="mb-1 flex items-center justify-between">
              <label className={`${label} mb-0`}>Site</label>
              <div className="flex gap-1">
                <button type="button" className={tab(siteMode === "existing")} onClick={() => setSiteMode("existing")}>Existing</button>
                <button type="button" className={tab(siteMode === "new")} onClick={() => setSiteMode("new")}>Create new</button>
              </div>
            </div>
            {siteMode === "existing" ? (
              <>
                <input className={input} list="site-options" value={existingSite} onChange={(e) => setExistingSite(e.target.value)} placeholder="Search sites…" />
                <datalist id="site-options">
                  {(sites ?? []).map((s) => <option key={s.name} value={s.name}>{s.facility}</option>)}
                </datalist>
              </>
            ) : (
              <div className="space-y-3 rounded-md border border-brand-200 bg-white p-3">
                <div>
                  <label className={label}>New site name</label>
                  <input className={input} value={siteName} onChange={(e) => setSiteName(e.target.value)} placeholder="e.g. UChicago" />
                  {siteName && siteNames.has(siteName) && <p className="mt-1 text-xs text-red-600">A site named “{siteName}” already exists.</p>}
                </div>
                <div>
                  <div className="mb-1 flex items-center justify-between">
                    <label className={`${label} mb-0`}>Facility</label>
                    <div className="flex gap-1">
                      <button type="button" className={tab(facMode === "existing")} onClick={() => setFacMode("existing")}>Existing</button>
                      <button type="button" className={tab(facMode === "new")} onClick={() => setFacMode("new")}>Create new</button>
                    </div>
                  </div>
                  {facMode === "existing" ? (
                    <>
                      <input className={input} list="fac-options" value={existingFac} onChange={(e) => setExistingFac(e.target.value)} placeholder="Search facilities…" />
                      <datalist id="fac-options">
                        {(facilities ?? []).map((f) => <option key={f.name} value={f.name} />)}
                      </datalist>
                    </>
                  ) : (
                    <>
                      <input className={input} value={facName} onChange={(e) => setFacName(e.target.value)} placeholder="e.g. University of Chicago" />
                      {facName && facNames.has(facName) && <p className="mt-1 text-xs text-red-600">A facility named “{facName}” already exists.</p>}
                    </>
                  )}
                </div>
              </div>
            )}
          </div>
          <p className="text-xs text-gray-500">
            New parents are created together with this resource in one atomic change request.
          </p>
        </div>
      )}
    </div>
  );
}
