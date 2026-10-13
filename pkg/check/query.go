package check

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"dbprobe/pkg/engine"
	"dbprobe/pkg/model"
)

func init() {
	Register("query", runQueryCheck)
}

const defaultMaxRows = 1000

// QuerySpec mendefinisikan satu query custom + ekspektasi hasilnya.
type QuerySpec struct {
	Name     string
	SQL      string
	Params   map[string]any
	Timeout  time.Duration
	ReadOnly bool
	MaxRows  int
	Expect   Expectation
}

// Expectation adalah aturan validasi hasil query.
type Expectation struct {
	RowCount    *int
	RowCountOp  string
	Cell        *CellExpect
	MaxDuration time.Duration
}

// CellExpect memvalidasi nilai satu sel.
type CellExpect struct {
	Column string
	Row    int
	Op     string
	Value  any
}

type queryData struct {
	Columns   []string `json:"columns,omitempty"`
	Rows      [][]any  `json:"rows,omitempty"`
	RowCount  int      `json:"row_count"`
	Truncated bool     `json:"truncated,omitempty"`
}

// TableColumns / TableRows memenuhi model.Tabular agar hasil query
// dapat dirender sebagai tabel di output terminal.
func (d queryData) TableColumns() []string { return d.Columns }
func (d queryData) TableRows() [][]any     { return d.Rows }

// runQueryCheck menjalankan seluruh QuerySpec milik target dan menggabungkan hasilnya.
func runQueryCheck(ctx context.Context, c Context) []model.Result {
	if len(c.Queries) == 0 {
		return []model.Result{{
			Target:  c.Target,
			Check:   "query",
			Status:  model.StatusSkip,
			Message: "tidak ada query_checks yang didefinisikan",
		}}
	}
	out := make([]model.Result, 0, len(c.Queries))
	for _, spec := range c.Queries {
		out = append(out, RunQuery(ctx, c.Target, c.Engine, c.DB, spec))
	}
	return out
}

// RunQuery menjalankan satu query dan memvalidasi hasilnya.
//
// Fungsi ini engine-agnostic: cukup bekerja di atas *sql.DB, sehingga berlaku
// untuk MySQL, PostgreSQL, Oracle, SQL Server, dst.
func RunQuery(ctx context.Context, target string, eng engine.Engine, db *sql.DB, spec QuerySpec) model.Result {
	start := time.Now()
	name := spec.Name
	if name == "" {
		name = "query"
	}
	res := model.Result{Target: target, Check: "query:" + name}

	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Guard read-only.
	if spec.ReadOnly && !isReadOnly(spec.SQL) {
		res.Status = model.StatusSkip
		res.Message = "query dilewati: bukan SELECT (mode read-only)"
		res.Hint = "jalankan dengan --allow-write untuk mengizinkan DML/DDL"
		res.SetDuration(time.Since(start))
		return res
	}

	sqlText, args, err := bindParams(spec.SQL, spec.Params, eng.Placeholder)
	if err != nil {
		res.Status = model.StatusFail
		res.Message = "gagal menyiapkan query"
		res.Err = err.Error()
		res.SetDuration(time.Since(start))
		return res
	}

	rows, err := db.QueryContext(qctx, sqlText, args...)
	if err != nil {
		st, h := classify(err)
		res.Status = st
		res.Message = "query gagal dijalankan"
		res.Hint = h
		res.Err = err.Error()
		res.SetDuration(time.Since(start))
		return res
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		res.Status = model.StatusFail
		res.Message = "gagal membaca kolom hasil"
		res.Err = err.Error()
		res.SetDuration(time.Since(start))
		return res
	}

	maxRows := spec.MaxRows
	if maxRows <= 0 {
		maxRows = defaultMaxRows
	}
	data, truncated, err := scanRows(rows, cols, maxRows)
	res.SetDuration(time.Since(start))
	if err != nil {
		res.Status = model.StatusFail
		res.Message = "gagal membaca hasil query"
		res.Err = err.Error()
		return res
	}

	res.Data = queryData{Columns: cols, Rows: data, RowCount: len(data), Truncated: truncated}

	if err := validate(spec.Expect, cols, data, res.Duration); err != nil {
		res.Status = model.StatusFail
		res.Message = err.Error()
		return res
	}

	res.Status = model.StatusOK
	res.Message = fmt.Sprintf("%d baris", len(data))
	if truncated {
		res.Message = fmt.Sprintf(">= %d baris (dipotong)", len(data))
	}
	return res
}

