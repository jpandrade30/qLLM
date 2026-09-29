package sqlparse

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"qLLM/internal/protocol"
)

type TableUse struct {
	Name  string
	Alias string
}

type ColUse struct {
	Qual string
	Name string
	Star bool
}

type Result struct {
	Tables  []TableUse
	Columns []ColUse
	Star    bool
	Limit   *int
}

var bannedFns = map[string]struct{}{
	"read_csv": {}, "read_csv_auto": {}, "read_parquet": {}, "read_json": {},
	"read_json_auto": {}, "read_ndjson": {}, "postgres_scan": {}, "sqlite_scan": {},
	"glob": {}, "httpfs": {}, "read_text": {}, "read_blob": {},
}

var bannedStmt = map[string]struct{}{
	"INSERT": {}, "UPDATE": {}, "DELETE": {}, "DROP": {}, "ALTER": {}, "CREATE": {},
	"TRUNCATE": {}, "COPY": {}, "ATTACH": {}, "DETACH": {}, "SET": {}, "PRAGMA": {},
	"CALL": {}, "INSTALL": {}, "LOAD": {}, "EXPORT": {}, "IMPORT": {}, "VACUUM": {},
	"MERGE": {}, "REPLACE": {}, "GRANT": {}, "REVOKE": {},
}

type tok struct {
	kind byte // i ident, n number, s string, p punct
	val  string
}

// Parse uses the latest SQL dialect (see protocol.SQLDialectLatest).
func Parse(sql string) (*Result, *protocol.ProtocolError) {
	return ParseWithVersion(sql, protocol.SQLDialectLatest)
}

// ParseWithVersion validates extractable tables/columns for ACL fetch planning.
func ParseWithVersion(sql, dialect string) (*Result, *protocol.ProtocolError) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return nil, protocol.NewError(protocol.ErrInvalidSQL, "sql is empty", nil)
	}
	if dialect == "" {
		dialect = protocol.SQLDialectLatest
	}
	toks, err := lex(sql)
	if err != nil {
		return nil, err
	}
	if hasMultiStatement(toks) {
		return nil, protocol.NewError(protocol.ErrInvalidSQL, "multiple statements are not allowed", nil)
	}
	p := &parser{toks: toks, dialect: dialect}
	out, perr := p.parseStmt()
	if perr != nil {
		return nil, perr
	}
	return out, nil
}

func InjectLimit(sql string, n int) string {
	s := strings.TrimRight(strings.TrimSpace(sql), ";")
	return s + " LIMIT " + strconv.Itoa(n)
}

type parser struct {
	toks    []tok
	i       int
	dialect string
}

func (p *parser) dialectV1() bool { return p.dialect == protocol.SQLDialect1 }

func (p *parser) peek() tok {
	if p.i >= len(p.toks) {
		return tok{kind: 0}
	}
	return p.toks[p.i]
}

func (p *parser) atKW(s string) bool {
	t := p.peek()
	return t.kind == 'i' && strings.EqualFold(t.val, s)
}

func (p *parser) eatKW(s string) bool {
	if p.atKW(s) {
		p.i++
		return true
	}
	return false
}

func (p *parser) parseStmt() (*Result, *protocol.ProtocolError) {
	out := &Result{}
	if p.eatKW("WITH") {
		if err := p.skipWith(out); err != nil {
			return nil, err
		}
	}
	if !p.atKW("SELECT") {
		t := p.peek()
		up := strings.ToUpper(t.val)
		if _, bad := bannedStmt[up]; bad {
			return nil, protocol.NewError(protocol.ErrInvalidSQL, "only SELECT is allowed", map[string]any{"stmt": up})
		}
		return nil, protocol.NewError(protocol.ErrInvalidSQL, "query must be SELECT (WITH optional)", nil)
	}
	return p.parseSelectChain(out)
}

func (p *parser) parseSelectChain(out *Result) (*Result, *protocol.ProtocolError) {
	if err := p.parseSelectCore(out); err != nil {
		return nil, err
	}
	for {
		setOp := ""
		switch {
		case p.eatKW("UNION"):
			setOp = "UNION"
		case p.eatKW("INTERSECT"):
			setOp = "INTERSECT"
		case p.eatKW("EXCEPT"):
			setOp = "EXCEPT"
		default:
			return out, nil
		}
		if p.dialectV1() {
			return nil, protocol.NewError(protocol.ErrInvalidSQL,
				setOp+" requires SQL dialect version \"2\" (omit version or set version to \"2\")",
				map[string]any{"dialect": p.dialect, "op": setOp})
		}
		p.eatKW("ALL")
		if p.eatKW("WITH") {
			if err := p.skipWith(out); err != nil {
				return nil, err
			}
		}
		if !p.atKW("SELECT") {
			return nil, protocol.NewError(protocol.ErrInvalidSQL, "set operation requires SELECT", map[string]any{"op": setOp})
		}
		branch := &Result{}
		if err := p.parseSelectCore(branch); err != nil {
			return nil, err
		}
		merge(out, branch)
	}
}

