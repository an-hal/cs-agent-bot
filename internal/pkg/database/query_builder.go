package database

import sq "github.com/Masterminds/squirrel"

// PSQL is a pre-configured StatementBuilder for PostgreSQL.
// It uses dollar sign placeholders ($1, $2, ...) which are required for PostgreSQL.
//
// Usage:
//
//	query, args, err := database.PSQL.
//	    Select("id", "name").
//	    From("users").
//	    Where(sq.Eq{"active": true}).
//	    ToSql()
//	if err != nil {
//	    return err
//	}
//	rows, err := db.QueryContext(ctx, query, args...)
//
// This helper ensures consistent query building across all repositories
// and eliminates the need to repeatedly specify PlaceholderFormat(sq.Dollar).
var PSQL = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
