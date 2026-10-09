// Command ff2cli rewrites Go source that uses github.com/peterbourgon/ff/v4
// to use github.com/UNO-SOFT/cli (and the standard library flag package).
//
// It performs the mechanical bulk of the migration:
//
//   - ff.NewFlagSet(x)        -> flag.NewFlagSet(x, flag.ContinueOnError)
//   - ff.NewFlagSetFrom(a, b) -> b
//   - ff flag methods         -> std flag methods (short rune dropped)
//   - short flags             -> cli.FlagConfigs entries
//   - ff.Command              -> cli.Command (Subcommands/ShortHelp/LongHelp renamed)
//   - Exec(ctx, args []string)-> Exec(ctx, *cli.State) with an "args := state.Args" prologue
//   - ff.ErrHelp              -> flag.ErrHelp
//   - ffhelp.Command(x).WriteTo(w) -> cli.PrintHelp(w, x)
//   - x.Parse(...)            -> cli.Parse(cmd, args)
//   - x.Run(ctx)              -> cli.Run(ctx, cmd, nil)
//   - imports                 -> add cli, add flag, drop ff/ffhelp
//
// What it does NOT do (reported, needs manual handling):
//   - environment variables (ff.WithEnvVarPrefix / ff.WithEnvVars)
//   - FlagConfigs for a command whose *flag.FlagSet is built in another file
//     and returned across a function boundary
//
// Usage:
//
//	go run . [-w] [-report] <dir-or-file>...
//
// Without -w the transformed source is written to stdout (single file only).
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

const (
	ffPath     = "github.com/peterbourgon/ff/v4"
	ffhelpPath = "github.com/peterbourgon/ff/v4/ffhelp"
	cliPath    = "github.com/UNO-SOFT/cli"
)

// shortRec is one short flag discovered on a flag set.
type shortRec struct {
	name  string // long flag name
	short string // one-letter short alias
}

// stdName maps an ff flag method to its standard library flag method.
var stdName = map[string]string{
	"Bool": "Bool", "BoolDefault": "Bool", "BoolVar": "BoolVar", "BoolVarDefault": "BoolVar",
	"BoolLong": "Bool", "BoolLongDefault": "Bool",
	"String": "String", "StringVar": "StringVar", "StringLong": "String",
	"StringEnum": "String", "StringEnumLong": "String",
	"Int": "Int", "IntVar": "IntVar", "IntLong": "Int",
	"Int64": "Int64", "Int64Var": "Int64Var", "Int64Long": "Int64",
	"Uint": "Uint", "UintVar": "UintVar", "UintLong": "Uint",
	"Uint64": "Uint64", "Uint64Var": "Uint64Var", "Uint64Long": "Uint64",
	"Float64": "Float64", "Float64Var": "Float64Var", "Float64Long": "Float64",
	"Duration": "Duration", "DurationVar": "DurationVar", "DurationLong": "Duration",
	"Value": "Var", "ValueLong": "Var",
}

// varForms have the pointer first and the short rune second.
var varForms = map[string]bool{
	"BoolVar": true, "BoolVarDefault": true, "StringVar": true,
	"IntVar": true, "Int64Var": true, "UintVar": true, "Uint64Var": true,
	"Float64Var": true, "DurationVar": true,
}

// longForms carry no short rune at all.
var longForms = map[string]bool{
	"BoolLong": true, "BoolLongDefault": true, "StringLong": true,
	"IntLong": true, "Int64Long": true, "UintLong": true, "Uint64Long": true,
	"Float64Long": true, "DurationLong": true, "StringEnumLong": true, "ValueLong": true,
}

func main() {
	write := flag.Bool("w", false, "write changes back to the files")
	report := flag.Bool("report", false, "print discovered FlagConfigs and warnings to stderr")
	flag.Parse()

	files := collect(flag.Args())
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "usage: ff2cli [-w] [-report] <dir-or-file>...")
		os.Exit(2)
	}
	if !*write && len(files) > 1 {
		fmt.Fprintln(os.Stderr, "refusing to print multiple files; use -w")
		os.Exit(2)
	}

	for _, fn := range files {
		src, err := os.ReadFile(fn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fn, err)
			continue
		}
		out, rep, changed, err := process(fn, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fn, err)
			continue
		}
		if !changed {
			continue
		}
		if *report {
			fmt.Fprintf(os.Stderr, "== %s\n%s", fn, rep)
		}
		if *write {
			if err := os.WriteFile(fn, out, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", fn, err)
			}
		} else {
			os.Stdout.Write(out)
		}
	}
}

