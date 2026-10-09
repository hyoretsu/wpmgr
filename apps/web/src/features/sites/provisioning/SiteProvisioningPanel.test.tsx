import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "@/test/render";
import { mockQueryResult } from "@/test/query-mocks";
import { useProvisioningStatus } from "./use-provisioning";
import { SiteProvisioningPanel } from "./SiteProvisioningPanel";

vi.mock("./use-provisioning", () => ({ useProvisioningStatus: vi.fn() }));
vi.mock("./ProvisioningStatus", () => ({ ProvisioningStatus: () => <p>Provisioned site progress</p> }));

describe("mixed fleet provisioning", () => {
  it("shows provisioning controls only for sites with a provisioning record", () => {
    vi.mocked(useProvisioningStatus).mockImplementation((_tenant, site) => mockQueryResult({ data: site === "managed" ? null : { id: "job", site_id: site, domain: "new.example.com", title: "New site", admin_username: "admin", admin_email: "admin@example.com", status: "complete", step: "complete", created_at: "", updated_at: "" } }));
    renderWithProviders(<><div data-testid="managed"><SiteProvisioningPanel tenantId="tenant" siteId="managed" /></div><SiteProvisioningPanel tenantId="tenant" siteId="provisioned" /></>);
    expect(screen.getByTestId("managed")).toBeEmptyDOMElement();
    expect(screen.getAllByRole("button", { name: "View provisioning" })).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "View provisioning" }));
    expect(screen.getByText("Provisioned site progress")).toBeInTheDocument();
  });
});
