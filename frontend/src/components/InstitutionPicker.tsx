"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { input, label } from "./ui";

// Public registry where new institutions are registered.
const REGISTRY_URL = "https://topology-institutions.osg-htc.org";

// InstitutionPicker requires the chosen value to be a real institution from the
// registry. The user searches by name; picking one sets its immutable id. If no
// match is found we link to the registry to register it and offer a
// rate-limited refresh to pull in a just-registered institution.
export function InstitutionPicker({
  value,
  initialName,
  onResolve,
  invalid,
}: {
  value: string; // institution id (iid_uri)
  initialName?: string;
  onResolve: (iid: string, valid: boolean) => void;
  invalid?: boolean;
}) {
  // `text` is what the user sees/types; `selected` is a confirmed pick.
  const [text, setText] = useState(initialName ?? value ?? "");
  const [selected, setSelected] = useState<{ iid: string; name: string } | null>(
    value ? { iid: value, name: initialName ?? value } : null,
  );
  const [open, setOpen] = useState(false);
  const [refreshMsg, setRefreshMsg] = useState("");

  const q = text.trim();
  const { data: results, refetch, isFetching } = useQuery({
    queryKey: ["institutions", q],
    queryFn: () => api.institutions(q),
    enabled: q.length >= 2 && !selected,
  });

  // An existing value (edit mode) counts as valid until the user changes it.
  useEffect(() => {
    if (value) onResolve(value, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const pick = (iid: string, name: string) => {
    setSelected({ iid, name });
    setText(name);
    setOpen(false);
    onResolve(iid, true);
  };

  const onType = (v: string) => {
    setText(v);
    setSelected(null);
    setOpen(true);
    onResolve("", false);
  };

  const refresh = async () => {
    setRefreshMsg("");
    const res = await api.refreshInstitutions();
    if (res.throttled) {
      setRefreshMsg(`Recently refreshed — try again in ~${res.retry_after_seconds}s.`);
    } else {
      setRefreshMsg(`Registry refreshed (${res.synced ?? 0} institutions).`);
      refetch();
    }
  };

  const noMatch = !selected && q.length >= 2 && !isFetching && (results ?? []).length === 0;

  return (
    <div>
      <label className={label}>Institution</label>
      <div className="relative">
        <input
          className={input + (invalid ? " border-red-500 ring-1 ring-red-400" : "")}
          value={text}
          onChange={(e) => onType(e.target.value)}
          onFocus={() => !selected && setOpen(true)}
          placeholder="Search your institution by name…"
        />
        {open && !selected && (results ?? []).length > 0 && (
          <ul className="absolute z-10 mt-1 max-h-56 w-full overflow-auto rounded-md border border-gray-200 bg-white shadow-sm">
            {(results ?? []).map((i) => (
              <li key={i.id}>
                <button
                  type="button"
                  className="block w-full px-3 py-1.5 text-left text-sm hover:bg-brand-50"
                  onClick={() => pick(i.id, i.name)}
                >
                  {i.name}
                  {i.ror_id && <span className="ml-2 text-xs text-gray-400">{i.ror_id}</span>}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {selected && (
        <p className="mt-1 text-xs text-gray-400">
          Institution id: <span className="font-mono">{selected.iid}</span>
        </p>
      )}

      {noMatch && (
        <div className="mt-1 rounded-md border border-amber-200 bg-amber-50 p-2 text-xs text-amber-800">
          No matching institution.{" "}
          <a href={REGISTRY_URL} target="_blank" rel="noopener noreferrer" className="underline">
            Register it in the institution registry
          </a>
          , then{" "}
          <button type="button" className="underline" onClick={refresh}>
            refresh
          </button>
          .{refreshMsg && <span className="ml-1 text-gray-600">{refreshMsg}</span>}
        </div>
      )}
    </div>
  );
}
