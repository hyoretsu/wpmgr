import { RotateCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/toast";
import { stepLabels } from "./provisioning-labels";
import { ProvisioningCredentials } from "./ProvisioningCredentials";
import { ProvisioningSkeleton } from "./ProvisioningSkeleton";
import { useProvisioningStatus, useRetryProvisioning } from "./use-provisioning";

export function ProvisioningStatus({ tenantId, siteId, showCredentials = false }: { tenantId: string; siteId: string; showCredentials?: boolean }) {
  const status = useProvisioningStatus(tenantId, siteId);
  const retry = useRetryProvisioning(tenantId, siteId);
  if (status.isPending) return <ProvisioningSkeleton />;
  if (status.isError) return <div className="space-y-2"><p role="alert" className="text-sm text-destructive">{status.error.message}</p><Button variant="outline" className="cursor-pointer" onClick={() => void status.refetch()}><RotateCw aria-hidden="true" />Reload provisioning status</Button></div>;
  const record = status.data;
  if (!record) return null;
  const failed = record.status === "failed";
  return <div className="space-y-3 rounded-lg border border-border p-4">
    <p className="text-sm font-medium">{record.domain}</p>
    <p className="text-sm" role="status">{record.status === "complete" ? "WordPress provisioned and enrolled" : failed ? "Provisioning needs attention" : "Provisioning WordPress"}</p>
    <p className="text-sm text-muted-foreground">Current step: {stepLabels[record.step]}</p>
    {failed ? <div className="space-y-2">
      <p role="alert" className="text-sm text-destructive">{record.error_code ? `Provisioning stopped (${record.error_code.replaceAll("_", " ")}). ` : "Provisioning stopped. "}Check domain DNS, proxy HTTPS issuance, and host provisioning configuration. Resources and credentials remain preserved.</p>
      <Button type="button" variant="outline" className="cursor-pointer" disabled={retry.isPending} onClick={() => retry.mutate(undefined, { onSuccess: () => toast.success("Provisioning retry queued"), onError: () => toast.error("Provisioning retry failed") })}><RotateCw aria-hidden="true" />{retry.isPending ? "Queueing retry..." : "Retry provisioning"}</Button>
      {retry.isError ? <p role="alert" className="text-sm text-destructive">{retry.error.message}</p> : null}
    </div> : null}
    {showCredentials ? <ProvisioningCredentials key={siteId} siteId={siteId} /> : null}
  </div>;
}
