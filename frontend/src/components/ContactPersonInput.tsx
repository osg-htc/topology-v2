"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { input } from "./ui";

// ContactPersonInput selects a contact person. Admins search ALL users live;
// non-admins pick from a supplied list of known contacts (e.g. parent-scoped).
// On selecting a match, the contact id is filled automatically.
export function ContactPersonInput({
  name,
  id,
  onChange,
  isAdmin,
  fallback,
}: {
  name: string;
  id: string;
  onChange: (name: string, id: string) => void;
  isAdmin: boolean;
  fallback: { name: string; id: string }[];
}) {
  const [q, setQ] = useState(name);

  // Admin: live search across all users. Non-admin: filter the fallback list.
  const { data: results } = useQuery({
    queryKey: ["user-search", q],
    queryFn: () => api.admin.searchUsers(q),
    enabled: isAdmin && q.trim().length >= 2,
  });

  const options: { name: string; id: string }[] = isAdmin
    ? (results ?? []).map((u) => ({ name: u.display_name, id: u.legacy_contact_id }))
    : fallback.filter((c) => c.name.toLowerCase().includes(q.toLowerCase())).slice(0, 50);

  const listId = `people-${name}-${id}`;

  const pick = (val: string) => {
    const match = options.find((o) => o.name === val);
    setQ(val);
    onChange(val, match ? match.id : id);
  };

  return (
    <>
      <input
        className={input}
        list={listId}
        placeholder={isAdmin ? "Search people…" : "Contact"}
        value={q}
        onChange={(e) => pick(e.target.value)}
      />
      <datalist id={listId}>
        {options.map((o) => (
          <option key={`${o.name}-${o.id}`} value={o.name}>
            {o.id}
          </option>
        ))}
      </datalist>
    </>
  );
}
