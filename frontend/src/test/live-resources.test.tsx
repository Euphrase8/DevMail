import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, cleanup } from "@testing-library/react";
import { afterEach, describe, it, expect, vi } from "vitest";
import { ApiResourcePage } from "@/components/mail/resources/api-resource-page";
import { api } from "@/lib/api";
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
function mount() {
  return render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <ApiResourcePage kind="domains" />
    </QueryClientProvider>,
  );
}
describe("Live resource UI", () => {
  it("saves a domain through the API and refreshes its list", async () => {
    vi.spyOn(api, "resourcePage").mockResolvedValue({
      data: [],
      meta: { page: 1, perPage: 25, total: 0 },
    });
    vi.spyOn(api, "listServers").mockResolvedValue([]);
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
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Add domain" }));
    fireEvent.change(screen.getByLabelText("name"), { target: { value: "example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({
        name: "example.test",
        serverId: "",
        quotaBytes: 10737418240,
        catchAll: false,
      }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
  it("keeps the form open and displays backend validation failures", async () => {
    vi.spyOn(api, "resourcePage").mockResolvedValue({
      data: [],
      meta: { page: 1, perPage: 25, total: 0 },
    });
    vi.spyOn(api, "listServers").mockResolvedValue([]);
    vi.spyOn(api, "createDomain").mockRejectedValue(new Error("Domain already exists."));
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Add domain" }));
    fireEvent.change(screen.getByLabelText("name"), { target: { value: "example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("Domain already exists."),
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});
