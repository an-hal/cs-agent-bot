-- Migration: Seed Templates
-- Version: 20250330000009
-- Description: Seeds the templates table with all WhatsApp message templates

-- RENEWAL Templates
INSERT INTO templates (template_id, template_name, template_content, template_category, language, active) VALUES
('TPL-REN60', 'Renewal H-60', 'Halo [PIC_Name], kontrak [Company_Name] akan berakhir dalam 60 hari pada [ContractEnd]. Sudah waktunya kita diskusikan perpanjangan kontrak. Ada yang bisa saya bantu?', 'RENEWAL', 'id', TRUE),

('TPL-REN45', 'Renewal H-45', 'Halo [PIC_Name], kontrak [Company_Name] akan berakhir dalam 45 hari pada [ContractEnd]. Sebagai informasi, promo diskon [PROMO_DEADLINE] masih berlaku untuk perpanjangan sebelum tanggal tersebut.', 'RENEWAL', 'id', TRUE),

('TPL-REN30', 'Renewal H-30', 'Halo [PIC_Name], kontrak [Company_Name] akan berakhir dalam 30 hari pada [ContractEnd]. Sudah saatnya untuk mengurus perpanjangan. Link quotation bisa diakses di: [QUOTATION_URL]', 'RENEWAL', 'id', TRUE),

('TPL-REN15', 'Renewal H-15', 'Halo [PIC_Name], hanya 15 hari lagi sebelum kontrak [Company_Name] berakhir pada [ContractEnd]. Mohon segera proses perpanjangan untuk menghindari gangguan layanan.', 'RENEWAL', 'id', TRUE),

('TPL-REN0', 'Renewal H-0', 'Halo [PIC_Name], kontrak [Company_Name] berakhir HARI INI pada [ContractEnd]. Segera hubungi kami untuk perpanjangan kontrak agar layanan tetap aktif.', 'RENEWAL', 'id', TRUE),

-- PAYMENT Templates
('TPL-PAY-PRE14', 'Invoice Pre 14 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] sebesar Rp[FinalPrice] akan jatuh tempo dalam 14 hari pada [DueDate]. Mohon dipersiapkan pembayarannya. Terima kasih.', 'PAYMENT', 'id', TRUE),

('TPL-PAY-PRE7', 'Invoice Pre 7 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] sebesar Rp[FinalPrice] akan jatuh tempo dalam 7 hari pada [DueDate]. Mohon segera diproses.', 'PAYMENT', 'id', TRUE),

('TPL-PAY-PRE3', 'Invoice Pre 3 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] sebesar Rp[FinalPrice] akan jatuh tempo dalam 3 hari pada [DueDate]. Mohon segera diproses untuk menghindari keterlambatan.', 'PAYMENT', 'id', TRUE),

('TPL-PAY-POST1', 'Invoice Post 1 Day', 'Halo [PIC_Name], invoice untuk [Company_Name] sebesar Rp[FinalPrice] sudah jatuh tempo kemarin ([DueDate]). Mohon segera diproses pembayarannya.', 'PAYMENT', 'id', TRUE),

('TPL-PAY-POST4', 'Invoice Post 4 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] sebesar Rp[FinalPrice] sudah overdue 4 hari sejak [DueDate]. Mohon segera diproses untuk menghindari penangguhan layanan.', 'PAYMENT', 'id', TRUE),

('TPL-PAY-POST8', 'Invoice Post 8 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] sebesar Rp[FinalPrice] sudah overdue 8 hari sejak [DueDate]. Mohon segera diproses. Jika ada kendala, silakan hubungi kami.', 'PAYMENT', 'id', TRUE),

-- CHECKIN Templates
('TPL-CHECKIN-FORM', 'Check-in Form', 'Halo [PIC_Name], bagaimana pengalaman Bapak/Ibu dengan sistem HRIS kami? Mohon isi form singkat ini: [CHECKIN_FORM_URL]. Feedback Anda sangat berharga bagi kami.', 'CHECKIN', 'id', TRUE),

('TPL-CHECKIN-CALL', 'Check-in Call', 'Halo [PIC_Name], terima kasih sudah mengisi form check-in. Bisakah kita jadwalkan call singkat untuk mendiskusikan pengalaman Anda? Ada yang bisa kami tingkatkan?', 'CHECKIN', 'id', TRUE),

-- NPS Templates
('TPL-EXP-NPS1', 'NPS Survey D+10-15', 'Halo [PIC_Name], sudah sekitar 2 minggu [Company_Name] menggunakan sistem HRIS kami. Seberapa besar kemungkinan Anda merekomendasikan kami ke rekan? (Skala 1-10)', 'NPS', 'id', TRUE),

('TPL-EXP-NPS2', 'NPS Survey D+40-45', 'Halo [PIC_Name], bagaimana pengalaman Anda sejauh ini? Mohon luangkan waktu sebentar untuk mengisi survey NPS di: [SURVEY_PLATFORM_URL]. Terima kasih!', 'NPS', 'id', TRUE),

('TPL-EXP-NPS3', 'NPS Survey D+55-60', 'Halo [PIC_Name], kami terus meningkatkan layanan. Mohon berikan penilaian Anda melalui survey singkat di: [SURVEY_PLATFORM_URL]. Feedback Anda sangat membantu.', 'NPS', 'id', TRUE),