func (p *parser) skipWith(out *Result) *protocol.ProtocolError {
	for {
		if p.peek().kind != 'i' {
			return protocol.NewError(protocol.ErrInvalidSQL, "WITH requires a name", nil)
		}
		p.i++
		if p.peek().kind == 'p' && p.peek().val == "(" {
			if err := p.skipBalanced(); err != nil {
				return err
			}
		}
		if !p.eatKW("AS") {
			return protocol.NewError(protocol.ErrInvalidSQL, "WITH requires AS", nil)
		}
		if p.peek().kind != 'p' || p.peek().val != "(" {
			return protocol.NewError(protocol.ErrInvalidSQL, "WITH AS requires subquery", nil)
		}
		inner, err := p.parseSubquery()
		if err != nil {
			return err
		}
		merge(out, inner)
		if p.peek().kind == 'p' && p.peek().val == "," {
			p.i++
			continue
		}
		return nil
	}
}

func (p *parser) parseSubquery() (*Result, *protocol.ProtocolError) {
	if p.peek().kind != 'p' || p.peek().val != "(" {
		return nil, protocol.NewError(protocol.ErrInvalidSQL, "expected subquery", nil)
	}
	p.i++
	sub := &parser{toks: p.toks, i: p.i, dialect: p.dialect}
	inner, err := sub.parseStmt()
	if err != nil {
		return nil, err
	}
	p.i = sub.i
	if p.peek().kind != 'p' || p.peek().val != ")" {
		return nil, protocol.NewError(protocol.ErrInvalidSQL, "unclosed subquery", nil)
	}
	p.i++
	return inner, nil
}

func (p *parser) parseSelect(out *Result) (*Result, *protocol.ProtocolError) {
	return p.parseSelectChain(out)
}

func (p *parser) parseSelectCore(out *Result) *protocol.ProtocolError {
	p.i++ // SELECT
	p.eatKW("DISTINCT")
	p.eatKW("ALL")
	if err := p.parseSelectList(out); err != nil {
		return err
	}
	if p.eatKW("FROM") {
		if err := p.parseFrom(out); err != nil {
			return err
		}
	}
	for p.i < len(p.toks) {
		switch {
		case p.atKW("WHERE"), p.atKW("HAVING"):
			p.i++ // WHERE or HAVING
			if err := p.scanExpr(out, "WHERE"); err != nil {
				return err
			}
		case p.eatKW("GROUP"):
			p.eatKW("BY")
			if err := p.scanExpr(out, "GROUP"); err != nil {
				return err
			}
		case p.eatKW("QUALIFY"):
			if p.dialectV1() {
				return protocol.NewError(protocol.ErrInvalidSQL,
					"QUALIFY requires SQL dialect version \"2\"",
					map[string]any{"dialect": p.dialect})
			}
			if err := p.scanExpr(out, "QUALIFY"); err != nil {
				return err
			}
		case p.eatKW("ORDER"):
			p.eatKW("BY")
			if err := p.scanExpr(out, "ORDER"); err != nil {
				return err
			}
		case p.eatKW("LIMIT"):
			n, err := p.parseInt()
			if err != nil {
				return err
			}
			out.Limit = &n
			p.eatKW("OFFSET")
			if p.peek().kind == 'n' {
				p.i++
			}
		case p.eatKW("OFFSET"):
			if p.peek().kind == 'n' {
				p.i++
			}
		case p.atKW("UNION"), p.atKW("INTERSECT"), p.atKW("EXCEPT"):
			return nil
		case p.peek().kind == 'p' && p.peek().val == ")":
			return nil
		case p.peek().kind == 0:
			return nil
		default:
			prev := p.i
			if err := p.scanExpr(out, "tail"); err != nil {
				return err
			}
			if p.i == prev {
				return protocol.NewError(protocol.ErrInvalidSQL, "unexpected token in SELECT",
					map[string]any{"token": p.peek().val})
			}
		}
	}
	return nil
}

