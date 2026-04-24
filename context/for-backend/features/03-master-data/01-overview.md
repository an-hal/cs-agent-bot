# Master Data — Backend Implementation Guide

## Context
Dashboard ini adalah multi-workspace CRM (Dealls, KantorKu, Sejutacita holding).
Setiap workspace bisa punya jenis bisnis berbeda, sehingga Master Data harus **flexible** — ada kolom fixed (core) dan kolom custom (per workspace).

## Arsitektur: Hybrid Schema (Fixed + Dynamic)

**Core fields** = kolom tetap, dibutuhkan oleh sistem (routing, automation, billing).
**Custom fields** = kolom tambahan per workspace, disimpan di JSONB.

### Kenapa Hybrid?
- Full fixed → tidak bisa handle beda industri (HR vs logistik vs retail)
- Full dynamic → tidak bisa hardcode workflow logic, validasi sulit
- Hybrid → core selalu ada + custom per kebutuhan. Ini yang dipakai Salesforce, HubSpot, Pipedrive.

## Alur Data

```
                    READ (core + custom fields)
                  ┌─────────────────────────┐
                  ▼                         │
           ┌─────────────┐                 │
           │ Master Data  │◄── WRITE (update flags, stage, status)
           │   (PostgreSQL)│                 │
           └─────────────┘                 │
                  │                         │
        ┌────────┼────────┐                │
        ▼        ▼        ▼                │
      SDR       BD       AE    CS         │
   Stage=LEAD  PROSPECT  CLIENT  CLIENT   │
        │        │        │       │        │
        └────────┴────────┴───────┘────────┘
                 WRITE back
```

### Stage Transitions (mengubah Master Data)
- SDR qualified → **`Stage = 'PROSPECT'`** → BD takes over
- BD payment confirmed → **`Stage = 'CLIENT'`** → AE takes over
- AE lifecycle loop (P0-P6) → writes flags, Payment_Status, renewed
- CS → READ only dari CLIENT, writes CSAT_Score + Risk_Flag
- BD dormant D+90 → **`Stage = 'LEAD'`, `lead_segment = 'RECYCLED'`** → SDR re-engages

### Setiap Node Workflow Harus:
1. **READ** fields tertentu dari Master Data (core + custom)
2. Evaluate condition
3. Execute action (send WA, update DB, etc.)
4. **WRITE** result kembali ke Master Data (sent flags, status changes)