// collect expands dirs into .go files (recursively), skipping vendor/hidden/testdata.
func collect(paths []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if strings.HasSuffix(p, ".go") && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			add(p)
			continue
		}
		if !fi.IsDir() {
			add(p)
			continue
		}
		filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if path == p {
					return nil
				}
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			add(path)
			return nil
		})
	}
	sort.Strings(out)
	return out
}

// process rewrites one file. It returns the formatted source, a report, and
// whether anything changed.
func process(filename string, src []byte) ([]byte, string, bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, "", false, err
	}
	if !importsFF(f) {
		return nil, "", false, nil
	}

	var rep strings.Builder
	warn := func(format string, args ...any) {
		fmt.Fprintf(&rep, "  ! "+format+"\n", args...)
	}

	cmdPtr := preScan(f)
	live := map[string][]shortRec{}
	reportShorts := map[string][]shortRec{} // command name -> shorts, for -report

	astutil.Apply(f, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.AssignStmt:
			resetLive(n.Lhs, n.Rhs, live)
		case *ast.ValueSpec:
			resetLiveSpec(n, live)
		case *ast.SelectorExpr:
			rewriteSelector(n, cmdPtr)
		case *ast.CompositeLit:
			handleCommand(n, live, cmdPtr, reportShorts, warn)
		case *ast.CallExpr:
			return handleCall(c, n, live, cmdPtr, reportShorts, warn)
		}
		return true
	}, nil)

	// Fix up imports based on what is actually referenced now.
	used := usedPackages(f)
	if used["flag"] {
		addImport(f, "flag")
	}
	if used["cli"] {
		addImport(f, cliPath)
	}
	if !used["ff"] {
		deleteImport(f, ffPath)
	} else {
		warn("leftover ff.* reference remains; inspect imports")
	}
	if !used["ffhelp"] {
		deleteImport(f, ffhelpPath)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, rep.String(), false, err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, rep.String(), false, err
	}

	if len(reportShorts) > 0 {
		names := make([]string, 0, len(reportShorts))
		for k := range reportShorts {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			fmt.Fprintf(&rep, "  FlagConfigs %s: %v\n", k, reportShorts[k])
		}
	}

	return formatted, rep.String(), true, nil
}

func importsFF(f *ast.File) bool {
	for _, imp := range f.Imports {
		if importPath(imp) == ffPath {
			return true
		}
	}
	return false
}

// preScan finds identifiers known to be *ff.Command / ff.Command so that
// x.Parse / x.Run and x.Subcommands can be rewritten with the right pointer-ness.
func preScan(f *ast.File) map[string]bool {
	ptr := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range x.Rhs {
				if i >= len(x.Lhs) {
					break
				}
				id, ok := x.Lhs[i].(*ast.Ident)
				if !ok {
					continue
				}
				if isCmdLit(unwrapAddr(rhs)) {
					ptr[id.Name] = isAddr(rhs)
				}
			}
		case *ast.ValueSpec:
			for i, v := range x.Values {
				if i >= len(x.Names) {
					break
				}
				if isCmdLit(unwrapAddr(v)) {
					ptr[x.Names[i].Name] = isAddr(v)
				}
			}
			if isPtrCmdType(x.Type) {
				for _, nm := range x.Names {
					ptr[nm.Name] = true
				}
			} else if isCmdType(x.Type) {
				for _, nm := range x.Names {
					ptr[nm.Name] = false
				}
			}
		case *ast.CallExpr:
			// ffhelp.Command(E): E is already a *Command; &E means E is a value.
			sel, ok := x.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Command" || len(x.Args) == 0 {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "ffhelp" {
				return true
			}
			switch e := x.Args[0].(type) {
			case *ast.UnaryExpr:
				if e.Op == token.AND {
					if id, ok := e.X.(*ast.Ident); ok {
						ptr[id.Name] = false
					}
				}
			case *ast.Ident:
				ptr[e.Name] = true
			}
		}
		return true
	})
	return ptr
}

func unwrapAddr(e ast.Expr) ast.Expr {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		return u.X
	}
	return e
}

func isAddr(e ast.Expr) bool {
	u, ok := e.(*ast.UnaryExpr)
	return ok && u.Op == token.AND
}