func (p *parser) parseSelectList(out *Result) *protocol.ProtocolError {
	for {
		if p.atKW("FROM") || p.peek().kind == 0 {
			break
		}
		if p.peek().kind == 'p' && p.peek().val == "*" {
			out.Star = true
			p.i++
		} else if err := p.scanSelectItem(out); err != nil {
			return err
		}
		if p.peek().kind == 'p' && p.peek().val == "," {
			p.i++
			continue
		}
		break
	}
	return nil
}

func (p *parser) scanSelectItem(out *Result) *protocol.ProtocolError {
	t := p.peek()
	if t.kind == 'i' {
		name := t.val
		p.i++
		if p.peek().kind == 'p' && p.peek().val == "(" {
			if _, bad := bannedFns[strings.ToLower(name)]; bad {
				return protocol.NewError(protocol.ErrInvalidSQL, "function not allowed: "+name, map[string]any{"fn": name})
			}
			if err := p.skipBalancedScan(out); err != nil {
				return err
			}
			return p.scanExpr(out, "FROM")
		}
		if p.peek().kind == 'p' && p.peek().val == "." {
			p.i++
			n := p.peek()
			if n.kind == 'p' && n.val == "*" {
				out.Star = true
				out.Columns = append(out.Columns, ColUse{Qual: name, Star: true})
				p.i++
			} else if n.kind == 'i' {
				out.Columns = append(out.Columns, ColUse{Qual: name, Name: n.val})
				p.i++
			}
			if p.eatKW("AS") {
				if p.peek().kind == 'i' && !isFromBreaker(p.peek().val) {
					p.i++
				}
			}
			return nil
		}
		if isSQLKeyword(name) || p.atExprContinuator() {
			p.i-- // rescan expression from this identifier
			return p.scanExpr(out, "FROM")
		}
		out.Columns = append(out.Columns, ColUse{Name: name})
		if p.eatKW("AS") {
			if p.peek().kind == 'i' && !isFromBreaker(p.peek().val) {
				p.i++
			}
		}
		return nil
	}
	return p.scanExpr(out, "FROM")
}

func (p *parser) parseFrom(out *Result) *protocol.ProtocolError {
	if err := p.parseTableRef(out); err != nil {
		return err
	}
	for {
		if p.peek().kind == 'p' && p.peek().val == "," {
			p.i++
			if err := p.parseTableRef(out); err != nil {
				return err
			}
			continue
		}
		join := p.eatKW("JOIN") || p.eatKW("INNER") || p.eatKW("LEFT") || p.eatKW("RIGHT") || p.eatKW("FULL") || p.eatKW("CROSS") || p.eatKW("NATURAL")
		if p.eatKW("OUTER") {
			join = true
		}
		if p.eatKW("JOIN") {
			join = true
		}
		if !join {
			return nil
		}
		if err := p.parseTableRef(out); err != nil {
			return err
		}
		if p.eatKW("ON") {
			if err := p.scanExpr(out, "ON"); err != nil {
				return err
			}
		}
		p.eatKW("USING")
	}
}

func (p *parser) parseTableRef(out *Result) *protocol.ProtocolError {
	if p.peek().kind == 'p' && p.peek().val == "(" {
		inner, err := p.parseSubquery()
		if err != nil {
			return err
		}
		merge(out, inner)
		p.optionalAlias()
		return nil
	}
	if p.peek().kind != 'i' {
		return protocol.NewError(protocol.ErrInvalidSQL, "expected table name", nil)
	}
	name := p.peek().val
	p.i++
	if p.peek().kind == 'p' && p.peek().val == "(" {
		if _, bad := bannedFns[strings.ToLower(name)]; bad {
			return protocol.NewError(protocol.ErrInvalidSQL, "table function not allowed: "+name, map[string]any{"fn": name})
		}
		return protocol.NewError(protocol.ErrInvalidSQL, "table functions are not allowed", map[string]any{"fn": name})
	}
	if p.peek().kind == 'p' && p.peek().val == "." {
		return protocol.NewError(protocol.ErrInvalidSQL, "schema-qualified tables are not allowed", map[string]any{"table": name})
	}
	alias := p.optionalAlias()
	out.Tables = append(out.Tables, TableUse{Name: name, Alias: alias})
	return nil
}

func (p *parser) optionalAlias() string {
	p.eatKW("AS")
	if p.peek().kind == 'i' && !isFromBreaker(p.peek().val) {
		a := p.peek().val
		p.i++
		return a
	}
	return ""
}

func (p *parser) atExprContinuator() bool {
	if p.peek().kind != 'p' {
		return false
	}
	switch p.peek().val {
	case "+", "-", "*", "/", "%":
		return true
	default:
		return false
	}
}

