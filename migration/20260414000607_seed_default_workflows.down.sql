-- Remove seeded default workflows (tabs/stats/columns cascade via FK)
DELETE FROM workflows WHERE slug IN (
  '4fc22c98-1e3b-4901-aa86-9f81b33354d2',
  '0c85261e-277c-4143-93b3-bb6714eaff08',
  '406e6b25-37f6-4531-aade-aa42df2d52a3',
  '01400f6a-cdc9-43a0-8409-b96e316bec91'
);
