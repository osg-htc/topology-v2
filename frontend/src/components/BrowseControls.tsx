"use client";

// Shared "show inactive" toggle for browse pages (active-only by default).
export function InactiveToggle({
  value,
  onChange,
}: {
  value: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="flex items-center gap-2 text-sm text-gray-600">
      <input type="checkbox" checked={value} onChange={(e) => onChange(e.target.checked)} />
      Show inactive
    </label>
  );
}