// classify memetakan error DB menjadi status + hint yang actionable.
//
//	permission -> SKIP (butuh privilege tambahan)
//	not exist  -> WARN (objek/capability tidak tersedia)
//	timeout    -> WARN
//	lainnya    -> FAIL
func classify(err error) (model.Status, string) {
	if err == nil {
		return model.StatusOK, ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return model.StatusWarn, "query melebihi batas waktu (timeout)"
	}

	low := strings.ToLower(err.Error())
	switch {
	case isPermission(low):
		return model.StatusSkip, "butuh privilege tambahan (mis. SELECT_CATALOG_ROLE / SELECT ANY DICTIONARY / VIEW SERVER STATE)"
	case isMissing(low):
		return model.StatusWarn, "objek tidak tersedia di server ini (cek nama tabel/schema)"
	case strings.Contains(low, "timeout"), strings.Contains(low, "deadline exceeded"),
		strings.Contains(low, "context canceled"):
		return model.StatusWarn, "query melebihi batas waktu (timeout)"
	default:
		return model.StatusFail, ""
	}
}

func isPermission(low string) bool {
	for _, s := range []string{
		"denied",
		"permission",
		"privilege",
		"not authorized",
		"access denied",
		"ora-00942", // Oracle menyembunyikan "no privilege" sebagai "table or view does not exist"
		"ora-01031",
		"ora-01039",
		"42501", // PostgreSQL insufficient_privilege
	} {
		if strings.Contains(low, s) {
			return true
		}
	}
	return false
}

func isMissing(low string) bool {
	for _, s := range []string{
		"does not exist",
		"doesn't exist",
		"unknown database",
		"unknown table",
		"no such table",
		"undefined table",
		"invalid object name",
		"42p01", // PostgreSQL undefined_table
	} {
		if strings.Contains(low, s) {
			return true
		}
	}
	return false
}

// validate menerapkan seluruh aturan ekspektasi terhadap hasil query.
func validate(exp Expectation, cols []string, data [][]any, dur time.Duration) error {
	if exp.RowCount != nil {
		op := exp.RowCountOp
		if op == "" {
			op = "="
		}
		if !cmpInt(int64(len(data)), op, int64(*exp.RowCount)) {
			return fmt.Errorf("row count tidak sesuai: dapat %d, diharapkan %s %d", len(data), op, *exp.RowCount)
		}
	}
	if exp.Cell != nil {
		ce := exp.Cell
		if ce.Row < 0 || ce.Row >= len(data) {
			return fmt.Errorf("baris %d tidak ada (hanya %d baris)", ce.Row, len(data))
		}
		idx := indexOf(cols, ce.Column)
		if idx < 0 {
			return fmt.Errorf("kolom %q tidak ada di hasil (kolom: %s)", ce.Column, strings.Join(cols, ", "))
		}
		if err := cmpValue(data[ce.Row][idx], ce.Op, ce.Value); err != nil {
			return fmt.Errorf("kolom %q: %w", ce.Column, err)
		}
	}
	if exp.MaxDuration > 0 && dur > exp.MaxDuration {
		return fmt.Errorf("query terlalu lambat: %s > %s", dur, exp.MaxDuration)
	}
	return nil
}

