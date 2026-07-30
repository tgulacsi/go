package inspectsql

import (
	"fmt"
	"go/ast"
	"go/types"
	"iter"
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
	FactTypes:        []analysis.Fact{new(foundFact)},
}

func run(pass *analysis.Pass) (any, error) {
	var (
		index = pass.ResultOf[typeindex.Analyzer].(*typeindex.Index)
		info  = pass.TypesInfo
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

			// Use granular edits to preserve original formatting.
			edits := []analysis.TextEdit{
				{
					Pos:     qryArg.Pos(),
					End:     qryArg.Pos(),
					NewText: []byte("safesql.FromConstant("),
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

	if len(pass.AllObjectFacts()) > 0 {
		pass.ExportPackageFact(new(foundFact))
	}

	return nil, nil
}

// foundFact is a fact associated with functions that match -name.
// We use it to exercise the fact machinery in tests.
type foundFact struct{}

func (*foundFact) String() string { return "found" }
func (*foundFact) AFact()         {}

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
