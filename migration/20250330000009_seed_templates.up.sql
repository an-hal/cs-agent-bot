-- Migration: Seed Templates
-- Version: 20250330000009
-- Description: Seeds the templates table with all WhatsApp message templates
-- Template IDs must match exactly what the trigger usecase passes to sendMessage()
-- Variables use [Bracket_Syntax] matching the resolver in internal/usecase/template/resolver.go

-- RENEWAL Templates (trigger IDs: REN60, REN45, REN30, REN15, REN0)
INSERT INTO templates (template_id, template_name, template_content, template_category, language, active) VALUES
('REN60', 'Renewal H-60', 'Halo [PIC_Name], kontrak [Company_Name] akan berakhir dalam 60 hari. Sudah waktunya kita diskusikan perpanjangan kontrak. Ada yang bisa saya bantu?', 'RENEWAL', 'id', TRUE),
('REN45', 'Renewal H-45', 'Halo [PIC_Name], kontrak [Company_Name] akan berakhir dalam 45 hari. Sudah saatnya mempertimbangkan perpanjangan. Silakan cek detail di: [link_quotation]', 'RENEWAL', 'id', TRUE),
('REN30', 'Renewal H-30', 'Halo [PIC_Name], kontrak [Company_Name] akan berakhir dalam 30 hari. Sudah saatnya untuk mengurus perpanjangan. Link quotation bisa diakses di: [link_quotation]', 'RENEWAL', 'id', TRUE),
('REN15', 'Renewal H-15', 'Halo [PIC_Name], hanya 15 hari lagi sebelum kontrak [Company_Name] berakhir. Mohon segera proses perpanjangan untuk menghindari gangguan layanan.', 'RENEWAL', 'id', TRUE),
('REN0', 'Renewal H-0', 'Halo [PIC_Name], kontrak [Company_Name] berakhir HARI INI. Segera hubungi kami untuk perpanjangan kontrak agar layanan tetap aktif.', 'RENEWAL', 'id', TRUE),

-- PAYMENT Templates (trigger IDs: TPL-PAY-PRE14, TPL-PAY-PRE7, TPL-PAY-PRE3, TPL-PAY-POST1, TPL-PAY-POST4, TPL-PAY-POST8, TPL-PAY-POST15)
('TPL-PAY-PRE14', 'Invoice Pre 14 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] akan jatuh tempo dalam 14 hari pada [Due_Date]. Mohon dipersiapkan pembayarannya. Terima kasih.', 'PAYMENT', 'id', TRUE),
('TPL-PAY-PRE7', 'Invoice Pre 7 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] akan jatuh tempo dalam 7 hari pada [Due_Date]. Mohon segera diproses.', 'PAYMENT', 'id', TRUE),
('TPL-PAY-PRE3', 'Invoice Pre 3 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] akan jatuh tempo dalam 3 hari pada [Due_Date]. Mohon segera diproses untuk menghindari keterlambatan.', 'PAYMENT', 'id', TRUE),
('TPL-PAY-POST1', 'Invoice Post 1 Day', 'Halo [PIC_Name], invoice untuk [Company_Name] sudah jatuh tempo kemarin ([Due_Date]). Mohon segera diproses pembayarannya.', 'PAYMENT', 'id', TRUE),
('TPL-PAY-POST4', 'Invoice Post 4 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] sudah overdue 4 hari sejak [Due_Date]. Mohon segera diproses untuk menghindari penangguhan layanan.', 'PAYMENT', 'id', TRUE),
('TPL-PAY-POST8', 'Invoice Post 8 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] sudah overdue 8 hari sejak [Due_Date]. Mohon segera diproses. Jika ada kendala, silakan hubungi kami.', 'PAYMENT', 'id', TRUE),
('TPL-PAY-POST15', 'Invoice Post 15 Days', 'Halo [PIC_Name], invoice untuk [Company_Name] sudah overdue 15 hari sejak [Due_Date]. Ini peringatan terakhir sebelum layanan ditangguhkan. Mohon segera proses pembayaran.', 'PAYMENT', 'id', TRUE),

-- CHECKIN Templates (trigger IDs: TPL-CHECKIN-FORM, TPL-CHECKIN-CALL)
('TPL-CHECKIN-FORM', 'Check-in Form', 'Halo [PIC_Name], bagaimana pengalaman Bapak/Ibu dengan sistem HRIS kami? Mohon isi form singkat ini: [link_checkin_form]. Feedback Anda sangat berharga bagi kami.', 'CHECKIN', 'id', TRUE),
('TPL-CHECKIN-CALL', 'Check-in Call', 'Halo [PIC_Name], terima kasih sudah mengisi form check-in. Bisakah kita jadwalkan call singkat untuk mendiskusikan pengalaman Anda? Ada yang bisa kami tingkatkan?', 'CHECKIN', 'id', TRUE),

-- NPS Templates (trigger IDs: NPS1, NPS2, NPS3)
('NPS1', 'NPS Survey Early', 'Halo [PIC_Name], sudah sekitar 2 minggu [Company_Name] menggunakan sistem HRIS kami. Seberapa besar kemungkinan Anda merekomendasikan kami ke rekan? (Skala 1-10)', 'NPS', 'id', TRUE),
('NPS2', 'NPS Survey Mid', 'Halo [PIC_Name], bagaimana pengalaman Anda sejauh ini? Mohon luangkan waktu sebentar untuk mengisi survey NPS di: [link_survey]. Terima kasih!', 'NPS', 'id', TRUE),
('NPS3', 'NPS Survey Late', 'Halo [PIC_Name], kami terus meningkatkan layanan. Mohon berikan penilaian Anda melalui survey singkat di: [link_survey]. Feedback Anda sangat membantu.', 'NPS', 'id', TRUE),