func isFromBreaker(s string) bool {
	switch strings.ToUpper(s) {
	case "ON", "WHERE", "GROUP", "ORDER", "LIMIT", "HAVING", "QUALIFY", "JOIN", "INNER", "LEFT", "RIGHT",
		"FULL", "CROSS", "NATURAL", "OUTER", "USING", "UNION", "EXCEPT", "INTERSECT", "OFFSET", "OVER", "PARTITION", "XOR":
		return true
	}
	return false
}

func (p *parser) scanExpr(out *Result, until string) *protocol.ProtocolError {
	_ = until
	depth := 0
	for p.i < len(p.toks) {
		t := p.peek()
		if t.kind == 0 {
			return nil
		}
		if t.kind == 'p' {
			switch t.val {
			case "(":
				depth++
				p.i++
				continue
			case ")":
				if depth == 0 {
					return nil
				}
				depth--
				p.i++
				continue
			case ",":
				if depth == 0 && (until == "FROM" || until == "GROUP" || until == "ORDER" || until == "select") {
					return nil
				}
				p.i++
				continue
			}
		}
		if depth == 0 && t.kind == 'i' && isClauseKW(t.val) {
			return nil
		}
		if t.kind == 'i' {
			name := t.val
			p.i++
			if strings.EqualFold(name, "AS") {
				// output alias is not a catalog column to fetch
				if p.peek().kind == 'i' {
					p.i++
				}
				continue
			}
			if p.peek().kind == 'p' && p.peek().val == "(" {
				if _, bad := bannedFns[strings.ToLower(name)]; bad {
					return protocol.NewError(protocol.ErrInvalidSQL, "function not allowed: "+name, map[string]any{"fn": name})
				}
				if err := p.skipBalancedScan(out); err != nil {
					return err
				}
				continue
			}
			if p.peek().kind == 'p' && p.peek().val == "." {
				p.i++
				n := p.peek()
				if n.kind == 'p' && n.val == "*" {
					out.Star = true
					out.Columns = append(out.Columns, ColUse{Qual: name, Star: true})
					p.i++
				} else if n.kind == 'i' {
					out.Columns = append(out.Columns, ColUse{Qual: name, Name: n.val})
					p.i++
				}
				continue
			}
			if !isSQLKeyword(name) {
				out.Columns = append(out.Columns, ColUse{Name: name})
			}
			continue
		}
		p.i++
	}
	return nil
}

func isClauseKW(s string) bool {
	switch strings.ToUpper(s) {
	case "FROM", "WHERE", "GROUP", "ORDER", "LIMIT", "HAVING", "QUALIFY", "JOIN", "INNER", "LEFT", "RIGHT",
		"FULL", "CROSS", "UNION", "EXCEPT", "INTERSECT", "OFFSET", "ON":
		return true
	}
	return false
}

func (p *parser) parseInt() (int, *protocol.ProtocolError) {
	if p.peek().kind != 'n' {
		return 0, protocol.NewError(protocol.ErrInvalidSQL, "LIMIT requires a number", nil)
	}
	n, err := strconv.Atoi(p.peek().val)
	p.i++
	if err != nil || n < 0 {
		return 0, protocol.NewError(protocol.ErrInvalidSQL, "invalid LIMIT", nil)
	}
	return n, nil
}

func (p *parser) skipBalanced() *protocol.ProtocolError {
	return p.skipBalancedScan(&Result{})
}

func (p *parser) skipBalancedScan(out *Result) *protocol.ProtocolError {
	if p.peek().kind != 'p' || p.peek().val != "(" {
		return protocol.NewError(protocol.ErrInvalidSQL, "expected (", nil)
	}
	p.i++
	depth := 1
	for p.i < len(p.toks) && depth > 0 {
		t := p.peek()
		if t.kind == 'p' && t.val == "(" {
			depth++
			p.i++
			continue
		}
		if t.kind == 'p' && t.val == ")" {
			depth--
			p.i++
			continue
		}
		if t.kind == 'i' && depth >= 1 {
			name := t.val
			p.i++
			if strings.EqualFold(name, "DISTINCT") || strings.EqualFold(name, "ALL") {
				continue
			}
			if p.peek().kind == 'p' && p.peek().val == "(" {
				if _, bad := bannedFns[strings.ToLower(name)]; bad {
					return protocol.NewError(protocol.ErrInvalidSQL, "function not allowed: "+name, map[string]any{"fn": name})
				}
				if err := p.skipBalancedScan(out); err != nil {
					return err
				}
				continue
			}
			if p.peek().kind == 'p' && p.peek().val == "." {
				p.i++
				n := p.peek()
				if n.kind == 'i' {
					out.Columns = append(out.Columns, ColUse{Qual: name, Name: n.val})
					p.i++
				} else if n.kind == 'p' && n.val == "*" {
					out.Star = true
					p.i++
				}
				continue
			}
			if !isSQLKeyword(name) {
				out.Columns = append(out.Columns, ColUse{Name: name})
			}
			continue
		}
		p.i++
	}
	if depth != 0 {
		return protocol.NewError(protocol.ErrInvalidSQL, "unbalanced parentheses", nil)
	}
	return nil
}

