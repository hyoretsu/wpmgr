import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Button } from "@/components/ui/button";
import { DialogBody, DialogFooter } from "@/components/ui/dialog";
import { ProvisioningField } from "./ProvisioningField";
import { provisioningFormSchema, type ProvisioningFormValues } from "./form-schema";

interface Props { onSubmit: (values: ProvisioningFormValues) => void; onClose: () => void; pending: boolean; error?: string; onBack?: () => void; }
export function ProvisioningForm({ onSubmit, onClose, pending, error, onBack }: Props) {
  const form = useForm<ProvisioningFormValues>({ resolver: zodResolver(provisioningFormSchema), defaultValues: { domain: "", title: "", admin_username: "", admin_email: "" } });
  return <form noValidate onSubmit={event => {
      const fields = new FormData(event.currentTarget);
      for (const name of ["domain", "title", "admin_username", "admin_email"] as const) {
        const value = fields.get(name);
        form.setValue(name, typeof value === "string" ? value : "");
      }
      void form.handleSubmit(onSubmit)(event);
    }}>
    <DialogBody>
      <p className="text-sm text-muted-foreground">Point domain DNS at this Docker host. HTTPS and agent enrollment must finish before provisioning completes.</p>
      <Controller name="domain" control={form.control} render={({ field, fieldState }) => <ProvisioningField {...field} label="Domain" placeholder="example.com" required error={fieldState.error?.message} disabled={pending} />} />
      <Controller name="title" control={form.control} render={({ field, fieldState }) => <ProvisioningField {...field} label="Site title (optional)" placeholder="My WordPress site" error={fieldState.error?.message} disabled={pending} />} />
      <p className="text-xs text-muted-foreground">Title defaults to the domain.</p>
      <Controller name="admin_username" control={form.control} render={({ field, fieldState }) => <ProvisioningField {...field} label="Admin username" placeholder="site-admin" autoComplete="username" required error={fieldState.error?.message} disabled={pending} />} />
      <Controller name="admin_email" control={form.control} render={({ field, fieldState }) => <ProvisioningField {...field} label="Admin email" placeholder="admin@example.com" type="email" autoComplete="email" required error={fieldState.error?.message} disabled={pending} />} />
      {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
    </DialogBody>
    <DialogFooter>
      {onBack ? <Button type="button" variant="outline" className="cursor-pointer" disabled={pending} onClick={onBack}>Back to setup choices</Button> : null}
      <Button type="button" variant="outline" className="cursor-pointer" disabled={pending} onClick={onClose}>Cancel</Button>
      <Button type="submit" className="cursor-pointer" disabled={pending}>{pending ? "Starting provisioning..." : "Provision WordPress"}</Button>
    </DialogFooter>
  </form>;
}
