"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, ContactableUser } from "@/lib/api";
import { input } from "./ui";

// ContactPersonInput selects a contact person by searching real users
// (GET /users/search, safe for any authenticated user) and requires an
// explicit pick from the results -- typing alone never resolves to anyone.
// Mirrors InstitutionPicker's selected/open pattern: a name that doesn't get
// clicked from the list must not silently keep whatever id was there before,
// so every keystroke clears the resolved reference rather than carrying over
// a stale one.
export function ContactPersonInput({
  name,
  id,
  onChange,
}: {
  name: string;
  id: string;
  onChange: (name: string, id: string) => void;
}) {
  const [text, setText] = useState(name);
  // `selected` is a confirmed pick; starts set only if this row already has
  // a real id (a prior pick, or an already-linked stored contact). A legacy
  // row with a display name but no id starts unselected, so editing it
  // requires searching and picking a real match to confirm it.
  const [selected, setSelected] = useState<ContactableUser | null>(
    id ? { id, display_name: name } : null,
  );
  const [open, setOpen] = useState(false);

  const q = text.trim();
  const { data: results } = useQuery({
    queryKey: ["user-search", q],
    queryFn: () => api.usersSearch(q),
    enabled: q.length >= 2 && !selected,
  });

  const pick = (u: ContactableUser) => {
    setSelected(u);
    setText(u.display_name);
    setOpen(false);
    onChange(u.display_name, u.id);
  };

  const onType = (v: string) => {
    setText(v);
    setSelected(null);
    setOpen(true);
    onChange(v, "");
  };

  const unlinked = !selected && text.trim() !== "";

  return (
    <div>
      <div className="relative">
        <input
          className={input}
          value={text}
          onChange={(e) => onType(e.target.value)}
          onFocus={() => !selected && setOpen(true)}
          placeholder="Search people…"
        />
        {open && !selected && (results ?? []).length > 0 && (
          <ul className="absolute z-10 mt-1 max-h-56 w-full overflow-auto rounded-md border border-gray-200 bg-white shadow-sm">
            {(results ?? []).map((u) => (
              <li key={u.id}>
                <button
                  type="button"
                  className="block w-full px-3 py-1.5 text-left text-sm hover:bg-brand-50"
                  onClick={() => pick(u)}
                >
                  {u.display_name}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      {unlinked && (
        <p className="mt-1 text-xs text-amber-600">Not linked to an account — search and select to confirm.</p>
      )}
    </div>
  );
}
