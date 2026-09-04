package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type TableInfo struct {
	Name     string `json:"name"`
	RowCount int64  `json:"row_count"`
}

type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	PrimaryKey bool   `json:"primary_key"`
	NotNull    bool   `json:"not_null"`
}

type TableData struct {
	Columns []ColumnInfo     `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

type QueryResult struct {
	Columns      []string         `json:"columns,omitempty"`
	Rows         []map[string]any `json:"rows,omitempty"`
	RowsAffected int64            `json:"rows_affected"`
	DurationMs   int64            `json:"duration_ms"`
	Truncated    bool             `json:"truncated,omitempty"`
}

// GetTables returns user-defined SQLite tables with row counts.
func (db *DB) GetTables(ctx context.Context) ([]TableInfo, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != 'goose_db_version'
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tables := make([]TableInfo, 0, len(names))
	for _, name := range names {
		var count int64
		countRow := db.SQL.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", name))
		if err := countRow.Scan(&count); err != nil {
			count = 0
		}

		tables = append(tables, TableInfo{
			Name:     name,
			RowCount: count,
		})
	}
	return tables, nil
}

func (db *DB) getTableColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	rows, err := db.SQL.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("pragma table_info: %w", err)
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:       name,
			Type:       ctype,
			NotNull:    notnull == 1,
			PrimaryKey: pk >= 1,
		})
	}
	return cols, rows.Err()
}

// GetTableRows returns paginated table rows and metadata.
func (db *DB) GetTableRows(ctx context.Context, table string, limit, offset int, sortBy, sortOrder string) (*TableData, error) {
	// Validate table exists in sqlite_master
	var exists bool
	err := db.SQL.QueryRowContext(ctx, `
		SELECT 1 FROM sqlite_master WHERE type='table' AND name = ?
	`, table).Scan(&exists)
	if err != nil || !exists {
		return nil, fmt.Errorf("table %q not found", table)
	}

	cols, err := db.getTableColumns(ctx, table)
	if err != nil {
		return nil, err
	}

	var total int64
	if err := db.SQL.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", table)).Scan(&total); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Validate sort column
	validSortCol := ""
	if sortBy != "" {
		for _, c := range cols {
			if strings.EqualFold(c.Name, sortBy) {
				validSortCol = c.Name
				break
			}
		}
	}
	orderDir := "ASC"
	if strings.EqualFold(sortOrder, "DESC") {
		orderDir = "DESC"
	}

	var query string
	if validSortCol != "" {
		query = fmt.Sprintf("SELECT * FROM %q ORDER BY %q %s LIMIT ? OFFSET ?", table, validSortCol, orderDir)
	} else {
		query = fmt.Sprintf("SELECT * FROM %q LIMIT ? OFFSET ?", table)
	}

	rows, err := db.SQL.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying rows: %w", err)
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var resultRows []map[string]any
	for rows.Next() {
		colValues := make([]any, len(colNames))
		colPointers := make([]any, len(colNames))
		for i := range colValues {
			colPointers[i] = &colValues[i]
		}
		if err := rows.Scan(colPointers...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]any, len(colNames))
		for i, colName := range colNames {
			val := colValues[i]
			if b, ok := val.([]byte); ok {
				rowMap[colName] = string(b)
			} else {
				rowMap[colName] = val
			}
		}
		resultRows = append(resultRows, rowMap)
	}

	return &TableData{
		Columns: cols,
		Rows:    resultRows,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, rows.Err()
}

// UpdateTableRow updates a row matching pkCol = pkVal with updates.
func (db *DB) UpdateTableRow(ctx context.Context, table, pkCol string, pkVal any, updates map[string]any) error {
	var exists bool
	err := db.SQL.QueryRowContext(ctx, `
		SELECT 1 FROM sqlite_master WHERE type='table' AND name = ?
	`, table).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("table %q not found", table)
	}

	cols, err := db.getTableColumns(ctx, table)
	if err != nil {
		return err
	}

	colMap := make(map[string]bool, len(cols))
	var pkCount int
	var isPk bool
	for _, c := range cols {
		colMap[c.Name] = true
		if c.PrimaryKey {
			pkCount++
			if c.Name == pkCol {
				isPk = true
			}
		}
	}

	if !isPk {
		return fmt.Errorf("column %q is not a primary key in table %q", pkCol, table)
	}
	if pkCount > 1 {
		return fmt.Errorf("table %q has a composite primary key and cannot be updated via single column", table)
	}

	var setClauses []string
	var args []any
	for col, val := range updates {
		if !colMap[col] {
			return fmt.Errorf("unknown column %q in table %q", col, table)
		}
		if col == pkCol {
			continue // do not update primary key
		}
		setClauses = append(setClauses, fmt.Sprintf("%q = ?", col))
		args = append(args, val)
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no columns to update")
	}

	args = append(args, pkVal)
	query := fmt.Sprintf("UPDATE %q SET %s WHERE %q = ?", table, strings.Join(setClauses, ", "), pkCol)

	res, err := db.SQL.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating row: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("row with %s = %v not found", pkCol, pkVal)
	}
	return nil
}

func stripLeadingSQLComments(sql string) string {
	s := strings.TrimSpace(sql)
	for {
		if strings.HasPrefix(s, "--") {
			idx := strings.Index(s, "\n")
			if idx == -1 {
				return ""
			}
			s = strings.TrimSpace(s[idx+1:])
			continue
		}
		if strings.HasPrefix(s, "/*") {
			idx := strings.Index(s, "*/")
			if idx == -1 {
				return ""
			}
			s = strings.TrimSpace(s[idx+2:])
			continue
		}
		break
	}
	return s
}

// ExecuteAdminQuery executes a raw SQL query.
func (db *DB) ExecuteAdminQuery(ctx context.Context, query string) (*QueryResult, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	start := time.Now()
	cleanQuery := stripLeadingSQLComments(trimmed)
	upper := strings.ToUpper(cleanQuery)
	isSelect := strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "WITH") || strings.HasPrefix(upper, "PRAGMA") || strings.HasPrefix(upper, "EXPLAIN") || strings.HasPrefix(upper, "VALUES") || strings.Contains(upper, "RETURNING")

	if isSelect {
		rows, err := db.SQL.QueryContext(ctx, trimmed)
		duration := time.Since(start).Milliseconds()
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		colNames, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		const maxQueryRows = 1000
		var resultRows []map[string]any
		var truncated bool
		for rows.Next() {
			if len(resultRows) >= maxQueryRows {
				truncated = true
				break
			}

			colValues := make([]any, len(colNames))
			colPointers := make([]any, len(colNames))
			for i := range colValues {
				colPointers[i] = &colValues[i]
			}
			if err := rows.Scan(colPointers...); err != nil {
				return nil, err
			}

			rowMap := make(map[string]any, len(colNames))
			for i, colName := range colNames {
				val := colValues[i]
				if b, ok := val.([]byte); ok {
					rowMap[colName] = string(b)
				} else {
					rowMap[colName] = val
				}
			}
			resultRows = append(resultRows, rowMap)
		}

		return &QueryResult{
			Columns:      colNames,
			Rows:         resultRows,
			RowsAffected: int64(len(resultRows)),
			DurationMs:   duration,
			Truncated:    truncated,
		}, rows.Err()
	}

	res, err := db.SQL.ExecContext(ctx, trimmed)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}

	affected, _ := res.RowsAffected()
	return &QueryResult{
		RowsAffected: affected,
		DurationMs:   duration,
	}, nil
}
