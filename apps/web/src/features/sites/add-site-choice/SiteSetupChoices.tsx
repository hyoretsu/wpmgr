import { Link2, Server, RotateCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DialogBody, DialogFooter } from "@/components/ui/dialog";
import { activeRole, useMe } from "@/features/auth/use-auth";
import { useProvisioningConfig } from "../provisioning/use-provisioning";

export function SiteSetupChoices({ onConnect, onProvision, onClose }: { onConnect: () => void; onProvision: () => void; onClose: () => void }) {
  const { data: me } = useMe();
  const config = useProvisioningConfig(me?.active_tenant_id ?? "");
  const canProvision = Boolean(me && !me.hosted && activeRole(me) === "owner");
  return <>
    <DialogBody>
      <div className="space-y-2 rounded-lg border border-border p-4">
        <h3 className="font-medium">Connect existing WordPress</h3>
        <p className="text-sm text-muted-foreground">Manage a site hosted anywhere. Keep its hosting, database, and WordPress content; connect using Fleet Agent.</p>
        <Button variant="outline" className="cursor-pointer" onClick={onConnect}><Link2 aria-hidden="true" />Connect existing site</Button>
      </div>
      {canProvision ? <div className="space-y-2 rounded-lg border border-border p-4">
        <h3 className="font-medium">Provision new WordPress</h3>
        <p className="text-sm text-muted-foreground">Create WordPress on this configured Docker host with its own database, HTTPS, and Fleet Agent. Point your domain DNS here before starting.</p>
        {config.isPending ? <div role="status" aria-label="Loading provisioning availability" className="h-9 w-44 animate-pulse rounded-md border border-border bg-muted" /> : config.isError ? <><p role="alert" className="text-sm text-destructive">Could not check provisioning availability. You can still connect an existing site.</p><Button variant="outline" className="cursor-pointer" onClick={() => void config.refetch()}><RotateCw aria-hidden="true" />Retry availability</Button></> : config.data?.enabled ? <Button variant="outline" className="cursor-pointer" onClick={onProvision}><Server aria-hidden="true" />Provision new site</Button> : <p className="text-sm text-muted-foreground">Provisioning is optional and currently disabled. Ask your host operator to enable it.</p>}
      </div> : null}
    </DialogBody>
    <DialogFooter><Button variant="outline" className="cursor-pointer" onClick={onClose}>Cancel</Button></DialogFooter>
  </>;
}
