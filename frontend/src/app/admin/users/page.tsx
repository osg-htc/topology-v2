"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, AdminUser, ApiError } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";

const ROLES = ["administrator", "manager", "user", "contact_reader"];

export default function AdminUsersPage() {
  const qc = useQueryClient();
  const { data, isLoading, error } = useQuery({
    queryKey: ["admin-users"],
    queryFn: api.admin.listUsers,
    retry: false,
  });

  const setRole = useMutation({
    mutationFn: (v: { id: string; role: string; action: "add" | "remove" }) =>
      api.admin.setUserRole(v.id, v.role, v.action),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-users"] }),
  });
  const setUsername = useMutation({
    mutationFn: (v: { id: string; username: string }) => api.admin.setUsername(v.id, v.username),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-users"] }),
    onError: (e) => alert(String(e)),
  });

  if (error) {
    // Only a real 403 means "you're not an admin" -- any other failure must
    // show its own message, not blame the viewer's role.
    const forbidden = error instanceof ApiError && error.status === 403;
    return (
      <div className="p-8">
        <PageHeader title="Users" />
        <Card>
          <p className="text-sm text-red-600">
            {forbidden ? "Administrator role required." : String(error)}
          </p>
        </Card>
      </div>
    );
  }

  return (
    <div className="p-8">
      <PageHeader title="Users" />
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : (
        <div className="space-y-3">
          {(data ?? []).map((u: AdminUser) => (
            <Card key={u.id}>
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <div className="font-medium text-navy-900">
                    {u.display_name || "(no name)"}
                    {u.username && <span className="ml-1 text-sm text-gray-400">({u.username})</span>}
                    {u.is_provisioned && (
                      <span className="ml-2 rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-800">
                        provisioned
                      </span>
                    )}
                    <button
                      className="ml-2 text-xs text-brand-600 hover:underline"
                      onClick={() => {
                        const v = prompt(`Set username for ${u.display_name}`, u.username || "");
                        if (v && v !== u.username) setUsername.mutate({ id: u.id, username: v });
                      }}
                    >
                      edit username
                    </button>
                  </div>
                  <div className="text-xs text-gray-300" title={u.id}>
                    id: {u.id}
                  </div>
                  <div className="mt-1 space-y-0.5 text-xs text-gray-500">
                    {(u.identities ?? []).length === 0 && <div>no linked identities</div>}
                    {(u.identities ?? []).map((id) => (
                      <div key={id.id}>
                        {id.email || id.subject}
                        {id.cilogon_id ? ` · ${id.cilogon_id}` : ""}
                        <span className="text-gray-400"> ({id.issuer})</span>
                      </div>
                    ))}
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  {ROLES.map((role) => {
                    const has = (u.roles ?? []).includes(role);
                    return (
                      <button
                        key={role}
                        onClick={() =>
                          setRole.mutate({ id: u.id, role, action: has ? "remove" : "add" })
                        }
                        className={`rounded-full px-3 py-1 text-xs font-medium ${
                          has
                            ? "bg-brand-600 text-white"
                            : "border border-gray-300 text-gray-500 hover:bg-gray-50"
                        }`}
                      >
                        {role}
                      </button>
                    );
                  })}
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
