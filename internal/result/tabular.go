package result

import "qLLM/internal/protocol"

// New constructs a value.
func New(columns []protocol.Column, rows [][]any, truncated bool) *protocol.TabularResult {
	return &protocol.TabularResult{
		Columns:   columns,
		Rows:      rows,
		RowCount:  len(rows),
		Truncated: truncated,
	}
}

// Empty implements runtime behavior for this package.
func Empty(columns []protocol.Column) *protocol.TabularResult {
	return New(columns, [][]any{}, false)
}
