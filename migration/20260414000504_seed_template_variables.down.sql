-- Best-effort cleanup of seeded variables (keeps any user-created rows intact by key list)
DELETE FROM template_variables WHERE variable_key IN (
    'Company_Name','PIC_Name','PIC_Nickname','contact_name_prefix_manual','contact_name_primary',
    'SDR_Name','SDR_Owner','AM_Name','HC_Size','Industry','Usage_Score','Expiry_Date','Due_Date',
    'contract_end','months_active','amount','Invoice_ID','link_wiki','link_nps_survey',
    'link_checkin_form','link_calendar','link_invoice','link_quotation','link_pricing','link_deck',
    'referral_form_link'
);