// bindParams mengganti parameter bernama (:nama) menjadi placeholder engine.
func bindParams(sqlText string, params map[string]any, ph func(int) string) (string, []any, error) {
	if len(params) == 0 {
		return sqlText, nil, nil
	}
	var b strings.Builder
	var args []any
	pos := 0
	i := 0
	for i < len(sqlText) {
		ch := sqlText[i]
		if ch == ':' && i+1 < len(sqlText) && isIdentStart(sqlText[i+1]) &&
			(i == 0 || sqlText[i-1] != ':') {
			j := i + 1
			for j < len(sqlText) && isIdentPart(sqlText[j]) {
				j++
			}
			name := sqlText[i+1 : j]
			val, ok := params[name]
			if !ok {
				return "", nil, fmt.Errorf("parameter %q tidak ditemukan di config", name)
			}
			pos++
			b.WriteString(ph(pos))
			args = append(args, val)
			i = j
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String(), args, nil
}

// isReadOnly mengembalikan true bila statement termasuk read-only.
func isReadOnly(sqlText string) bool {
	s := sqlText
	for {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "--"), strings.HasPrefix(s, "#"):
			idx := strings.IndexByte(s, '\n')
			if idx < 0 {
				return false
			}
			s = s[idx+1:]
			continue
		case strings.HasPrefix(s, "/*"):
			idx := strings.Index(s, "*/")
			if idx < 0 {
				return false
			}
			s = s[idx+2:]
			continue
		}
		break
	}
	up := strings.ToUpper(s)
	for _, kw := range []string{"SELECT", "WITH", "SHOW", "EXPLAIN", "DESCRIBE", "DESC", "PRAGMA", "VALUES", "TABLE"} {
		if strings.HasPrefix(up, kw) {
			rest := up[len(kw):]
			if rest == "" || !isIdentPart(rest[0]) {
				return true
			}
		}
	}
	return false
}

func scanRows(rows *sql.Rows, cols []string, maxRows int) ([][]any, bool, error) {
	var out [][]any
	raw := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	truncated := false
	for rows.Next() {
		if maxRows > 0 && len(out) >= maxRows {
			truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return out, truncated, err
		}
		row := make([]any, len(cols))
		for i, v := range raw {
			switch t := v.(type) {
			case []byte:
				row[i] = string(t)
			case time.Time:
				row[i] = t.Format(time.RFC3339Nano)
			default:
				row[i] = t
			}
		}
		out = append(out, row)
	}
	return out, truncated, rows.Err()
}

func indexOf(cols []string, name string) int {
	for i, c := range cols {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}

func cmpInt(got int64, op string, want int64) bool {
	switch op {
	case "", "=":
		return got == want
	case "!=":
		return got != want
	case ">":
		return got > want
	case "<":
		return got < want
	case ">=":
		return got >= want
	case "<=":
		return got <= want
	}
	return false
}

func cmpValue(got any, op string, want any) error {
	switch op {
	case "contains":
		return boolErr(strings.Contains(toStr(got), toStr(want)), op, got, want)
	case "regex":
		re, err := regexp.Compile(toStr(want))
		if err != nil {
			return fmt.Errorf("regex tidak valid: %w", err)
		}
		return boolErr(re.MatchString(toStr(got)), op, got, want)
	}

	if gf, gok := toFloat(got); gok {
		if wf, wok := toFloat(want); wok {
			return boolErr(cmpFloat(gf, op, wf), op, got, want)
		}
	}

	gs, ws := toStr(got), toStr(want)
	switch op {
	case "", "=":
		return boolErr(gs == ws, op, got, want)
	case "!=":
		return boolErr(gs != ws, op, got, want)
	case ">":
		return boolErr(gs > ws, op, got, want)
	case "<":
		return boolErr(gs < ws, op, got, want)
	case ">=":
		return boolErr(gs >= ws, op, got, want)
	case "<=":
		return boolErr(gs <= ws, op, got, want)
	}
	return fmt.Errorf("operator tidak dikenal: %q", op)
}

func cmpFloat(a float64, op string, b float64) bool {
	switch op {
	case "", "=":
		return a == b
	case "!=":
		return a != b
	case ">":
		return a > b
	case "<":
		return a < b
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	}
	return false
}

func boolErr(ok bool, op string, got, want any) error {
	if ok {
		return nil
	}
	return fmt.Errorf("dapat %v, diharapkan %q %v", got, op, want)
}

func toStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int8:
		return float64(t), true
	case int16:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint8:
		return float64(t), true
	case uint16:
		return float64(t), true
	case uint32:
		return float64(t), true
	case uint64:
		return float64(t), true
	case float32:
		return float64(t), true
	case float64:
		if math.IsNaN(t) {
			return 0, false
		}
		return t, true
	case string:
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isIdentPart(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}
