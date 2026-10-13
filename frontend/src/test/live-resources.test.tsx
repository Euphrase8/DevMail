import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, cleanup } from "@testing-library/react";
import { afterEach, describe, it, expect, vi } from "vitest";
import { SettingsPage } from "@/components/mail/settings/settings-page";
import { MailProvider, useMail } from "@/components/mail/store";
import { api } from "@/lib/api";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function MockProvider({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MailProvider>{children}</MailProvider>
    </QueryClientProvider>
  );
}

function mockBackend() {
  const empty = [] as never[];
  vi.spyOn(api, "health").mockResolvedValue({
    status: "ok",
    db: "ok",
    time: "",
    version: "",
  });
  vi.spyOn(api, "listServers").mockResolvedValue(empty);
  vi.spyOn(api, "listDomains").mockResolvedValue(empty);
  vi.spyOn(api, "listUsers").mockResolvedValue(empty);
  vi.spyOn(api, "listJobs").mockResolvedValue(empty);
  vi.spyOn(api, "listAudit").mockResolvedValue(empty);
  vi.spyOn(api, "listProviders").mockResolvedValue(empty);
  vi.spyOn(api, "listOps").mockResolvedValue(empty);
  vi.spyOn(api, "dashboard").mockResolvedValue({
    servers: { total: 0, healthy: 0, degraded: 0, offline: 0, pending: 0, maintenance: 0 },
    domains: { total: 0, verified: 0, active: 0, degraded: 0, pending: 0, suspended: 0 },
    mailboxes: 0,
    aliases: 0,
    users: 0,
    recent: [],
  });
  vi.spyOn(api, "me").mockRejectedValue(new Error("anonymous"));
  vi.spyOn(api, "getSettings").mockResolvedValue({ name: "DevMail" });
}

function DispatchHarness() {
  const { save, remove } = useMail();
  return (
    <>
      <button
        onClick={() =>
          save("domains", {
            id: "domains-new",
            name: "example.test",
            status: "Pending",
            server: "",
            quota: "10",
            catchall: "Disabled",
          })
        }
      >
        save domain
      </button>
      <button onClick={() => remove("servers", "server-1")}>remove server</button>
    </>
  );
}

describe("Live resource UI", () => {
  it("persists a restored-page domain save through the API", async () => {
    mockBackend();
    const create = vi.spyOn(api, "createDomain").mockResolvedValue({
      id: "domain",
      name: "example.test",
      serverId: "",
      status: "pending",
      verificationToken: "",
      verificationName: "",
      verifiedAt: "",
      catchAll: false,
      quotaBytes: 10737418240,
      createdAt: "",
    });
    render(
      <MockProvider>
        <DispatchHarness />
      </MockProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "save domain" }));
    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({
        name: "example.test",
        serverId: undefined,
        quotaBytes: 10737418240,
        catchAll: false,
      }),
    );
  });

  it("routes restored-page deletions to the API", async () => {
    mockBackend();
    const del = vi.spyOn(api, "deleteServer").mockResolvedValue({ ok: true });
    render(
      <MockProvider>
        <DispatchHarness />
      </MockProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "remove server" }));
    await waitFor(() => expect(del).toHaveBeenCalledWith("server-1"));
  });

  it("saves workspace settings through the API from the restored settings page", async () => {
    mockBackend();
    const update = vi.spyOn(api, "updateSettings").mockResolvedValue({});
    render(
      <MockProvider>
        <SettingsPage />
      </MockProvider>,
    );
    await screen.findByRole("heading", { name: "Your workspace, your way." });
    fireEvent.change(screen.getByLabelText("name"), { target: { value: "Operations" } });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(update).toHaveBeenCalledWith({ name: "Operations" }));
  });
});
