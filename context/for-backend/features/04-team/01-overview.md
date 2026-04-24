# Team Management & RBAC — Backend Implementation Guide

## Context
Sistem multi-workspace (Dealls, KantorKu, Sejutacita holding) membutuhkan
role-based access control yang granular. Setiap role bisa punya permission
berbeda **per workspace** dan **per module**.

## Model RBAC

### Hierarki
```
Role
  └─ Per Workspace
       └─ Per Module
            └─ Per Action (view_list, view_detail, create, edit, delete, export, import)
```

### Contoh
```
AE Officer
  ├─ Workspace: Dealls
  │    ├─ Dashboard:  view_list=true, view_detail=true, create=false, ...
  │    ├─ AE:         view_list=true, view_detail=true, create=true, edit=true, delete=false, export=true, import=false
  │    ├─ Data Master: view_list=true, view_detail=true, create=true, edit=true, delete=false, export=true, import=false
  │    └─ Team:       SEMUA=false (no access)
  └─ Workspace: KantorKu
       └─ (sama dengan Dealls — atau bisa berbeda)
```

### View Scope (khusus `view_list`)
Action `view_list` punya 3 kemungkinan value:
- `false` — tidak bisa akses modul sama sekali
- `true` — bisa lihat data dalam workspace sendiri
- `'all'` — bisa lihat data lintas workspace (untuk Super Admin / Finance holding)

## Modules
Setiap module di dashboard dipetakan ke permission set:

| ID            | Label              | Group       |
|---------------|--------------------|-------------|
| `dashboard`   | Dashboard          | Overview    |
| `analytics`   | Analytics          | Overview    |
| `reports`     | Reports            | Overview    |
| `ae`          | Account Executive  | Teams       |
| `sdr`         | SDR                | Teams       |
| `bd`          | Business Dev       | Teams       |
| `cs`          | Customer Service   | Teams       |
| `data_master` | Data Master        | Management  |
| `team`        | Team               | Management  |

## Actions per Module

| Action        | Deskripsi                                        |
|---------------|--------------------------------------------------|
| `view_list`   | Lihat daftar/halaman utama modul                 |
| `view_detail` | Lihat detail individual record                   |
| `create`      | Buat record baru                                 |
| `edit`        | Edit record yang sudah ada                       |
| `delete`      | Hapus record (danger action)                     |
| `export`      | Export data ke Excel/CSV                         |
| `import`      | Import data dari Excel/CSV                       |

## Default Roles

### Super Admin
- Akses penuh ke semua modul dan semua workspace
- `view_list = 'all'` di semua modul
- Satu-satunya yang bisa manage role Super Admin lain

### Admin (per workspace)
- Akses penuh dalam 1 workspace
- Bisa manage team kecuali delete member
- Tidak bisa akses workspace lain

### Manager
- Multi-workspace assignment
- Bisa view, create, edit — tidak bisa delete
- Tidak bisa akses modul Team (no team management)

### Officer Roles (AE, SDR, CS)
- Akses penuh ke modul spesifik + read-only ke overview modules
- Tidak bisa delete, import
- Scope: workspace yang ditugaskan

### Finance
- Read-only + export untuk Reports, Analytics, AE, Data Master
- `view_list = 'all'` di holding scope
- Tidak bisa create/edit/delete apapun

### Viewer
- Read-only di semua modul kecuali Team
- Cocok untuk stakeholder eksternal atau magang

## Member States

| Status     | Deskripsi                                      |
|------------|------------------------------------------------|
| `active`   | Bisa login dan menggunakan dashboard            |
| `pending`  | Sudah diundang, belum accept/setup password     |
| `inactive` | Dinonaktifkan oleh admin (tidak bisa login)     |

## Multi-workspace Assignment
- 1 member bisa ditugaskan di beberapa workspace
- Role menentukan permissions **per workspace**
- Member melihat workspace selector di sidebar berdasarkan assignment-nya

## Alur Undangan
```
1. Admin klik "Undang Member"
2. Isi: email, role, workspace(s)
3. Backend:
   a. Cek email belum ada di workspace ini
   b. Create team_member record (status = 'pending')
   c. Kirim email undangan dengan invite_token
   d. Log ke team_activity_logs (action = 'invite_member')
4. Member klik link undangan
5. Setup password / connect account
6. Status berubah ke 'active'
```

## Permission Check Pattern
Setiap API endpoint harus check permission sebelum execute:

```
GET /api/v1/master-data/clients
  → Check: user has 'view_list' on module 'data_master' for current workspace
  → If view_list = 'all': return data from all workspaces
  → If view_list = true: return data from current workspace only
  → If view_list = false: return 403

PUT /api/v1/master-data/clients/{id}
  → Check: user has 'edit' on module 'data_master' for current workspace
  → If false: return 403

DELETE /api/v1/master-data/clients/{id}
  → Check: user has 'delete' on module 'data_master' for current workspace
  → If false: return 403
```