func merge(dst, src *Result) {
	dst.Tables = append(dst.Tables, src.Tables...)
	dst.Columns = append(dst.Columns, src.Columns...)
	if src.Star {
		dst.Star = true
	}
}

func hasMultiStatement(toks []tok) bool {
	for _, t := range toks {
		if t.kind == 'p' && t.val == ";" {
			return true
		}
	}
	return false
}

func isSQLKeyword(s string) bool {
	switch strings.ToUpper(s) {
	case "SELECT", "FROM", "WHERE", "JOIN", "INNER", "LEFT", "RIGHT", "FULL", "OUTER", "CROSS",
		"ON", "AS", "AND", "OR", "NOT", "IN", "IS", "NULL", "TRUE", "FALSE", "CASE", "WHEN", "THEN",
		"ELSE", "END", "BETWEEN", "LIKE", "ILIKE", "GROUP", "BY", "ORDER", "ASC", "DESC", "LIMIT",
		"OFFSET", "HAVING", "QUALIFY", "DISTINCT", "ALL", "UNION", "EXCEPT", "INTERSECT", "WITH", "NATURAL",
		"USING", "COUNT", "SUM", "AVG", "MIN", "MAX", "CAST", "COALESCE", "OVER", "PARTITION", "XOR", "FILTER",
		"WINDOW", "ROWS", "RANGE", "GROUPS", "ROW", "EXCLUDE", "TIES", "UNBOUNDED", "PRECEDING", "FOLLOWING", "NULLS":
		return true
	}
	return false
}

func lex(s string) ([]tok, *protocol.ProtocolError) {
	var out []tok
	i := 0
	for i < len(s) {
		r, w := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			i += w
			continue
		}
		if r == '-' && i+1 < len(s) && s[i+1] == '-' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if r == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 >= len(s) {
				return nil, protocol.NewError(protocol.ErrInvalidSQL, "unclosed comment", nil)
			}
			i += 2
			continue
		}
		if r == '\'' {
			j := i + 1
			for j < len(s) {
				if s[j] == '\'' {
					if j+1 < len(s) && s[j+1] == '\'' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j >= len(s) {
				return nil, protocol.NewError(protocol.ErrInvalidSQL, "unclosed string", nil)
			}
			out = append(out, tok{kind: 's', val: s[i : j+1]})
			i = j + 1
			continue
		}
		if r == '"' {
			j := i + 1
			for j < len(s) && s[j] != '"' {
				j++
			}
			if j >= len(s) {
				return nil, protocol.NewError(protocol.ErrInvalidSQL, "unclosed identifier", nil)
			}
			out = append(out, tok{kind: 'i', val: s[i+1 : j]})
			i = j + 1
			continue
		}
		if unicode.IsLetter(r) || r == '_' {
			j := i + w
			for j < len(s) {
				rr, ww := utf8.DecodeRuneInString(s[j:])
				if unicode.IsLetter(rr) || unicode.IsDigit(rr) || rr == '_' {
					j += ww
					continue
				}
				break
			}
			out = append(out, tok{kind: 'i', val: s[i:j]})
			i = j
			continue
		}
		if unicode.IsDigit(r) {
			j := i + w
			for j < len(s) {
				rr, ww := utf8.DecodeRuneInString(s[j:])
				if unicode.IsDigit(rr) {
					j += ww
					continue
				}
				break
			}
			out = append(out, tok{kind: 'n', val: s[i:j]})
			i = j
			continue
		}
		if strings.ContainsRune("=<>!", r) && i+1 < len(s) {
			n := s[i+1]
			if n == '=' || (r == '<' && n == '>') {
				out = append(out, tok{kind: 'p', val: s[i : i+2]})
				i += 2
				continue
			}
		}
		out = append(out, tok{kind: 'p', val: string(r)})
		i += w
	}
	return out, nil
}
