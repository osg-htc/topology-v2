"use client";

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, input } from "@/components/ui";

export default function AdminRestorePage() {
  const [msg, setMsg] = useState("");
  const [ref, setRef] = useState("master");

  const importGH = useMutation({
    mutationFn: () => api.admin.importGitHub(ref),
    onSuccess: () => setMsg("Imported from GitHub."),
    onError: (e) => setMsg(String(e)),
  });

  return (
    <div className="p-8">
      <PageHeader title="Restore" />
      {msg && <p className="mb-4 text-sm text-gray-600">{msg}</p>}

      <Card className="max-w-xl">
        <h3 className="mb-3 text-sm font-semibold text-gray-700">Import from GitHub</h3>
        <p className="mb-3 text-xs text-gray-500">
          Pull the existing topology repository and replace the current data.
        </p>
        <div className="flex items-center gap-2">
          <input
            className={input}
            value={ref}
            onChange={(e) => setRef(e.target.value)}
            placeholder="git ref (e.g. master)"
          />
          <button
            className={btn}
            disabled={importGH.isPending}
            onClick={() => {
              if (confirm("Import from GitHub? This replaces current topology data.")) {
                importGH.mutate();
              }
            }}
          >
            Import
          </button>
        </div>
      </Card>
    </div>
  );
}
