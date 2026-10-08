import { useState } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from "@/components/ui/dialog";
import { useMe, activeRole } from "@/features/auth/use-auth";
import { ProvisioningForm } from "./ProvisioningForm";
import { ProvisioningStatus } from "./ProvisioningStatus";
import { useCreateProvisioning, useProvisioningConfig } from "./use-provisioning";

export function ProvisionWordPressDialog() {
  const { data: me } = useMe();
  const tenantId = me?.active_tenant_id ?? "";
  const config = useProvisioningConfig(tenantId);
  const create = useCreateProvisioning(tenantId);
  const [open, setOpen] = useState(false);
  const [siteId, setSiteId] = useState<string | null>(null);
  if (me?.hosted || activeRole(me) !== "owner") return null;
  if (config.isPending) return <div className="h-9 w-44 animate-pulse rounded-md border border-border bg-muted" aria-label="Loading provisioning availability" role="status" />;
  if (config.isError) return <Button variant="outline" className="cursor-pointer" onClick={() => void config.refetch()}>Reload provisioning availability</Button>;
  if (!config.data?.enabled) return null;
  return <>
    <Button variant="outline" className="cursor-pointer" onClick={() => { create.reset(); setSiteId(null); setOpen(true); }}><Plus aria-hidden="true" />Provision WordPress</Button>
    <Dialog open={open} onClose={() => setOpen(false)}>
      {open ? <DialogContent ariaLabelledBy="provision-title">
        <DialogHeader><DialogTitle id="provision-title">Provision WordPress</DialogTitle></DialogHeader>
        {siteId ? <><DialogBody><ProvisioningStatus tenantId={tenantId} siteId={siteId} showCredentials /></DialogBody><DialogFooter><Button variant="outline" className="cursor-pointer" onClick={() => setOpen(false)}>Close</Button></DialogFooter></> : <ProvisioningForm pending={create.isPending} error={create.isError ? create.error.message : undefined} onClose={() => setOpen(false)} onSubmit={values => create.mutate({ ...values, title: values.title || undefined }, { onSuccess: record => setSiteId(record.site_id) })} />}
      </DialogContent> : null}
    </Dialog>
  </>;
}