('TPL-EXP-HIGH-REFERRAL', 'Referral Request D+104', 'Halo [PIC_Name], senang mendengar pengalaman baik Anda dengan sistem HRIS kami! Apakah Anda punya rekan atau perusahaan lain yang membutuhkan sistem serupa? Kami berikan [REFERRAL_BENEFIT] untuk setiap referral yang berhasil.', 'NPS', 'id', TRUE),

-- CROSS-SELL Templates
('TPL-CS-H7', 'Cross-sell Day 7', 'Halo [PIC_Name], selamat! [Company_Name] sudah 1 minggu menggunakan sistem HRIS kami. Ada fitur-fitur lain yang mungkin cocok untuk kebutuhan perusahaan Anda. Mau saya jelaskan?', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-H14', 'Cross-sell Day 14', 'Halo [PIC_Name], sudah 2 minggu berlangganan. Kami memiliki modul tambahan seperti payroll advanced, attendance dengan geofencing, dan custom report. Ada yang tertarik untuk dicoba?', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-H21', 'Cross-sell Day 21', 'Halo [PIC_Name], 3 minggu sudah berlalu. Untuk perusahaan dengan HC [HCSize], biasanya membutuhkan modul performance management. Apakah [Company_Name] tertarik?', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-H30', 'Cross-sell Day 30', 'Halo [PIC_Name], sudah 1 bulan! Kami punya solusi untuk integrasi dengan sistem payroll pihak ketiga. Apakah tim HR Anda mengalami kesulitan dengan payroll saat ini?', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-H45', 'Cross-sell Day 45', 'Halo [PIC_Name], untuk efisiensi, kami menawarkan modul recruitment system yang terintegrasi dengan HRIS. Bisa membantu tim Anda dalam proses hiring. Mau saya kirim demo?', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-H60', 'Cross-sell Day 60', 'Halo [PIC_Name], 2 bulan sudah berlalu. Kami melihat banyak perusahaan sejenis [Company_Name] yang meng-upgrade paket mereka untuk mendapatkan fitur custom dashboard dan API access. Apakah ini relevan untuk Anda?', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-H75', 'Cross-sell Day 75', 'Halo [PIC_Name], apakah tim IT Anda butuh integrasi API yang lebih extensive? Kami menyediakan dedicated support untuk enterprise integration. Mari diskusi kebutuhan teknisnya.', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-H90', 'Cross-sell Day 90', 'Halo [PIC_Name], 3 bulan bersama [Company_Name]! Kami akan launching fitur baru bulan depan: AI-powered leave recommendation dan smart scheduling. Mau saya jadwalkan demo exclusive?', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-LT1', 'Cross-sell Long-term 1', 'Halo [PIC_Name], untuk perusahaan sebesar [Company_Name], kami menawarkan modul Employee Engagement dengan pulse survey dan eNPS tracking. Ini bisa membantu HR memantau morale karyawan.', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-LT2', 'Cross-sell Long-term 2', 'Halo [PIC_Name], dari analisis penggunaan, fitur yang paling sering digunakan adalah attendance dan leave. Kami punya modul tambahan untuk shift management dan overtime calculation yang bisa menghemat waktu admin hingga 40%.', 'CROSS_SELL', 'id', TRUE),

('TPL-CS-LT3', 'Cross-sell Long-term 3', 'Halo [PIC_Name], untuk jangka panjang, kami menawarkan annual contract dengan discount hingga 20% + free training untuk tim HR. Ini bisa menghemat budget operasional [Company_Name] secara signifikan.', 'CROSS_SELL', 'id', TRUE),

-- REPLY Templates (for automated replies to WhatsApp messages)
('TPL-REPLY-POSITIVE', 'Reply Positive', 'Terima kasih sudah merespon! Tim kami akan segera menghubungi Anda untuk follow up lebih lanjut.', 'REPLY', 'id', TRUE),

('TPL-REPLY-HELP', 'Reply Help', 'Tentu, saya bantu. Apakah ada informasi spesifik yang Anda butuhkan?', 'REPLY', 'id', TRUE),

('TPL-REPLY-PAYMENT-ACK', 'Reply Payment ACK', 'Terima kasih infonya. Mohon segera diproses pembayaran invoice Anda. Detail pembayaran bisa dicek di dashboard.', 'REPLY', 'id', TRUE),

('TPL-REPLY-PAYMENT-CHECK', 'Reply Payment Check', 'Baik, kami akan cek status pembayaran Anda. Mohon tunggu sebentar, tim finance akan mengkonfirmasi.', 'REPLY', 'id', TRUE),

('TPL-REPLY-EXTENSION', 'Reply Extension', 'Baik, kami mengerti. Mohon informasikan kapan estimasi pembayaran bisa dilakukan agar kami bisa update di sistem.', 'REPLY', 'id', TRUE),

-- HEALTH Templates
('TPL-HEALTH-LOW-USAGE', 'Health Low Usage', 'Halo [PIC_Name], kami perhatikan penggunaan aplikasi masih rendah. Ada kendala atau sesuatu yang bisa kami bantu? Tim support kami siap membantu.', 'HEALTH', 'id', TRUE),

('TPL-HEALTH-RISK-ALERT', 'Health Risk Alert', 'Halo [PIC_Name], berdasarkan data penggunaan dan engagement, kami mendeteksi potensi risiko churn. Apakah ada masalah yang belum terselesaikan? [OwnerName] akan segera menghubungi Anda.', 'HEALTH', 'id', TRUE);

-- Create indexes for template lookups
CREATE INDEX IF NOT EXISTS idx_templates_category_active ON templates(template_category, active);
CREATE INDEX IF NOT EXISTS idx_templates_id_active ON templates(template_id, active);
