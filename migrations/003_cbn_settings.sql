-- 003_cbn_settings.sql
-- CBN compliance settings and configuration

CREATE TABLE IF NOT EXISTS cbn_settings (
    id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    setting_key           VARCHAR(100) UNIQUE NOT NULL,
    setting_value         TEXT NOT NULL,
    description           TEXT,
    effective_date        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- CBN transaction limits
INSERT INTO cbn_settings (setting_key, setting_value, description) VALUES
    ('daily_transaction_limit_tier1', '500000', 'Daily transaction limit for tier 1 accounts (NGN)'),
    ('daily_transaction_limit_tier2', '5000000', 'Daily transaction limit for tier 2 accounts (NGN)'),
    ('daily_transaction_limit_tier3', '50000000', 'Daily transaction limit for tier 3 accounts (NGN)'),
    ('single_transaction_limit_nip', '10000000', 'Single NIP transfer limit (NGN)'),
    ('audit_retention_days', '2557', 'Number of days to retain audit logs (7 years ~2557 days including leap years per CBN)'),
    ('reporting_currency', 'NGN', 'Base reporting currency as per CBN regulation'),
    ('nibss_nip_enabled', 'true', 'Whether NIBSS NIP transfers are enabled'),
    ('transaction_timeout_seconds', '30', 'Transaction processing timeout in seconds')
ON CONFLICT (setting_key) DO NOTHING;

-- Compliance report table for regulatory submissions
CREATE TABLE IF NOT EXISTS compliance_reports (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    report_type    VARCHAR(50) NOT NULL,
    report_date    DATE NOT NULL,
    report_data    JSONB,
    status         VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    submitted_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_compliance_reports_type_date ON compliance_reports(report_type, report_date);
