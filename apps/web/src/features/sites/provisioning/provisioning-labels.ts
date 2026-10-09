import type { ProvisioningRecord } from "@wpmgr/api";
export const statusLabels: Record<ProvisioningRecord["status"], string> = {
  queued: "Queued", running: "In progress", failed: "Needs attention", complete: "Complete",
};
export const stepLabels: Record<ProvisioningRecord["step"], string> = {
  queued: "Waiting for worker", database: "Creating isolated database", volume: "Preparing persistent storage", wordpress: "Starting WordPress", install: "Installing WordPress", agent: "Installing and enrolling Fleet Agent", https: "Verifying HTTPS", complete: "Ready",
};
