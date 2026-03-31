-- Migration: Create templates table
-- Version: 20250330000008
-- Description: Creates the templates table for storing WhatsApp message templates

CREATE TABLE templates (
  template_id       VARCHAR(50)   PRIMARY KEY,
  template_name     VARCHAR(100)  NOT NULL,
  template_content  TEXT          NOT NULL,
  template_category VARCHAR(20)   NOT NULL,
  language          VARCHAR(10)   DEFAULT 'id',
  active            BOOLEAN       DEFAULT TRUE,
  created_at        TIMESTAMP     DEFAULT NOW(),
  updated_at        TIMESTAMP     DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX idx_templates_category ON templates(template_category);
CREATE INDEX idx_templates_active ON templates(active);
CREATE INDEX idx_templates_language ON templates(language);
CREATE INDEX idx_templates_name ON templates(template_name);

-- Insert initial templates
INSERT INTO templates (template_id, template_name, template_content, template_category) VALUES
  ('renewal_60d', 'Renewal 60 Days', 'Halo {{.pic_name}}, kontrak Kantorku.id Anda akan berakhir dalam 60 hari ({{.contract_end}}). Mari kita diskusikan perpanjangan.', 'renewal'),
  ('renewal_45d', 'Renewal 45 Days', 'Halo {{.pic_name}}, kontrak Anda akan berakhir dalam 45 hari. Sudah waktunya untuk perpanjangan.', 'renewal'),
  ('renewal_30d', 'Renewal 30 Days', 'Halo {{.pic_name}}, kontrak Anda akan berakhir dalam 30 hari. Apakah ada pertanyaan?', 'renewal'),
  ('renewal_15d', 'Renewal 15 Days', 'Halo {{.pic_name}}, hanya 15 hari lagi sebelum kontrak berakhir. Mari kita atur perpanjangan.', 'renewal'),
  ('renewal_0d', 'Renewal Due Today', 'Halo {{.pic_name}}, kontrak Anda berakhir hari ini. Segera hubungi kami untuk perpanjangan.', 'renewal'),
  ('checkin_a1_form', 'Check-in A1 Form', 'Halo {{.pic_name}}, bagaimana pengalaman Anda dengan Kantorku.id? Mohon isi form singkat ini: {{.form_link}}', 'checkin'),
  ('checkin_a1_call', 'Check-in A1 Call', 'Halo {{.pic_name}}, terima kasih sudah mengisi form. Bisakah kita schedule call singkat?', 'checkin'),
  ('invoice_pre14', 'Invoice Pre 14 Days', 'Halo {{.pic_name}}, invoice #{{.invoice_id}} sebesar Rp{{.amount}} akan jatuh tempo dalam 14 hari ({{.due_date}}).', 'invoice'),
  ('invoice_pre7', 'Invoice Pre 7 Days', 'Halo {{.pic_name}}, invoice #{{.invoice_id}} jatuh tempo dalam 7 hari.', 'invoice'),
  ('invoice_pre3', 'Invoice Pre 3 Days', 'Halo {{.pic_name}}, invoice #{{.invoice_id}} jatuh tempo dalam 3 hari. Mohon diproses.', 'invoice'),
  ('nps_survey', 'NPS Survey', 'Halo {{.pic_name}}, seberapa besar kemungkinan Anda merekomendasikan Kantorku.id? (1-10)', 'nps'),
  ('referral_request', 'Referral Request', 'Halo {{.pic_name}}, apakah Anda punya rekan yang membutuhkan sistem HRIS? Kami berikan bonus referral!', 'referral'),
  ('low_usage_alert', 'Low Usage Alert', 'Halo {{.pic_name}}, kami perhatikan penggunaan aplikasi masih rendah. Ada yang bisa kami bantu?', 'health'),
  ('cross_sell_h7', 'Cross-sell HT 7d', 'Halo {{.pic_name}}, kami memiliki fitur baru yang mungkin cocok untuk perusahaan Anda.', 'cross_sell'),
  ('overdue_post1', 'Overdue Post 1 Day', 'Halo {{.pic_name}}, invoice #{{.invoice_id}} sudah jatuh tempo kemarin. Mohon segera diproses.', 'overdue');
