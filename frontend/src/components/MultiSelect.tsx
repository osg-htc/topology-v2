"use client";

import { useState } from "react";
import { input } from "./ui";

// MultiSelect is a token/chip input backed by a datalist of known options. The
// user picks from the list (or types a value and presses Enter); selections show
// as removable chips. Used for tags and allowed VOs.
export function MultiSelect({
  options,
  value,
  onChange,
  placeholder,
  allowCustom = true,
}: {
  options: string[];
  value: string[];
  onChange: (next: string[]) => void;
  placeholder?: string;
  allowCustom?: boolean;
}) {
  const [text, setText] = useState("");
  const listId = `ms-${Math.abs(hashCode(placeholder ?? options.join(",")))}`;

  const add = (v: string) => {
    const t = v.trim();
    if (!t || value.includes(t)) return;
    if (!allowCustom && !options.includes(t)) return;
    onChange([...value, t]);
    setText("");
  };
  const remove = (v: string) => onChange(value.filter((x) => x !== v));

  return (
    <div>
      {value.length > 0 && (
        <div className="mb-2 flex flex-wrap gap-1.5">
          {value.map((v) => (
            <span
              key={v}
              className="inline-flex items-center gap-1 rounded-full bg-brand-100 px-2 py-0.5 text-xs text-brand-800"
            >
              {v}
              <button
                type="button"
                onClick={() => remove(v)}
                className="text-brand-600 hover:text-brand-900"
                aria-label={`Remove ${v}`}
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}
      <input
        className={input}
        list={listId}
        value={text}
        placeholder={placeholder}
        onChange={(e) => {
          // Selecting from the datalist fires a change with the full value.
          const v = e.target.value;
          if (options.includes(v)) {
            add(v);
          } else {
            setText(v);
          }
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            add(text);
          }
        }}
      />
      <datalist id={listId}>
        {options
          .filter((o) => !value.includes(o))
          .map((o) => (
            <option key={o} value={o} />
          ))}
      </datalist>
    </div>
  );
}

function hashCode(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (Math.imul(31, h) + s.charCodeAt(i)) | 0;
  return h;
}
