import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "@/test/render";
import type { ProvisioningConfig } from "@wpmgr/api";
import { mockQueryResult } from "@/test/query-mocks";
import { useMe } from "@/features/auth/use-auth";
import { useProvisioningConfig } from "../provisioning/use-provisioning";
import { AddSiteChoiceDialog } from "./AddSiteChoiceDialog";

vi.mock("@/features/auth/use-auth", () => ({ useMe: vi.fn(), activeRole: () => "owner" }));
vi.mock("../provisioning/use-provisioning", () => ({ useProvisioningConfig: vi.fn() }));
vi.mock("../add-site-dialog", () => ({ AddSiteDialog: ({ onBack }: { onBack: () => void }) => <div>Managed connection form<button onClick={onBack}>Back to setup choices</button></div> }));
vi.mock("../provisioning/ProvisionWordPressDialog", () => ({ ProvisionWordPressDialog: ({ onBack }: { onBack: () => void }) => <div>New WordPress form<button onClick={onBack}>Back to setup choices</button></div> }));

beforeEach(() => {
  vi.mocked(useMe).mockReturnValue(mockQueryResult({ data: { user: { id: "u", email: "owner@example.com", name: "Owner", created_at: "", updated_at: "" }, memberships: [{ user_id: "u", tenant_id: "t", role: "owner" }], active_tenant_id: "t", hosted: false } }));
  vi.mocked(useProvisioningConfig).mockReturnValue(mockQueryResult({ data: { enabled: true } }));
});

describe("per-site setup choice", () => {
  it("lets one fleet connect managed sites and provision others", () => {
    renderWithProviders(<AddSiteChoiceDialog />);
    fireEvent.click(screen.getByRole("button", { name: "Add site" }));
    expect(screen.getByText(/share the same fleet/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Connect existing site" }));
    expect(screen.getByText("Managed connection form")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back to setup choices" }));
    fireEvent.click(screen.getByRole("button", { name: "Provision new site" }));
    expect(screen.getByText("New WordPress form")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back to setup choices" }));
    expect(screen.getByRole("button", { name: "Connect existing site" })).toBeEnabled();
  });
  it.each(["pending", "error", "disabled"] as const)("keeps managed connection available when provisioning is %s", state => {
    vi.mocked(useProvisioningConfig).mockReturnValue(state === "disabled" ? mockQueryResult<ProvisioningConfig>({ data: { enabled: false } }) : state === "pending" ? mockQueryResult<ProvisioningConfig>({ isPending: true }) : mockQueryResult<ProvisioningConfig>({ isError: true, error: new Error("unavailable") }));
    renderWithProviders(<AddSiteChoiceDialog />);
    fireEvent.click(screen.getByRole("button", { name: "Add site" }));
    expect(screen.queryByRole("button", { name: "Provision new site" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Connect existing site" }));
    expect(screen.getByText("Managed connection form")).toBeInTheDocument();
  });
});
