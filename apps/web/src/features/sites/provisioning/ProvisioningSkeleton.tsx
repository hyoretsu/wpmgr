import { Skeleton } from "@/components/ui/skeleton";
export function ProvisioningSkeleton() {
  return <div role="status" aria-label="Loading provisioning status" aria-busy="true" className="space-y-3 rounded-lg border border-border p-4">
    <Skeleton className="h-5 w-48" /><Skeleton className="h-4 w-64" /><Skeleton className="h-9 w-36" />
  </div>;
}
