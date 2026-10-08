import { useState } from "react";
import { getSiteProvisioningCredentials, type ProvisioningCredentials as Credentials } from "@wpmgr/api";
import { KeyRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/toast";

export function ProvisioningCredentials({ siteId }: { siteId: string }) {
  const [credentials, setCredentials] = useState<Credentials | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const reveal = async () => {
    setPending(true); setError(null);
    try {
      const result = await getSiteProvisioningCredentials({ path: { siteId } });
      if (result.error || !result.data) throw new Error("Could not retrieve credentials. Check site access and retry.");
      setCredentials(result.data);
      toast.success("Admin credentials retrieved");
    } catch { setError("Could not retrieve credentials. Check site access and retry."); }
    finally { setPending(false); }
  };
  return <div className="space-y-2">
    <Button type="button" variant="outline" disabled={pending} className="cursor-pointer" onClick={() => {
      if (credentials) { setCredentials(null); toast.info("Admin credentials hidden"); }
      else void reveal();
    }}><KeyRound aria-hidden="true" />{pending ? "Retrieving credentials..." : credentials ? "Hide admin credentials" : "Show admin credentials"}</Button>
    {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
    {credentials ? <dl className="space-y-2 rounded-md border border-border p-3 text-sm">
      <div><dt className="text-muted-foreground">Username</dt><dd className="font-mono">{credentials.admin_username}</dd></div>
      <div><dt className="text-muted-foreground">Password</dt><dd className="break-all font-mono">{credentials.admin_password}</dd></div>
      <div><dt className="text-muted-foreground">Admin login</dt><dd><a href={credentials.login_url} target="_blank" rel="noreferrer" className="cursor-pointer underline underline-offset-4">Open WordPress admin</a></dd></div>
    </dl> : null}
  </div>;
}
