-- Migration: Seed API keys and business config into system_config
-- Version: 20260410000002
-- Description: Adds all Category-A keys (API tokens, business config) that are
--              migrating from env vars to the system_config table.
--              Uses ON CONFLICT DO NOTHING so existing rows are preserved.

INSERT INTO system_config (key, value, description) VALUES
  ('HALOAI_API_URL',          '', 'HaloAI WhatsApp API base URL'),
  ('WA_API_TOKEN',            '', 'WhatsApp API bearer token (sensitive)'),
  ('HALOAI_BUSINESS_ID',      '', 'HaloAI business identifier'),
  ('HALOAI_CHANNEL_ID',       '', 'HaloAI channel identifier'),
  ('TELEGRAM_BOT_TOKEN',      '', 'Telegram bot token for notifications (sensitive)'),
  ('TELEGRAM_AE_LEAD_ID',     '', 'Telegram chat ID of AE Lead for escalations'),
  ('PROMO_DEADLINE',          '', 'Promo expiry date (YYYY-MM-DD)'),
  ('SURVEY_PLATFORM_URL',     '', 'NPS survey platform URL'),
  ('CHECKIN_FORM_URL',        '', 'Check-in form URL sent to clients'),
  ('REFERRAL_BENEFIT',        '1 bulan gratis', 'Referral reward copy'),
  ('QUOTATION_URL',           '', 'Quotation / pricing page URL'),
  ('ACV_HIGH_THRESHOLD',      '50000000', 'ACV threshold for High-value segment (IDR)'),
  ('ACV_MID_THRESHOLD',       '5000000',  'ACV threshold for Mid-value segment (IDR)'),
  ('SENIOR_AE_TELEGRAM_IDS',  '', 'Comma-separated Telegram IDs of senior AEs'),
  ('AE_TEAM_TELEGRAM_IDS',    '', 'Comma-separated Telegram IDs of all AEs'),
  ('ANGRY_KEYWORDS_EXTRA',    '', 'Extra comma-separated angry keywords for classifier'),
  ('SILENCE_THRESHOLD_DAYS',  '30', 'Days without reply before silence escalation'),
  ('JWT_VALIDATE_URL',        'https://api.sejutacita.id/v1/login/self', 'JWT validation endpoint')
ON CONFLICT (key) DO NOTHING;
