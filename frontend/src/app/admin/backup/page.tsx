"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input } from "@/components/ui";

export default function AdminBackupPage() {
  const qc = useQueryClient();
  const [msg, setMsg] = useState("");
  const [ref, setRef] = useState("master");

  const { data, isLoading, error } = useQuery({
    queryKey: ["backups"],
    queryFn: api.admin.listBackups,
    retry: false,
  });

  const refresh = () => qc.invalidateQueries({ queryKey: ["backups"] });
  const run = (p: Promise<unknown>, ok: string) =>
    p.then(() => {
      setMsg(ok);
      refresh();
    }).catch((e) => setMsg(String(e)));

  const create = useMutation({ mutationFn: () => run(api.admin.createBackup(), "Backup created.") });
  const restore = useMutation({
    mutationFn: (key: string) => run(api.admin.restore(key), "Restored."),
  });
  const importGH = useMutation({
    mutationFn: () => run(api.admin.importGitHub(ref), "Imported from GitHub."),
  });

  if (error) {
    return (
      <div className="p-8">
        <PageHeader title="Backup & restore" />
        <Card>
          <p className="text-sm text-red-600">Administrator role required.</p>
        </Card>
      </div>
    );
  }

  return (
    <div className="p-8">
      <PageHeader
        title="Backup & restore"
        action={
          <button className={btn} disabled={create.isPending} onClick={() => create.mutate()}>
            Create backup
          </button>
        }
      />
      {msg && <p className="mb-4 text-sm text-gray-600">{msg}</p>}

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <h3 className="mb-3 text-sm font-semibold text-gray-700">S3 backups</h3>
          {isLoading ? (
            <p className="text-gray-400">Loading…</p>
          ) : (data?.backups?.length ?? 0) === 0 ? (
            <p className="text-sm text-gray-500">No backups yet.</p>
          ) : (
            <ul className="space-y-2">
              {data!.backups.map((key) => (
                <li key={key} className="flex items-center justify-between text-sm">
                  <span className="font-mono text-xs text-gray-600">{key}</span>
                  <button
                    className={btnSecondary}
                    disabled={restore.isPending}
                    onClick={() => {
                      if (confirm("Restore this backup? This replaces current topology data.")) {
                        restore.mutate(key);
                      }
                    }}
                  >
                    Restore
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card>
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
    </div>
  );
}
