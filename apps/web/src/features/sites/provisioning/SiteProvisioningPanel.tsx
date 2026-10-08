import { useState } from "react";
import { Settings2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from "@/components/ui/dialog";
import { statusLabels } from "./provisioning-labels";
import { ProvisioningStatus } from "./ProvisioningStatus";
import { ProvisioningSkeleton } from "./ProvisioningSkeleton";
import { useProvisioningStatus } from "./use-provisioning";

export function SiteProvisioningPanel({ tenantId, siteId }: { tenantId: string; siteId: string }) {
  const status = useProvisioningStatus(tenantId, siteId);
  const [open, setOpen] = useState(false);
  if (status.isPending) return <ProvisioningSkeleton />;
  if (status.isError) return <p role="alert" className="text-sm text-destructive">Could not load provisioning status. Reload the site list to retry.</p>;
  if (!status.data) return null;
  return <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border p-3">
    <div><p className="text-sm font-medium">{status.data.domain}</p><p className="text-xs text-muted-foreground">WordPress provisioning: {statusLabels[status.data.status]}</p></div>
    <Button variant="outline" className="cursor-pointer" onClick={() => setOpen(true)}><Settings2 aria-hidden="true" />View provisioning</Button>
    <Dialog open={open} onClose={() => setOpen(false)}>{open ? <DialogContent ariaLabelledBy={`provision-status-${siteId}`}>
      <DialogHeader><DialogTitle id={`provision-status-${siteId}`}>WordPress provisioning</DialogTitle></DialogHeader>
      <DialogBody><ProvisioningStatus tenantId={tenantId} siteId={siteId} showCredentials /></DialogBody>
      <DialogFooter><Button variant="outline" onClick={() => setOpen(false)}>Close</Button></DialogFooter>
    </DialogContent> : null}</Dialog>
  </div>;
}
