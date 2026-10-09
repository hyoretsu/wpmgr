import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { renderWithProviders } from "@/test/render";
import { ProvisioningForm } from "./ProvisioningForm";

function renderForm(onSubmit = vi.fn(), pending = false) {
  renderWithProviders(<Dialog open onClose={vi.fn()}><DialogContent ariaLabelledBy="form-title"><DialogTitle id="form-title">New WordPress site</DialogTitle><ProvisioningForm onSubmit={onSubmit} onClose={vi.fn()} pending={pending} /></DialogContent></Dialog>);
  return onSubmit;
}
describe("ProvisioningForm", () => {
  it("submits current native values before debounce settles", async () => {
    const submit = renderForm();
    fireEvent.change(screen.getByLabelText(/Domain/), { target: { value: "example.com" } });
    fireEvent.change(screen.getByLabelText(/Admin username/), { target: { value: "site-admin" } });
    fireEvent.change(screen.getByLabelText(/Admin email/), { target: { value: "admin@example.com" } });
    fireEvent.submit(screen.getByRole("button", { name: "Create new WordPress" }).closest("form")!);
    await waitFor(() => expect(submit).toHaveBeenCalledWith({ domain: "example.com", title: "", admin_username: "site-admin", admin_email: "admin@example.com" }, expect.anything()));
  });
  it("reports invalid domain instead of starting", async () => {
    const submit = renderForm();
    fireEvent.change(screen.getByLabelText(/Domain/), { target: { value: "https://example.com" } });
    fireEvent.submit(screen.getByRole("button", { name: "Create new WordPress" }).closest("form")!);
    expect(await screen.findByText(/Enter a bare domain/)).toBeInTheDocument();
    expect(submit).not.toHaveBeenCalled();
  });
  it("disables repeated submission while pending", () => {
    renderForm(vi.fn(), true);
    expect(screen.getByRole("button", { name: "Starting provisioning..." })).toBeDisabled();
  });
});
