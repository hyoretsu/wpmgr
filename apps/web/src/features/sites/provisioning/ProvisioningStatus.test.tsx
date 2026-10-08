import { fireEvent, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderWithProviders } from "@/test/render";
import { mockQueryResult, mockMutationResult } from "@/test/query-mocks";
import type { ProvisioningRecord } from "@wpmgr/api";
import { ProvisioningStatus } from "./ProvisioningStatus";
import { useProvisioningStatus, useRetryProvisioning } from "./use-provisioning";
vi.mock("./use-provisioning", () => ({ useProvisioningStatus: vi.fn(), useRetryProvisioning: vi.fn() }));
vi.mock("./ProvisioningCredentials", () => ({ ProvisioningCredentials: () => null }));
const record: ProvisioningRecord = { id: "job", site_id: "site", domain: "example.com", title: "Example", admin_username: "site-admin", admin_email: "admin@example.com", status: "failed", step: "https", error_code: "provisioning_step_failed", created_at: "2026-10-08T00:00:00Z", updated_at: "2026-10-08T00:00:00Z" };
beforeEach(() => {
  vi.mocked(useRetryProvisioning).mockReturnValue(mockMutationResult({}));
  vi.mocked(useProvisioningStatus).mockReturnValue(mockQueryResult({ data: record }));
});
describe("provisioning status", () => {
  it("keeps loading separate from missing state", () => {
    vi.mocked(useProvisioningStatus).mockReturnValue(mockQueryResult({ isPending: true, data: undefined }));
    renderWithProviders(<ProvisioningStatus tenantId="tenant" siteId="site" />);
    expect(screen.getByRole("status", { name: "Loading provisioning status" })).toBeInTheDocument();
    expect(screen.queryByText(/needs attention/)).not.toBeInTheDocument();
  });
  it("shows failed HTTPS step and queues retry", () => {
    const mutate = vi.fn();
    vi.mocked(useRetryProvisioning).mockReturnValue(mockMutationResult({ mutate }));
    renderWithProviders(<ProvisioningStatus tenantId="tenant" siteId="site" />);
    expect(screen.getByText("Current step: Verifying HTTPS")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Check domain DNS");
    fireEvent.click(screen.getByRole("button", { name: "Retry provisioning" }));
    expect(mutate).toHaveBeenCalled();
  });
});
