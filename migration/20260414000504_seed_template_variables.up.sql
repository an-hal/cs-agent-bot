-- Migration: Seed common template variables for all existing workspaces
-- Version: 20260414000504

INSERT INTO template_variables (workspace_id, variable_key, display_label, source_type, source_field, description, example_value)
SELECT w.id, v.key, v.label, v.source_type, v.source_field, v.description, v.example
FROM workspaces w
CROSS JOIN (VALUES
    ('Company_Name',              'Nama Perusahaan',    'master_data_core',  'company_name',         'Nama perusahaan dari Master Data', 'PT Maju Digital'),
    ('PIC_Name',                  'Nama PIC',           'master_data_core',  'pic_name',             'Nama PIC utama',                   'Budi Santoso'),
    ('PIC_Nickname',              'Panggilan PIC',      'master_data_core',  'pic_nickname',         'Panggilan akrab PIC',              'Budi'),
    ('contact_name_prefix_manual','Gelar PIC',          'master_data_custom','prefix',               'Pak/Bu',                           'Pak'),
    ('contact_name_primary',      'Nama Kontak Utama',  'master_data_core',  'pic_name',             'Nama PIC utama',                   'Budi'),
    ('SDR_Name',                  'Nama SDR',           'master_data_core',  'owner_name',           'Nama SDR pemilik akun',            'Rina'),
    ('SDR_Owner',                 'Owner SDR',          'master_data_core',  'owner_name',           'Alias Owner SDR',                  'Rina'),
    ('AM_Name',                   'Nama AM',            'master_data_core',  'owner_name',           'Account Manager pemilik akun',     'Arief'),
    ('HC_Size',                   'Ukuran HC',          'master_data_custom','hc_size',              'Headcount perusahaan',             '150'),
    ('Industry',                  'Industri',           'master_data_custom','industry',             'Industri perusahaan',              'Technology'),
    ('Usage_Score',               'Skor Usage',         'master_data_custom','usage_score',          'Skor penggunaan produk',           '85'),
    ('Expiry_Date',               'Tanggal Berakhir',   'computed',           'contract_end',         'Format tanggal kontrak berakhir',  '30 Jun 2026'),
    ('Due_Date',                  'Tanggal Jatuh Tempo','computed',           'due_date',             'Jatuh tempo invoice',              '15 May 2026'),
    ('contract_end',              'Akhir Kontrak',      'master_data_core',  'contract_end',         'Tanggal kontrak berakhir',         '2026-06-30'),
    ('months_active',             'Bulan Aktif',        'computed',           'months_active',        'Bulan aktif sejak kontrak dimulai','8'),
    ('amount',                    'Jumlah',             'invoice',            'amount',               'Nominal invoice (IDR)',            'Rp 12.500.000'),
    ('Invoice_ID',                'ID Invoice',         'invoice',            'invoice_id',           'ID invoice',                       'INV-2026-001'),
    ('link_wiki',                 'Link Wiki',          'workspace_config',   NULL,                   'URL wiki/tutorial produk',         'https://wiki.example.com'),
    ('link_nps_survey',           'Link NPS Survey',    'generated',          NULL,                   'URL NPS survey per company',       'https://nps.example.com/abc'),
    ('link_checkin_form',         'Link Check-in Form', 'generated',          NULL,                   'URL form check-in',                'https://checkin.example.com/abc'),
    ('link_calendar',             'Link Kalender',      'workspace_config',   NULL,                   'URL booking kalender AE/SDR',      'https://cal.com/ae-dealls'),
    ('link_invoice',              'Link Invoice',       'invoice',            'paper_id_url',         'URL invoice PDF',                  'https://pay.example.com/abc'),
    ('link_quotation',            'Link Quotation',     'generated',          NULL,                   'URL quotation per deal',           'https://q.example.com/abc'),
    ('link_pricing',              'Link Pricing',       'workspace_config',   NULL,                   'URL halaman pricing',              'https://example.com/pricing'),
    ('link_deck',                 'Link Deck',          'workspace_config',   NULL,                   'URL pitch deck',                   'https://example.com/deck'),
    ('referral_form_link',        'Link Referral',      'generated',          NULL,                   'URL form referral',                'https://ref.example.com/abc')
) AS v(key, label, source_type, source_field, description, example)
ON CONFLICT (workspace_id, variable_key) DO NOTHING;
