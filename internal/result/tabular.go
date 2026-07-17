package result

import "qLLM/internal/protocol"

func New(columns []protocol.Column, rows [][]any, truncated bool) *protocol.TabularResult {
	return &protocol.TabularResult{
		Columns:   columns,
		Rows:      rows,
		RowCount:  len(rows),
		Truncated: truncated,
	}
}

func Empty(columns []protocol.Column) *protocol.TabularResult {
	return New(columns, [][]any{}, false)
}
