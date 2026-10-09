import { useState } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { AddSiteDialog } from "../add-site-dialog";
import { ProvisionWordPressDialog } from "../provisioning/ProvisionWordPressDialog";
import { SiteSetupChoices } from "./SiteSetupChoices";

export function AddSiteChoiceDialog() {
  const [step, setStep] = useState<"choice" | "connect" | "provision" | null>(null);
  const close = () => setStep(null);
  const back = () => setStep("choice");
  return <>
    <Button type="button" className="cursor-pointer" onClick={() => setStep("choice")}><Plus aria-hidden="true" />Add site</Button>
    <Dialog open={step === "choice"} onClose={close}>
      {step === "choice" ? <DialogContent ariaLabelledBy="site-setup-title">
        <DialogHeader>
          <DialogTitle id="site-setup-title">Add site</DialogTitle>
          <DialogDescription>Choose setup for this site. Existing and provisioned sites can share the same fleet.</DialogDescription>
        </DialogHeader>
        <SiteSetupChoices onConnect={() => setStep("connect")} onProvision={() => setStep("provision")} onClose={close} />
      </DialogContent> : null}
    </Dialog>
    {step === "connect" ? <AddSiteDialog open onClose={close} onBack={back} /> : null}
    {step === "provision" ? <ProvisionWordPressDialog open onClose={close} onBack={back} /> : null}
  </>;
}
