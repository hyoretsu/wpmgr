import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useDebouncedInput } from "@/hooks/use-debounced-input";

interface Props {
  name: string;
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
  onBlur: () => void;
  error?: string;
  required?: boolean;
  autoComplete?: string;
  type?: "text" | "email";
  disabled?: boolean;
}

export function ProvisioningField({ name, label, placeholder, value, onChange, onBlur, error, required, autoComplete, type = "text", disabled }: Props) {
  const [local, setLocal] = useDebouncedInput(value, onChange);
  const id = `provision-${name}`;
  return <div className="space-y-2">
    <Label htmlFor={id}>{label}{required ? <span className="ml-1 text-destructive" aria-hidden="true">*</span> : null}</Label>
    <Input id={id} name={name} type={type} value={local} placeholder={placeholder}
      autoComplete={autoComplete} required={required} disabled={disabled}
      onChange={event => setLocal(event.currentTarget.value)}
      onBlur={() => { onChange(local); onBlur(); }}
      aria-invalid={Boolean(error)} aria-describedby={error ? `${id}-error` : undefined} />
    {error ? <p id={`${id}-error`} role="alert" className="text-sm text-destructive">{error}</p> : null}
  </div>;
}
