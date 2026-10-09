-- Same-host provisioning records preserve resource ownership and retry secrets.
CREATE TABLE IF NOT EXISTS site_provisioning (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    site_id uuid NOT NULL UNIQUE,
    domain text NOT NULL UNIQUE CHECK (
        domain = lower(domain) AND length(domain) BETWEEN 3 AND 253
        AND domain ~ '^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$'
        AND position('.' in domain) > 0
        AND domain NOT LIKE '%..%'
    ),
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    admin_username text NOT NULL CHECK (length(admin_username) BETWEEN 1 AND 60),
    admin_email text NOT NULL CHECK (length(admin_email) BETWEEN 3 AND 254),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'failed', 'complete')),
    step text NOT NULL DEFAULT 'queued' CHECK (step IN ('queued', 'database', 'volume', 'wordpress', 'install', 'agent', 'https', 'complete')),
    encrypted_credentials bytea NOT NULL CHECK (octet_length(encrypted_credentials) > 0),
    config jsonb NOT NULL CHECK (jsonb_typeof(config) = 'object'),
    error_code text CHECK (error_code ~ '^[a-z][a-z0-9_]{0,63}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT site_provisioning_site_tenant_fkey FOREIGN KEY (site_id, tenant_id)
        REFERENCES sites(id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT site_provisioning_complete_check CHECK ((status = 'complete') = (step = 'complete')),
    CONSTRAINT site_provisioning_error_check CHECK ((status = 'failed') = (error_code IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS site_provisioning_tenant_idx ON site_provisioning (tenant_id);
ALTER TABLE site_provisioning ENABLE ROW LEVEL SECURITY;
ALTER TABLE site_provisioning FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public'
        AND tablename = 'site_provisioning' AND policyname = 'site_provisioning_tenant_isolation') THEN
        CREATE POLICY site_provisioning_tenant_isolation ON site_provisioning
            USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
            WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public'
        AND tablename = 'site_provisioning' AND policyname = 'site_provisioning_site_scope') THEN
        CREATE POLICY site_provisioning_site_scope ON site_provisioning AS RESTRICTIVE FOR ALL
            USING (coalesce(current_setting('app.site_scope', true), '') <> 'on'
                OR site_id = ANY(string_to_array(nullif(current_setting('app.allowed_site_ids', true), ''), ',')::uuid[]))
            WITH CHECK (coalesce(current_setting('app.site_scope', true), '') <> 'on'
                OR site_id = ANY(string_to_array(nullif(current_setting('app.allowed_site_ids', true), ''), ',')::uuid[]));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'public'
        AND tablename = 'site_provisioning' AND policyname = 'site_provisioning_agent') THEN
        CREATE POLICY site_provisioning_agent ON site_provisioning FOR SELECT
            USING (current_setting('app.agent', true) = 'on');
    END IF;
END;
$$;
GRANT SELECT, INSERT ON site_provisioning TO wpmgr_app;
REVOKE DELETE, TRUNCATE, UPDATE ON site_provisioning FROM wpmgr_app;
GRANT UPDATE (status, step, error_code, updated_at) ON site_provisioning TO wpmgr_app;
