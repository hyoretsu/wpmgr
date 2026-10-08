import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { getSiteProvisioningCredentials } from "@wpmgr/api";
import { renderWithProviders } from "@/test/render";
import { ProvisioningCredentials } from "./ProvisioningCredentials";
vi.mock("@wpmgr/api", async original => ({ ...await original<typeof import("@wpmgr/api")>(), getSiteProvisioningCredentials: vi.fn() }));
vi.mock("@/components/toast", () => ({ toast: { success: vi.fn(), info: vi.fn() } }));
beforeEach(() => vi.clearAllMocks());
describe("provisioning credentials", () => {
  it("retrieves only on request, clears on hiding and unmount", async () => {
    vi.mocked(getSiteProvisioningCredentials).mockResolvedValue({ data: { admin_username: "site-admin", admin_password: "private-password", login_url: "https://example.com/wp-admin/" }, response: new Response(), request: new Request("https://example.com") });
    const view = renderWithProviders(<ProvisioningCredentials siteId="site-id" />);
    expect(getSiteProvisioningCredentials).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Show admin credentials" }));
    expect(await screen.findByText("private-password")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Hide admin credentials" }));
    expect(screen.queryByText("private-password")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show admin credentials" }));
    await waitFor(() => expect(screen.getByText("private-password")).toBeInTheDocument());
    view.unmount();
    renderWithProviders(<ProvisioningCredentials siteId="site-id" />);
    expect(screen.queryByText("private-password")).not.toBeInTheDocument();
  });
  it("shows retrieval error without revealing a password", async () => {
    vi.mocked(getSiteProvisioningCredentials).mockRejectedValue(new Error("Denied"));
    renderWithProviders(<ProvisioningCredentials siteId="site-id" />);
    fireEvent.click(screen.getByRole("button", { name: "Show admin credentials" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not retrieve credentials");
  });
});
