# Collections — Backend Implementation Guide

## Context

**Collections** adalah fitur user-defined generic tables — mirip Airtable / Notion Database versi minimal. User bisa bikin "tabel" sendiri dari UI tanpa perlu deploy backend. Dipakai untuk data ad-hoc yang **tidak layak bikin fitur penuh** tapi masih butuh tempat terstruktur.

## ⚠️ Important Distinction

| | Data Master (Clients) | Collections |
|---|---|---|
| **Nature** | Core business entity | User-defined ad-hoc data |
| **Schema** | Fixed + JSONB custom fields | 100% dynamic (user define fields) |
| **Source of truth for workflow?** | ✅ YES — memory rule applies | ❌ NO — workflow tidak read/write Collections |
| **Referenced by invoices/pipeline?** | ✅ YES | ❌ NO |
| **Defined by** | Dev (schema migration) | User (runtime, via UI) |
| **Validation** | Zod + backend strict | User-defined per field |

**Aturan ketat:** Collections **tidak boleh di-reference** oleh Workflow Engine, Invoice, atau pipeline logic. Kalau data cukup penting untuk di-reference workflow, data itu harus jadi **dedicated feature** (seperti Vouchers, Products) — bukan Collection.

## Use Cases

**Cocok untuk Collections:**
- Internal FAQ / knowledge base
- Daftar event internal
- Draft idea / backlog non-technical
- Vendor contact list (yang bukan client)
- Checklist kepatuhan tim
- Meeting notes terstruktur

**TIDAK cocok (harus dedicated table):**
- Voucher / promo (di-reference invoice)
- Product catalog (di-reference pipeline)
- Lead/prospect (Data Master sudah cover)
- Payment record (audit-critical)

## Arsitektur: Meta-Schema

3 tabel inti:

```
collections              → meta: nama tabel, owner, workspace
collection_fields        → meta: kolom per collection (type, label, required)
collection_records       → data: satu row per record, nilai di JSONB
```

```
┌──────────────────────────┐
│ collections              │
│ ──────────────────────── │
│ id, workspace_id, name,  │
│ slug, icon, description  │
└───────┬──────────────────┘
        │ 1:N
        ▼
┌──────────────────────────┐
│ collection_fields        │
│ ──────────────────────── │
│ id, collection_id, key,  │
│ label, type, required,   │
│ options (jsonb), order   │
└───────┬──────────────────┘
        │
        │ defines schema of →
        ▼
┌──────────────────────────┐
│ collection_records       │
│ ──────────────────────── │
│ id, collection_id,       │
│ data (jsonb), created_*, │
│ updated_*                │
└──────────────────────────┘
```

## Field Types Supported

| Type | Storage | UI | Validation |
|---|---|---|---|
| `text` | string | `<input>` | maxLength |
| `textarea` | string | `<textarea>` | maxLength |
| `number` | number | `<input type="number">` | min/max |
| `boolean` | bool | toggle | — |
| `date` | ISO string | date picker | — |
| `datetime` | ISO string | datetime picker | — |
| `enum` | string | `<select>` | options required |
| `multi_enum` | string[] | multi-select | options required |
| `url` | string | `<input>` | URL format |
| `email` | string | `<input>` | email format |
| `link_client` | uuid | client picker | FK → clients |
| `file` | url | upload | size/type |

**Catatan:** `link_client` adalah satu-satunya jembatan balik ke Data Master. Tetap satu arah — Collection *reference* Client, Client tidak tahu tentang Collection.

## Scoping

Collection selalu **per-workspace**. Tidak ada collection global. User di workspace Dealls tidak bisa lihat collection di workspace KantorKu.

## RBAC

- **Admin** — full CRUD collections + fields + records
- **Editor** — CRUD records only, tidak bisa ubah schema
- **Viewer** — read records only

Collection-level permission disimpan di `collections.permissions` (jsonb) — bisa override default workspace role.

## Checker-Maker

Schema changes (add/remove field, delete collection) → **butuh approval** (masuk ke `05-checker-maker.md`, operation type: `collection_schema_change`).
Data changes (CRUD records) → **tidak butuh approval** (low-risk, cukup audit log).

## Scale Assumptions

- Max 50 collections per workspace
- Max 30 fields per collection
- Max 10,000 records per collection (hard limit untuk MVP)
- Query via JSONB GIN index untuk filter dasar

Kalau user butuh lebih dari batas di atas → indikasi harus jadi dedicated feature, bukan Collection.