func isCmdLit(e ast.Expr) bool {
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return false
	}
	return isCmdType(cl.Type)
}

func isCmdType(e ast.Expr) bool {
	if e == nil {
		return false
	}
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Command" && isFFOrCLI(sel.X)
}

func isPtrCmdType(e ast.Expr) bool {
	if e == nil {
		return false
	}
	star, ok := e.(*ast.StarExpr)
	if !ok {
		return false
	}
	return isCmdType(star.X)
}

func isFFOrCLI(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && (id.Name == "ff" || id.Name == "cli")
}

func isFFFlagSetType(e ast.Expr) bool {
	if e == nil {
		return false
	}
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "FlagSet" && isFFOrCLI(sel.X)
}

// resetLive clears recorded shorts for identifiers assigned a new ff flag set.
func resetLive(lhs, rhs []ast.Expr, live map[string][]shortRec) {
	for i, r := range rhs {
		if i >= len(lhs) {
			break
		}
		if !isFFNew(r) {
			continue
		}
		if id, ok := lhs[i].(*ast.Ident); ok {
			live[id.Name] = nil
		}
	}
}

func resetLiveSpec(v *ast.ValueSpec, live map[string][]shortRec) {
	for i, r := range v.Values {
		if i >= len(v.Names) {
			break
		}
		if isFFNew(r) {
			live[v.Names[i].Name] = nil
		}
	}
}

func isFFNew(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "ff" {
		return false
	}
	return sel.Sel.Name == "NewFlagSet" || sel.Sel.Name == "NewFlagSetFrom"
}

// rewriteSelector renames ff.Command/ff.FlagSet/ff.ErrHelp and the
// Subcommands/ShortHelp/LongHelp fields.
func rewriteSelector(n *ast.SelectorExpr, cmdPtr map[string]bool) {
	switch n.Sel.Name {
	case "Command":
		if id, ok := n.X.(*ast.Ident); ok && id.Name == "ff" {
			id.Name = "cli"
		}
	case "FlagSet":
		if id, ok := n.X.(*ast.Ident); ok && id.Name == "ff" {
			id.Name = "flag"
		}
	case "ErrHelp":
		if id, ok := n.X.(*ast.Ident); ok && id.Name == "ff" {
			id.Name = "flag"
		}
	case "Subcommands":
		n.Sel.Name = "SubCommands"
	}
}

// renameKeys rewrites ff.Command field names used as composite literal keys.
func renameKeys(n *ast.CompositeLit) {
	for _, elt := range n.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		id, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch id.Name {
		case "Subcommands":
			id.Name = "SubCommands"
		case "ShortHelp":
			id.Name = "Summary"
		case "LongHelp":
			id.Name = "Description"
		}
	}
}

// handleCommand inserts FlagConfigs and rewrites the Exec signature of a
// command composite literal.
func handleCommand(n *ast.CompositeLit, live map[string][]shortRec, cmdPtr map[string]bool, report map[string][]shortRec, warn func(string, ...any)) {
	if !isFFOrCLIType(n.Type) {
		return
	}
	renameKeys(n)
	var flagsIdent string
	var shorts []shortRec
	for _, elt := range n.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Flags":
			if id, ok := kv.Value.(*ast.Ident); ok {
				flagsIdent = id.Name
				shorts = append([]shortRec(nil), live[id.Name]...)
			}
		case "Exec":
			fl, ok := kv.Value.(*ast.FuncLit)
			if ok {
				rewriteExec(fl)
			}
		}
	}

	if len(shorts) > 0 {
		shorts = dedupe(shorts)
		name := ""
		for _, elt := range n.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Name" {
					if lit, ok := kv.Value.(*ast.BasicLit); ok {
						name, _ = strconv.Unquote(lit.Value)
					}
				}
			}
		}
		if name == "" {
			name = flagsIdent
		}
		report[name] = append(report[name], shorts...)
		if !hasKey(n, "FlagConfigs") {
			n.Elts = append(n.Elts, &ast.KeyValueExpr{
				Key:   ast.NewIdent("FlagConfigs"),
				Value: flagConfigsLit(shorts),
			})
		}
	}
}

func isFFOrCLIType(e ast.Expr) bool {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	return isCmdType(e)
}

func hasKey(n *ast.CompositeLit, name string) bool {
	for _, elt := range n.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok && id.Name == name {
				return true
			}
		}
	}
	return false
}

