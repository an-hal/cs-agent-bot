# Filter DSL Specification

## Overview

Filter DSL dipakai di pipeline tabs, stat cards, dan query endpoints.
Frontend implementation: `lib/pipeline-utils.ts` → `applyFilter()` dan `computeMetric()`.

## Filter Syntax

Satu string filter, diparse oleh backend:

| Filter | SQL Equivalent | Example |
|--------|---------------|---------|
| `all` | No filter (return all) | `all` |
| `bot_active` | `bot_active = TRUE` | `bot_active` |
| `risk` | `risk_flag IN ('High','Mid') OR bot_active = FALSE OR payment_status IN ('Overdue','Terlambat')` | `risk` |
| `stage:{value}` | `stage = '{value}'` | `stage:LEAD` |
| `stage:{v1},{v2}` | `stage IN ('{v1}','{v2}')` | `stage:LEAD,DORMANT` |
| `value_tier:{value}` | `custom_fields->>'value_tier' = '{value}'` | `value_tier:High` |
| `value_tier:{v1},{v2}` | `custom_fields->>'value_tier' IN (...)` | `value_tier:High,Mid` |
| `sequence:{value}` | `sequence_status = '{value}'` | `sequence:active` |
| `payment:{value}` | `payment_status = '{value}'` | `payment:Pending` |
| `expiry:{days}` | `days_to_expiry BETWEEN 0 AND {days}` | `expiry:30` |

## Metric Syntax

Used by stat cards to compute values:

| Metric | SQL Equivalent | Example |
|--------|---------------|---------|
| `count` | `COUNT(*)` | `count` |
| `count:{filter}` | `COUNT(*) WHERE {filter}` | `count:bot_active` |
| `sum:{field}` | `SUM({field})` | `sum:Final_Price` |
| `avg:{field}` | `AVG({field})` | `avg:Days_to_Expiry` |

## Go Implementation

```go
func ParseFilter(filter string, wsID uuid.UUID) (string, []interface{}) {
    base := "workspace_id = $1"
    args := []interface{}{wsID}

    switch {
    case filter == "" || filter == "all":
        return base, args
    case filter == "bot_active":
        return base + " AND bot_active = TRUE", args
    case filter == "risk":
        return base + " AND (risk_flag IN ('High','Mid') OR bot_active = FALSE OR payment_status IN ('Overdue'))", args
    case strings.HasPrefix(filter, "stage:"):
        vals := strings.Split(strings.TrimPrefix(filter, "stage:"), ",")
        placeholders := make([]string, len(vals))
        for i, v := range vals {
            args = append(args, strings.TrimSpace(v))
            placeholders[i] = fmt.Sprintf("$%d", len(args))
        }
        return base + " AND stage IN (" + strings.Join(placeholders, ",") + ")", args
    case strings.HasPrefix(filter, "value_tier:"):
        val := strings.TrimPrefix(filter, "value_tier:")
        args = append(args, val)
        return base + " AND custom_fields->>'value_tier' = $" + fmt.Sprint(len(args)), args
    case strings.HasPrefix(filter, "payment:"):
        val := strings.TrimPrefix(filter, "payment:")
        args = append(args, val)
        return base + " AND payment_status = $" + fmt.Sprint(len(args)), args
    case strings.HasPrefix(filter, "expiry:"):
        days, _ := strconv.Atoi(strings.TrimPrefix(filter, "expiry:"))
        return base + fmt.Sprintf(" AND days_to_expiry BETWEEN 0 AND %d", days), args
    case strings.HasPrefix(filter, "sequence:"):
        val := strings.TrimPrefix(filter, "sequence:")
        args = append(args, val)
        return base + " AND sequence_status = $" + fmt.Sprint(len(args)), args
    default:
        return base, args
    }
}

func ComputeMetric(metric string, wsID uuid.UUID) (string, []interface{}) {
    args := []interface{}{wsID}

    switch {
    case metric == "count":
        return "SELECT COUNT(*) FROM master_data WHERE workspace_id = $1", args
    case strings.HasPrefix(metric, "count:"):
        filter := strings.TrimPrefix(metric, "count:")
        where, fArgs := ParseFilter(filter, wsID)
        return "SELECT COUNT(*) FROM master_data WHERE " + where, fArgs
    case strings.HasPrefix(metric, "sum:"):
        field := strings.TrimPrefix(metric, "sum:")
        return fmt.Sprintf("SELECT COALESCE(SUM(%s), 0) FROM master_data WHERE workspace_id = $1", field), args
    case strings.HasPrefix(metric, "avg:"):
        field := strings.TrimPrefix(metric, "avg:")
        return fmt.Sprintf("SELECT COALESCE(AVG(%s), 0) FROM master_data WHERE workspace_id = $1", field), args
    default:
        return "SELECT 0", nil
    }
}
```
