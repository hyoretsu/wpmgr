import { z } from "zod";

export const provisioningFormSchema = z.object({
  domain: z.string().trim().toLowerCase().min(1, "Enter a domain.")
    .max(253, "Domain is too long.")
    .regex(/^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/, "Enter a bare domain, such as example.com, without a URL or path."),
  title: z.string().trim().max(200, "Title is too long."),
  admin_username: z.string().trim().min(1, "Enter an admin username.")
    .max(60, "Username is too long.").regex(/^[a-zA-Z0-9_.@-]+$/, "Use letters, numbers, dots, underscores, @ or hyphens."),
  admin_email: z.email("Enter a valid admin email."),
});
export type ProvisioningFormValues = z.infer<typeof provisioningFormSchema>;