// rewriteExec changes func(ctx, args []string) to func(ctx, state *cli.State)
// and, if the body uses args, prepends "args := state.Args".
func rewriteExec(fl *ast.FuncLit) {
	if fl.Type == nil || fl.Type.Params == nil || len(fl.Type.Params.List) != 2 {
		return
	}
	second := fl.Type.Params.List[1]
	if !isStringSlice(second.Type) {
		return
	}
	if len(second.Names) == 0 {
		second.Names = []*ast.Ident{ast.NewIdent("state")}
	} else {
		second.Names[0].Name = "state"
	}
	second.Type = ptrSelector("cli", "State")

	if fl.Body != nil && usesIdent(fl.Body, "args") {
		assign := &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("args")},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{&ast.SelectorExpr{X: ast.NewIdent("state"), Sel: ast.NewIdent("Args")}},
		}
		fl.Body.List = append([]ast.Stmt{assign}, fl.Body.List...)
	}
}

func isStringSlice(e ast.Expr) bool {
	arr, ok := e.(*ast.ArrayType)
	if !ok {
		return false
	}
	id, ok := arr.Elt.(*ast.Ident)
	return ok && id.Name == "string"
}

func usesIdent(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && id.Name == name {
			found = true
			return false
		}
		return true
	})
	return found
}

func ptrSelector(pkg, name string) *ast.StarExpr {
	return &ast.StarExpr{X: &ast.SelectorExpr{X: ast.NewIdent(pkg), Sel: ast.NewIdent(name)}}
}

// handleCall rewrites the various ff call forms. It returns false to stop
// descending into nodes we have fully replaced.
func handleCall(c *astutil.Cursor, n *ast.CallExpr, live map[string][]shortRec, cmdPtr map[string]bool, report map[string][]shortRec, warn func(string, ...any)) bool {
	sel, ok := n.Fun.(*ast.SelectorExpr)
	if !ok {
		return true
	}

	// ff.NewFlagSet / ff.NewFlagSetFrom
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "ff" {
		switch sel.Sel.Name {
		case "NewFlagSet":
			n.Fun = &ast.SelectorExpr{X: ast.NewIdent("flag"), Sel: ast.NewIdent("NewFlagSet")}
			n.Args = append(n.Args, &ast.SelectorExpr{X: ast.NewIdent("flag"), Sel: ast.NewIdent("ContinueOnError")})
			return false
		case "NewFlagSetFrom":
			if len(n.Args) >= 2 {
				c.Replace(n.Args[1])
			}
			return false
		}
	}

	// ffhelp.Command(E).WriteTo(W) -> cli.PrintHelp(W, E)
	if sel.Sel.Name == "WriteTo" {
		if inner, ok := sel.X.(*ast.CallExpr); ok {
			if isel, ok := inner.Fun.(*ast.SelectorExpr); ok && isel.Sel.Name == "Command" {
				if id, ok := isel.X.(*ast.Ident); ok && id.Name == "ffhelp" && len(inner.Args) >= 1 && len(n.Args) >= 1 {
					cmd := derefCmd(inner.Args[0], cmdPtr)
					w := n.Args[0]
					n.Fun = &ast.SelectorExpr{X: ast.NewIdent("cli"), Sel: ast.NewIdent("PrintHelp")}
					n.Args = []ast.Expr{w, cmd}
					return false
				}
			}
		}
	}

	// x.Parse / x.Run / x.ParseAndRun -> cli.*
	if id, ok := sel.X.(*ast.Ident); ok && cmdPtr != nil {
		if isCmd, known := cmdPtr[id.Name]; known {
			cmdExpr := derefIdent(id.Name, isCmd)
			switch sel.Sel.Name {
			case "Parse":
				if len(n.Args) >= 1 {
					for _, a := range n.Args[1:] {
						if strings.Contains(render(a), "WithEnvVar") {
							warn("%s.Parse dropped env option %s", id.Name, render(a))
						}
					}
					n.Fun = &ast.SelectorExpr{X: ast.NewIdent("cli"), Sel: ast.NewIdent("Parse")}
					n.Args = []ast.Expr{cmdExpr, n.Args[0]}
					return false
				}
			case "Run":
				n.Fun = &ast.SelectorExpr{X: ast.NewIdent("cli"), Sel: ast.NewIdent("Run")}
				n.Args = []ast.Expr{firstOr(n.Args, identNil()), cmdExpr, identNil()}
				return false
			case "ParseAndRun":
				if len(n.Args) >= 2 {
					n.Fun = &ast.SelectorExpr{X: ast.NewIdent("cli"), Sel: ast.NewIdent("ParseAndRun")}
					n.Args = []ast.Expr{n.Args[0], cmdExpr, n.Args[1], identNil()}
					return false
				}
			}
		}
	}

	// ff flag methods on an ff flag set.
	if rec, ok := rewriteFlagCall(n); ok {
		if id, ok := sel.X.(*ast.Ident); ok && rec != nil {
			live[id.Name] = append(live[id.Name], *rec)
		}
		return false
	}
	return true
}

