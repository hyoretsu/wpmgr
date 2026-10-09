import { describe, expect, it } from "vitest";
import { provisioningFormSchema } from "./form-schema";
const input = { domain: "example.com", title: "", admin_username: "site-admin", admin_email: "admin@example.com" };
describe("provisioning form", () => {
  it("accepts bare domain and optional title", () => { expect(provisioningFormSchema.parse(input).title).toBe(""); });
  it("normalizes domain", () => { expect(provisioningFormSchema.parse({ ...input, domain: " EXAMPLE.COM " }).domain).toBe("example.com"); });
  it.each(["https://example.com", "example.com/path", "example.com:80", "-bad.example.com", "localhost", "127.0.0.1"])("rejects %s", domain => { expect(provisioningFormSchema.safeParse({ ...input, domain }).success).toBe(false); });
});
