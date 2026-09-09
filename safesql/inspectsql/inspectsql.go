package inspectsql

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"iter"
	"reflect"
	"strings"

	"github.com/tgulacsi/go/safesql/inspectsql/typeindex" // golang.org/x/tools/internal/typesinternal/typeindex
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/types/typeutil"
)

const Doc = `find calls to database/sql with query string not a safesql.SQL`

var Analyzer = &analysis.Analyzer{
	Name:             "inspectsql",
	Doc:              Doc,
	URL:              "https://pkg.go.dev/github.com/tgulacsi/go/safesql/inspectsql",
	Run:              run,
	RunDespiteErrors: true,
	Requires:         []*analysis.Analyzer{inspect.Analyzer, typeindex.Analyzer},
	FactTypes:        []analysis.Fact{(*SQLQuery)(nil)},
	ResultType:       reflect.TypeFor[[]string](),
}

func run(pass *analysis.Pass) (any, error) {
	var (
		index        = pass.ResultOf[typeindex.Analyzer].(*typeindex.Index)
		info         = pass.TypesInfo
		constQueries []string
	)
	for callee := range iterObjs(index) {
		for curCall := range index.Calls(callee) {
			call, ok := curCall.Node().(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				continue
			}
			var idx int
			if nm := typeutil.Callee(info, call).Name(); strings.HasSuffix(nm, "Context") {
				idx = 1
			}
			if len(call.Args) <= idx {
				continue
			}
			qryArg := call.Args[idx]
			if qryCall, ok := qryArg.(*ast.CallExpr); ok && typeutil.Callee(info, qryCall).Name() == "String" && len(qryCall.Args) == 0 {
				// OK safesql.SQL.String
				continue
			}

			// A constant string is safe: collect its value for the
			// "collect" command and don't report it as a diagnostic.
			if tv, ok := info.Types[qryArg]; ok && tv.Value != nil {
				if tv.Value.Kind() == constant.String {
					s := constant.StringVal(tv.Value)
					pass.ExportPackageFact(&SQLQuery{
						Query:    s,
						Position: pass.Fset.Position(qryArg.Pos()),
					})
					constQueries = append(constQueries, s)
					// OK: constant string query
					continue
				}
			}

			// Non-constant string argument: suggest wrapping it with
			// safesql.Make so it is marked as SQL.
			// Use granular edits to preserve original formatting.
			edits := []analysis.TextEdit{
				{
					Pos:     qryArg.Pos(),
					End:     qryArg.Pos(),
					NewText: []byte("safesql.Make("),
				},
				{
					Pos:     qryArg.End(),
					End:     qryArg.End(),
					NewText: []byte(")"),
				},
			}

			pass.Report(analysis.Diagnostic{
				// Highlight the format string.
				Pos:     qryArg.Pos(),
				End:     qryArg.End(),
				Message: fmt.Sprintf("use safesql.SQL(%s)", qryArg),
				SuggestedFixes: []analysis.SuggestedFix{{
					Message:   "use safesql.SQL instead of string",
					TextEdits: edits,
				}},
			})
		}
	}

	return constQueries, nil
}

// foundFact is a fact associated with functions that match -name.
// We use it to exercise the fact machinery in tests.
type SQLQuery struct {
	token.Position
	Query string
}

func (qry *SQLQuery) String() string { return qry.Query }
func (*SQLQuery) AFact()             {}

func iterObjs(index *typeindex.Index) iter.Seq[types.Object] {
	return func(yield func(types.Object) bool) {
		for _, t := range []string{"Conn", "DB", "Tx"} {
			for _, f := range []string{"Exec", "Prepare", "Query"} {
				if !yield(index.Selection("database/sql", t, f)) {
					return
				}
				if !yield(index.Selection("database/sql", t, f+"Context")) {
					return
				}
			}
		}
	}
}