func firstOr(args []ast.Expr, def ast.Expr) ast.Expr {
	if len(args) >= 1 {
		return args[0]
	}
	return def
}

func identNil() ast.Expr { return ast.NewIdent("nil") }

func derefIdent(name string, isPtr bool) ast.Expr {
	if isPtr {
		return ast.NewIdent(name)
	}
	return &ast.UnaryExpr{Op: token.AND, X: ast.NewIdent(name)}
}

func derefCmd(e ast.Expr, cmdPtr map[string]bool) ast.Expr {
	if id, ok := e.(*ast.Ident); ok {
		if isPtr, known := cmdPtr[id.Name]; known {
			return derefIdent(id.Name, isPtr)
		}
		return id
	}
	return e
}

// rewriteFlagCall rewrites one ff flag registration method call. It returns
// true if the call was an ff call, along with the short flag (if any).
func rewriteFlagCall(n *ast.CallExpr) (*shortRec, bool) {
	sel, ok := n.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	name := sel.Sel.Name
	std, known := stdName[name]
	if !known {
		return nil, false
	}
	// The receiver must be an ff flag set variable; this also rejects
	// unrelated methods that share a name (e.g. unique.Make(x).Value()).
	if _, ok := sel.X.(*ast.Ident); !ok {
		return nil, false
	}
	if !isFFFlagCall(name, n.Args) {
		return nil, false
	}

	var rec *shortRec
	if long := longName(name, n.Args); long != "" {
		if short := shortName(name, n.Args); short != "" {
			rec = &shortRec{name: long, short: short}
		}
	}

	n.Args = buildArgs(name, n.Args)
	sel.Sel.Name = std
	return rec, true
}

// isFFFlagCall distinguishes ff calls from std flag calls that share a name.
func isFFFlagCall(name string, args []ast.Expr) bool {
	switch name {
	case "Value":
		return len(args) == 4 && isShortLit(args[0])
	case "ValueLong":
		return len(args) == 3
	case "BoolDefault":
		return len(args) == 4 && isShortLit(args[0])
	case "BoolVarDefault":
		return len(args) == 5 && isShortLit(args[1])
	case "StringEnum":
		return len(args) >= 3 && isShortLit(args[0])
	case "StringEnumLong":
		return len(args) >= 2
	}
	if longForms[name] {
		// BoolLong/BoolLongDefault/StringLong/... take no short rune.
		return len(args) >= 2 && len(args) <= 3
	}
	if varForms[name] {
		// ff adds the short rune at index 1.
		if name == "BoolVar" {
			return len(args) == 4 && isShortLit(args[1])
		}
		return len(args) == 5 && isShortLit(args[1])
	}
	// non-var short forms
	switch name {
	case "Bool":
		return len(args) == 3 && isShortLit(args[0])
	case "String", "Int", "Int64", "Uint", "Uint64", "Float64", "Duration":
		return len(args) == 4 && isShortLit(args[0])
	}
	return false
}

func isShortLit(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	if !ok {
		return false
	}
	if lit.Kind == token.CHAR {
		return true
	}
	return lit.Kind == token.INT && lit.Value == "0"
}

func longName(name string, args []ast.Expr) string {
	idx := 1
	switch {
	case longForms[name]:
		idx = 0
	case varForms[name]:
		idx = 2
	}
	return strLitAt(args, idx)
}

func shortName(name string, args []ast.Expr) string {
	if longForms[name] {
		return ""
	}
	if varForms[name] {
		return runeAt(args, 1)
	}
	return runeAt(args, 0)
}

func strLitAt(args []ast.Expr, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	lit, ok := args[i].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return s
}

func runeAt(args []ast.Expr, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	lit, ok := args[i].(*ast.BasicLit)
	if !ok {
		return ""
	}
	switch lit.Kind {
	case token.CHAR:
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return ""
		}
		return s
	case token.INT:
		if lit.Value == "0" {
			return ""
		}
	}
	return ""
}

