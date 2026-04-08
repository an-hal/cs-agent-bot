-- Rollback: Remove workspace-scoped seed data

DELETE FROM conversation_states WHERE company_id IN ('DL01','DL02','DL03','DL04','DL05','DL06','DL07','DL08','KK01','KK02','KK03','KK04','KK05','KK06');
DELETE FROM client_flags WHERE company_id IN ('DL01','DL02','DL03','DL04','DL05','DL06','DL07','DL08','KK01','KK02','KK03','KK04','KK05','KK06');
DELETE FROM escalations WHERE company_id IN ('DL01','DL02','DL03','DL04','DL05','DL06','DL07','DL08','KK01','KK02','KK03','KK04','KK05','KK06');
DELETE FROM invoices WHERE invoice_id LIKE 'INV-DL%' OR invoice_id LIKE 'INV-KK%';
DELETE FROM clients WHERE company_id IN ('DL01','DL02','DL03','DL04','DL05','DL06','DL07','DL08','KK01','KK02','KK03','KK04','KK05','KK06');