-- REFERRAL Template (trigger ID: REFERRAL)
('REFERRAL', 'Referral Request', 'Halo [PIC_Name], senang mendengar pengalaman baik Anda dengan sistem HRIS kami! Apakah Anda punya rekan atau perusahaan lain yang membutuhkan sistem serupa? Kami berikan [Benefit_Referral] untuk setiap referral yang berhasil.', 'REFERRAL', 'id', TRUE),

-- CROSS-SELL Templates (trigger IDs: CS_H7, CS_H14, CS_H21, CS_H30, CS_H45, CS_H60, CS_H75, CS_H90, CS_LT1, CS_LT2, CS_LT3)
('CS_H7', 'Cross-sell Day 7', 'Halo [PIC_Name], selamat! [Company_Name] sudah 1 minggu menggunakan sistem HRIS kami. Ada fitur-fitur lain yang mungkin cocok untuk kebutuhan perusahaan Anda. Mau saya jelaskan?', 'CROSS_SELL', 'id', TRUE),
('CS_H14', 'Cross-sell Day 14', 'Halo [PIC_Name], sudah 2 minggu berlangganan. Kami memiliki modul tambahan seperti payroll advanced, attendance dengan geofencing, dan custom report. Ada yang tertarik untuk dicoba?', 'CROSS_SELL', 'id', TRUE),
('CS_H21', 'Cross-sell Day 21', 'Halo [PIC_Name], 3 minggu sudah berlalu. Untuk meningkatkan efisiensi, kami memiliki modul performance management yang bisa membantu tim HR. Apakah [Company_Name] tertarik?', 'CROSS_SELL', 'id', TRUE),
('CS_H30', 'Cross-sell Day 30', 'Halo [PIC_Name], sudah 1 bulan! Kami punya solusi untuk integrasi dengan sistem payroll pihak ketiga. Apakah tim HR Anda mengalami kesulitan dengan payroll saat ini?', 'CROSS_SELL', 'id', TRUE),
('CS_H45', 'Cross-sell Day 45', 'Halo [PIC_Name], untuk efisiensi, kami menawarkan modul recruitment system yang terintegrasi dengan HRIS. Bisa membantu tim Anda dalam proses hiring. Mau saya kirim demo?', 'CROSS_SELL', 'id', TRUE),
('CS_H60', 'Cross-sell Day 60', 'Halo [PIC_Name], 2 bulan sudah berlalu. Kami melihat banyak perusahaan sejenis yang meng-upgrade paket mereka untuk mendapatkan fitur custom dashboard dan API access. Apakah ini relevan untuk [Company_Name]?', 'CROSS_SELL', 'id', TRUE),
('CS_H75', 'Cross-sell Day 75', 'Halo [PIC_Name], apakah tim IT Anda butuh integrasi API yang lebih extensive? Kami menyediakan dedicated support untuk enterprise integration. Mari diskusi kebutuhan teknisnya.', 'CROSS_SELL', 'id', TRUE),
('CS_H90', 'Cross-sell Day 90', 'Halo [PIC_Name], 3 bulan bersama [Company_Name]! Kami akan launching fitur baru bulan depan: AI-powered leave recommendation dan smart scheduling. Mau saya jadwalkan demo exclusive?', 'CROSS_SELL', 'id', TRUE),
('CS_LT1', 'Cross-sell Long-term 1', 'Halo [PIC_Name], untuk perusahaan sebesar [Company_Name], kami menawarkan modul Employee Engagement dengan pulse survey dan eNPS tracking. Ini bisa membantu HR memantau morale karyawan.', 'CROSS_SELL', 'id', TRUE),
('CS_LT2', 'Cross-sell Long-term 2', 'Halo [PIC_Name], dari analisis penggunaan, fitur yang paling sering digunakan adalah attendance dan leave. Kami punya modul tambahan untuk shift management dan overtime calculation yang bisa menghemat waktu admin hingga 40%.', 'CROSS_SELL', 'id', TRUE),
('CS_LT3', 'Cross-sell Long-term 3', 'Halo [PIC_Name], untuk jangka panjang, kami menawarkan annual contract dengan discount hingga 20% + free training untuk tim HR. Ini bisa menghemat budget operasional [Company_Name] secara signifikan.', 'CROSS_SELL', 'id', TRUE),

-- HEALTH Templates (trigger IDs: LOW_USAGE, LOW_NPS)
('LOW_USAGE', 'Health Low Usage', 'Halo [PIC_Name], kami perhatikan penggunaan aplikasi masih rendah. Ada kendala atau sesuatu yang bisa kami bantu? Tim support kami siap membantu.', 'HEALTH', 'id', TRUE),
('LOW_NPS', 'Health Low NPS Alert', 'Halo [PIC_Name], berdasarkan data penggunaan dan engagement, kami mendeteksi potensi risiko churn. Apakah ada masalah yang belum terselesaikan? [Owner_Name] akan segera menghubungi Anda.', 'HEALTH', 'id', TRUE);

-- Create indexes for template lookups
CREATE INDEX IF NOT EXISTS idx_templates_category_active ON templates(template_category, active);
CREATE INDEX IF NOT EXISTS idx_templates_id_active ON templates(template_id, active);
