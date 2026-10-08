import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createProvisioning, getProvisioningConfig, getSiteProvisioning, retrySiteProvisioning, type CreateProvisioning } from "@wpmgr/api";
import { sitesKeys } from "../use-sites";

export const provisioningKeys = {
  all: ["provisioning"] as const,
  config: (tenantId: string) => ["provisioning", tenantId, "config"] as const,
  site: (tenantId: string, siteId: string) => ["provisioning", tenantId, "site", siteId] as const,
};
function requestError(error: unknown): Error {
  if (error instanceof Error) return error;
  if (error && typeof error === "object" && "message" in error && typeof error.message === "string") return new Error(error.message);
  return new Error("Could not reach provisioning service. Try again.");
}
export function useProvisioningConfig(tenantId: string) {
  return useQuery({ queryKey: provisioningKeys.config(tenantId), enabled: Boolean(tenantId), queryFn: async () => {
    const { data, error } = await getProvisioningConfig();
    if (error) throw requestError(error);
    if (!data) throw new Error("Empty provisioning configuration.");
    return data;
  } });
}
export function useProvisioningStatus(tenantId: string, siteId: string) {
  const cache = useQueryClient();
  return useQuery({ queryKey: provisioningKeys.site(tenantId, siteId), enabled: Boolean(tenantId && siteId), queryFn: async () => {
    const { data, error, response } = await getSiteProvisioning({ path: { siteId } });
    if (response.status === 404) return null;
    if (error) throw requestError(error);
    if (!data) throw new Error("Empty provisioning status.");
    if (data.status === "complete") void cache.invalidateQueries({ queryKey: sitesKeys.lists() });
    return data;
  }, refetchInterval: query => {
    const status = query.state.data?.status;
    return status && status !== "complete" && status !== "failed" ? 3000 : false;
  } });
}
export function useCreateProvisioning(tenantId: string) {
  const cache = useQueryClient();
  return useMutation({ mutationFn: async (body: CreateProvisioning) => {
    const { data, error } = await createProvisioning({ body });
    if (error) throw requestError(error);
    if (!data) throw new Error("Empty provisioning response.");
    return data;
  }, onSuccess: record => {
    cache.setQueryData(provisioningKeys.site(tenantId, record.site_id), record);
    void cache.invalidateQueries({ queryKey: sitesKeys.lists() });
  } });
}
export function useRetryProvisioning(tenantId: string, siteId: string) {
  const cache = useQueryClient();
  return useMutation({ mutationFn: async () => {
    const { data, error } = await retrySiteProvisioning({ path: { siteId } });
    if (error) throw requestError(error);
    if (!data) throw new Error("Empty retry response.");
    return data;
  }, onSuccess: record => {
    cache.setQueryData(provisioningKeys.site(tenantId, siteId), record);
    void cache.invalidateQueries({ queryKey: sitesKeys.lists() });
  } });
}
