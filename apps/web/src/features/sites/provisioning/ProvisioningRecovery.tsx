import { useMe, activeRole } from "@/features/auth/use-auth";
import { SiteProvisioningPanel } from "./SiteProvisioningPanel";
import { useProvisioningConfig } from "./use-provisioning";

export function ProvisioningRecovery({ siteIds }: { siteIds: string[] }) {
  const { data: me } = useMe();
  const tenantId = me?.active_tenant_id ?? "";
  const config = useProvisioningConfig(tenantId);
  if (me?.hosted || activeRole(me) !== "owner" || !config.data?.enabled) return null;
  return <div className="space-y-2">{siteIds.map(siteId => <SiteProvisioningPanel key={siteId} tenantId={tenantId} siteId={siteId} />)}</div>;
}