func buildArgs(name string, args []ast.Expr) []ast.Expr {
	switch name {
	case "BoolVar":
		// (p, short, long, usage) -> (p, long, false, usage)
		return []ast.Expr{args[0], args[2], boolLit(false), args[3]}
	case "Bool":
		// (short, long, usage) -> (long, false, usage)
		return []ast.Expr{args[1], boolLit(false), args[2]}
	case "BoolLong":
		return []ast.Expr{args[0], boolLit(false), args[1]}
	case "Value":
		// (short, long, value, usage) -> (value, long, usage)
		return []ast.Expr{args[2], args[1], args[3]}
	case "ValueLong":
		// (long, value, usage) -> (value, long, usage)
		return []ast.Expr{args[1], args[0], args[2]}
	case "StringEnum", "StringEnumLong":
		long, usage := args[1], args[2]
		var def ast.Expr = strLit("")
		if name == "StringEnumLong" {
			long, usage = args[0], args[1]
			if len(args) > 2 {
				def = args[2]
			}
		} else if len(args) > 3 {
			def = args[3]
		}
		return []ast.Expr{long, def, usage}
	}
	if longForms[name] {
		return args // no short rune to remove
	}
	if varForms[name] {
		return removeIdx(args, 1)
	}
	return removeIdx(args, 0)
}

func removeIdx(s []ast.Expr, i int) []ast.Expr {
	out := make([]ast.Expr, 0, len(s)-1)
	out = append(out, s[:i]...)
	out = append(out, s[i+1:]...)
	return out
}

func boolLit(b bool) *ast.Ident {
	if b {
		return ast.NewIdent("true")
	}
	return ast.NewIdent("false")
}

func strLit(s string) ast.Expr {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)}
}

func dedupe(in []shortRec) []shortRec {
	seen := map[string]bool{}
	var out []shortRec
	for _, r := range in {
		k := r.name + "=" + r.short
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}

func flagConfigsLit(recs []shortRec) ast.Expr {
	elts := make([]ast.Expr, 0, len(recs))
	for _, r := range recs {
		elts = append(elts, &ast.CompositeLit{
			Type: &ast.SelectorExpr{X: ast.NewIdent("cli"), Sel: ast.NewIdent("FlagConfig")},
			Elts: []ast.Expr{
				kv("Name", strLit(r.name)),
				kv("Short", strLit(r.short)),
			},
		})
	}
	return &ast.CompositeLit{
		Type: &ast.ArrayType{Elt: &ast.SelectorExpr{X: ast.NewIdent("cli"), Sel: ast.NewIdent("FlagConfig")}},
		Elts: elts,
	}
}

func kv(name string, v ast.Expr) *ast.KeyValueExpr {
	return &ast.KeyValueExpr{Key: ast.NewIdent(name), Value: v}
}

// render is a best-effort source rendering used only for warnings.
func render(n ast.Node) string {
	var b bytes.Buffer
	format.Node(&b, token.NewFileSet(), n)
	return b.String()
}

func usedPackages(f *ast.File) map[string]bool {
	used := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})
	return used
}

func importPath(s *ast.ImportSpec) string {
	p, err := strconv.Unquote(s.Path.Value)
	if err != nil {
		return ""
	}
	return p
}

func hasImport(f *ast.File, path string) bool {
	for _, s := range f.Imports {
		if importPath(s) == path {
			return true
		}
	}
	return false
}

func addImport(f *ast.File, path string) {
	if hasImport(f, path) {
		return
	}
	spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
	f.Imports = append(f.Imports, spec)
	var decl *ast.GenDecl
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			decl = g
			break
		}
	}
	if decl == nil {
		decl = &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{spec}}
		f.Decls = append([]ast.Decl{decl}, f.Decls...)
		return
	}
	decl.Specs = append(decl.Specs, spec)
}

func deleteImport(f *ast.File, path string) {
	for i, s := range f.Imports {
		if importPath(s) == path {
			f.Imports = append(f.Imports[:i], f.Imports[i+1:]...)
			break
		}
	}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.IMPORT {
			continue
		}
		out := g.Specs[:0]
		for _, s := range g.Specs {
			if is, ok := s.(*ast.ImportSpec); ok && importPath(is) == path {
				continue
			}
			out = append(out, s)
		}
		g.Specs = out
	}
}
