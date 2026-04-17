-- Reverse seed from 20260414000400
DELETE FROM invoices WHERE invoice_id LIKE 'INV-SEED-%';
DELETE FROM clients  WHERE company_id LIKE 'SEED-%';
