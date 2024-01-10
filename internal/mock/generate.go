package mock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/tools/go/packages"
)

// GenerateResult stores the result for a package from a call to Generate.
type GenerateResult struct {
	// PkgPath is the package's PkgPath.
	PkgPath string
	// OutputPath is the path where the generated output should be written.
	// May be empty if there were errors.
	OutputPath string
	// Content is the gofmt'd source code that was generated. May be nil if
	// there were errors during generation.
	Content []byte
	// Errs is a slice of errors identified during generation.
	Errs []error
}

// Commit writes the generated file to disk.
func (gen GenerateResult) Commit() error {
	if len(gen.Content) == 0 {
		return nil
	}
	return os.WriteFile(gen.OutputPath, gen.Content, 0666)
}

// GenerateOptions holds options for Generate.
type GenerateOptions struct {
	// Header will be inserted at the start of each generated file.
	Header []byte

	// PrefixOutputFile is the prefix of the file name to write the generated
	// output to. The suffix will be "vermock_gen.go" or "vermock_gen_test.go".
	PrefixOutputFile string

	// Tags is a list of additional build tags to add to the generated file.
	Tags string

	// Dir is the directory to run the build system's query tool
	// that provides information about the packages.
	// If Dir is empty, the tool is run in the current directory.
	Dir string

	// Env is the environment to use when invoking the build system's query tool.
	// If Env is nil, the current environment is used.
	// As in os/exec's Cmd, only the last value in the slice for
	// each environment key is used.
	Env []string
}

// GenerateOption modifies a GenerateOptions value and be used to configure
// Generate.
type GenerateOption func(*GenerateOptions) error

// WithDir sets the directory to run the build system's query tool.
func WithDir(dir string) GenerateOption {
	return func(opts *GenerateOptions) error {
		opts.Dir = dir
		return nil
	}
}

// WithWDFallback sets the directory to run the build system's query tool to
// the current working directory if Dir is empty.
func WithWDFallback() GenerateOption {
	return func(opts *GenerateOptions) error {
		if opts.Dir != "" {
			return nil
		}
		wd, err := os.Getwd()
		if err != nil {
			err = fmt.Errorf("failed to get working directory: %w", err)
			return err
		}
		opts.Dir = wd
		return nil
	}
}

// WithPrefixFileName sets the prefix of the file name to write the generated
// output to. The suffix will be "vermock_gen.go" or "vermock_gen_test.go".
func WithPrefixFileName(prefix string) GenerateOption {
	return func(opts *GenerateOptions) error {
		opts.PrefixOutputFile = prefix
		return nil
	}
}

// WithTags sets the build tags to use when generating the mock files.
func WithTags(tags string) GenerateOption {
	return func(opts *GenerateOptions) error {
		opts.Tags = tags
		return nil
	}
}

// WithHeader sets the header to insert at the start of each generated file.
func WithHeader(header []byte) GenerateOption {
	return func(opts *GenerateOptions) error {
		opts.Header = header
		return nil
	}
}

// WithHeaderFile sets the header to insert at the start of each generated file
// to the contents of the given file.
func WithHeaderFile(headerFile string) GenerateOption {
	return func(opts *GenerateOptions) error {
		if headerFile == "" {
			return nil
		}
		header, err := os.ReadFile(headerFile)
		if err != nil {
			err = fmt.Errorf("failed to read header file %q: %w", headerFile, err)
			return err
		}
		opts.Header = header
		return nil
	}
}

// WithEnv sets the environment to use when invoking the build system's query
// tool.
func WithEnv(env []string) GenerateOption {
	return func(opts *GenerateOptions) error {
		opts.Env = env
		return nil
	}
}

// WithArgs applies each GenerateOption in the given slice.  If any of the
// GenerateOptions return an error, WithArgs will return the error immediately.
// The args use the any type to be compatible with the subcommands package.
func WithArgs(args ...any) GenerateOption {
	return func(opts *GenerateOptions) (err error) {
		for _, arg := range args {
			switch arg := arg.(type) {
			case GenerateOption:
				err = arg(opts)
				if err != nil {
					err = fmt.Errorf("failed to apply generate option: %w", err)
					return
				}
			default:
				err = fmt.Errorf("unexpected argument type %T", arg)
				return
			}
		}
		return nil
	}
}

// Generate generates a code file for each package matching the given patterns.
// The code file will contain mock implementations for each struct type in any
// file in the package that has the vermockstub build tag.  As a consequence, the
// generated files will not be included in the package's build when using the
// vermockstub build tag.  An implementation for each method of each interface
// type that the struct type embeds will be generated, unless an implementation
// already exists elsewhere in the package.
// The generated files will be named vermock_gen.go, with an optional prefix.
// The generated files will also include a go:generate comment that can be used
// to regenerate the file.
func Generate(ctx context.Context, patterns []string, opts GenerateOptions) ([]GenerateResult, []error) {
	tags := "-tags=vermockstub"
	if opts.Tags != "" {
		tags += " " + opts.Tags
	}

	pkgs, errs := load(ctx, opts.Dir, opts.Env, []string{tags}, patterns)
	if len(errs) > 0 {
		return nil, errs
	}

	generated := make([]GenerateResult, len(pkgs))
	for i, pkg := range pkgs {
		generated[i].PkgPath = pkg.PkgPath
		outDir, err := detectOutputDir(pkg.GoFiles)
		if err != nil {
			generated[i].Errs = append(generated[i].Errs, err)
			continue
		}

		outputFile := opts.PrefixOutputFile + "vermock_gen"
		if strings.HasSuffix(pkg.Name, "_test") {
			outputFile += "_test"
		}
		outputFile += ".go"
		generated[i].OutputPath = filepath.Join(outDir, outputFile)

		g := newGen(pkg)
		findFunctions(g, pkg)
		errs := generateMocks(g, pkg)
		if len(errs) > 0 {
			generated[i].Errs = errs
			continue
		}

		goSrc := g.frame(opts.Tags)
		if len(opts.Header) > 0 {
			goSrc = append(opts.Header, goSrc...)
		}
		fmtSrc, err := format.Source(goSrc)
		if err != nil {
			// This is likely a bug from a poorly generated source file.
			// Add an error but also the unformatted source.
			generated[i].Errs = append(generated[i].Errs, err)
		} else {
			goSrc = fmtSrc
		}
		generated[i].Content = goSrc
	}

	return generated, nil
}

func detectOutputDir(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", errors.New("no files to derive output directory from")
	}
	dir := filepath.Dir(paths[0])
	for _, p := range paths[1:] {
		if dir2 := filepath.Dir(p); dir2 != dir {
			return "", fmt.Errorf("found conflicting directories %q and %q", dir, dir2)
		}
	}
	return dir, nil
}

func isMockStub(syntax *ast.File) bool {
	for _, group := range syntax.Comments {
		for _, comment := range group.List {
			if comment.Text == "// +build vermockstub" {
				return true
			}
			if strings.HasPrefix(comment.Text, "//go:build vermockstub") {
				return true
			}
		}
	}
	return false
}

func findFunctions(g *gen, pkg *packages.Package) {
	pkgName, _ := g.resolvePackageName("github.com/Versent/go-vermock")
	for _, syntax := range pkg.Syntax {
		for _, decl := range syntax.Decls {
			funcDecl, ok := g.addFunc(decl)
			if !ok {
				continue
			}
			if pkgName == "" {
				// vermock is not imported,
				// so there cannot not be any custom functions
				continue
			}
			if funcDecl.Recv != nil || funcDecl.Body == nil {
				continue
			}
			// search for calls to vermock.Expect or vermock.ExpectMany
			var funcName, structName, methodName string
			ast.Inspect(funcDecl.Body, func(node ast.Node) (next bool) {
				next = true
				switch stmt := node.(type) {
				case *ast.CallExpr:
					var (
						ok    bool
						index *ast.IndexExpr
						sel   *ast.SelectorExpr
						ident *ast.Ident
						lit   *ast.BasicLit
					)
					if index, ok = stmt.Fun.(*ast.IndexExpr); !ok {
						return
					}
					if sel, ok = index.X.(*ast.SelectorExpr); !ok {
						return
					}
					if sel.Sel.Name != "Expect" && sel.Sel.Name != "ExpectMany" {
						return
					}
					if ident, ok := sel.X.(*ast.Ident); !ok || ident.Name != pkgName {
						return
					}
					funcName = sel.Sel.Name
					if ident, ok = index.Index.(*ast.Ident); !ok {
						return
					}
					structName = ident.Name
					if len(stmt.Args) == 0 {
						return
					}
					if lit, ok = stmt.Args[0].(*ast.BasicLit); !ok || lit.Kind != token.STRING {
						return
					}
					if methodName != "" {
						methodName = ""
						return false
					}
					methodName = lit.Value
				}
				return
			})
			if methodName == "" {
				continue
			}
			specName := fmt.Sprintf("%s[%s](%s)", funcName, structName, methodName)
			g.funcs[specName] = struct{}{}
		}
	}
}

func generateMocks(g *gen, pkg *packages.Package) (errs []error) {
	for _, syntax := range pkg.Syntax {
		if !isMockStub(syntax) {
			continue
		}

		// Iterate over all declarations in the file
		for _, decl := range syntax.Decls {
			// Process only GenDecl (General Declarations like import, const, type, var)
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				if err := g.addDecl(nil, decl); err != nil {
					errs = append(errs, err)
				}
				continue
			}

			// Iterate over the specs in the GenDecl
			for _, spec := range genDecl.Specs {
				// Process only TypeSpec (Type Declarations)
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				obj := pkg.TypesInfo.ObjectOf(typeSpec.Name)

				// Process only structs
				structType, ok := obj.Type().Underlying().(*types.Struct)
				if !ok {
					decl := &ast.GenDecl{
						Tok: token.TYPE,
						Specs: []ast.Spec{
							clone(typeSpec),
						},
					}
					if err := g.addDecl(nil, decl); err != nil {
						errs = append(errs, err)
					}
					continue
				}

				mockFields := &ast.FieldList{
					List: []*ast.Field{},
				}
				mockDecl := &ast.GenDecl{
					Tok: token.TYPE,
					Specs: []ast.Spec{
						&ast.TypeSpec{
							Doc:     clone(typeSpec.Doc),
							Comment: clone(typeSpec.Comment),
							Name:    clone(typeSpec.Name),
							Type: &ast.StructType{
								Fields: mockFields,
							},
						},
					},
				}

				mockSize := pkg.TypesSizes.Sizeof(structType)

				// Check for embedded interfaces and generate mock methods
				for i := 0; i < structType.NumFields(); i++ {
					field := structType.Field(i)
					if field.Embedded() {
						// Generate:
						//   var _ <ifaceType> = (*<typeSpec.Name>)(nil)
						err := g.addInterfaceAssertion(
							*clone(&typeSpec.Type.(*ast.StructType).Fields.List[i].Type),
							clone(typeSpec.Name),
						)
						if err != nil {
							errs = append(errs, err)
						}

						ifaceType, ok := field.Type().Underlying().(*types.Interface)
						if ok {
							mockSize -= pkg.TypesSizes.Sizeof(field.Type())
							if err := generateMockMethods(g, ifaceType, typeSpec.Name.Name); err != nil {
								errs = append(errs, err)
							}
							continue
						}
					}
					mockFields.List = append(mockFields.List, clone(typeSpec.Type.(*ast.StructType).Fields.List[i]))
				}

				if mockSize == 0 {
					mockFields.List = append(mockFields.List, &ast.Field{
						Names: []*ast.Ident{{Name: "_"}},
						Type:  ast.NewIdent("byte"),
						Comment: &ast.CommentGroup{
							List: []*ast.Comment{{
								Text: "// prevent zero-size struct",
							}},
						},
					})
				}

				// Add the mock struct to the file
				err := g.addDecl(typeSpec.Name, mockDecl)
				if err != nil {
					errs = append(errs, err)
				}
			}
		}

		for _, impt := range syntax.Imports {
			if impt.Name != nil && impt.Name.Name == "_" {
				g.anonImports[impt.Path.Value] = true
			}
		}
	}

	return errs
}

func generateMockMethods(g *gen, iface *types.Interface, structName string) error {
	// Iterate through each method in the interface
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		methodName := method.Name()
		sig := method.Type().(*types.Signature)

		if err := addExpectFunc(g, "Expect", structName, methodName, sig); err != nil {
			return err
		}
		if err := addExpectFunc(g, "ExpectMany", structName, methodName, sig); err != nil {
			return err
		}
		if err := addMockMethod(g, structName, methodName, sig); err != nil {
			return err
		}
	}

	return nil
}

func addMockMethod(g *gen, structName, methodName string, sig *types.Signature) (err error) {
	// Start building the function declaration
	methDecl := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "m"}},
					Type: &ast.StarExpr{
						X: ast.NewIdent(structName),
					},
				},
			},
		},
		Name: ast.NewIdent(methodName),
		Type: &ast.FuncType{},
	}

	if _, ok := g.funcs[g.keyForFunc(methDecl)]; ok {
		// Method already exists
		return
	}

	methDecl.Type.Params = fieldList("v", sig.Variadic(), sig.Params())
	methDecl.Type.Results = fieldList("", false, sig.Results())

	// Create a function body (block statement)
	methDecl.Body = &ast.BlockStmt{List: []ast.Stmt{}}
	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(g.resolveImportName("vermock", "github.com/Versent/go-vermock")),
			Sel: ast.NewIdent(fmt.Sprintf("Call%d", sig.Results().Len())),
		},
		Args: []ast.Expr{
			ast.NewIdent("m"),
			&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", methodName)},
		},
	}
	forTuple("v", sig.Params(), func(_ int, name string, _ *types.Var) {
		call.Args = append(call.Args, ast.NewIdent(name))
	})
	if sig.Results().Len() > 0 {
		indices := make([]ast.Expr, sig.Results().Len())
		call.Fun = &ast.IndexListExpr{
			X:       call.Fun,
			Indices: indices,
		}
		for i, field := range methDecl.Type.Results.List {
			indices[i] = *clone(&field.Type)
		}
		methDecl.Body.List = append(methDecl.Body.List, &ast.ReturnStmt{
			Results: []ast.Expr{call},
		})
	} else {
		methDecl.Body.List = append(methDecl.Body.List, &ast.ExprStmt{
			X: call,
		})
	}

	// Generate the source code for the function
	return g.addDecl(methDecl.Name, methDecl)
}

func addExpectFunc(g *gen, funcName, structName, methodName string, sig *types.Signature) error {
	specName := fmt.Sprintf("%s[%s](%q)", funcName, structName, methodName)
	if _, ok := g.funcs[specName]; ok {
		// Custom implementation already exists
		return nil
	}

	// Disambiguate the function name
	name := ast.NewIdent(funcName + methodName)
	if _, ok := g.funcs[name.Name]; ok {
		if token.IsExported(structName) {
			name = ast.NewIdent(funcName + structName + methodName)
		} else {
			name = ast.NewIdent(funcName + cases.Title(language.AmericanEnglish, cases.NoLower).String(structName) + methodName)
		}
	}
	if _, ok := g.funcs[name.Name]; ok {
		name = ast.NewIdent(name.Name + "T")
	}
	if _, ok := g.funcs[name.Name]; ok {
		return fmt.Errorf("unable to disambiguate function name %q", name.Name)
	}

	delegateType := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{{
				Names: []*ast.Ident{{Name: "_"}},
				Type: &ast.SelectorExpr{
					X:   ast.NewIdent(g.resolveImportName("testing", "testing")),
					Sel: ast.NewIdent("TB"),
				},
			}},
		},
	}
	if funcName == "ExpectMany" {
		delegateType.Params.List = append(delegateType.Params.List, &ast.Field{
			Names: []*ast.Ident{{Name: "_"}},
			Type: &ast.SelectorExpr{
				X:   ast.NewIdent(g.resolveImportName("vermock", "github.com/Versent/go-vermock")),
				Sel: ast.NewIdent("CallCount"),
			},
		})
	}
	funcDecl := &ast.FuncDecl{
		Name: name,
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{{
					Type: &ast.FuncType{
						Params: &ast.FieldList{
							List: []*ast.Field{{
								Type: &ast.StarExpr{
									X: ast.NewIdent(structName),
								},
							}},
						},
					},
				}},
			},
			Params: &ast.FieldList{
				List: []*ast.Field{{
					Names: []*ast.Ident{{Name: "delegate"}},
					Type:  delegateType,
				}},
			},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{&ast.CallExpr{
					Fun: &ast.IndexListExpr{
						X: &ast.SelectorExpr{
							X:   ast.NewIdent(g.resolveImportName("vermock", "github.com/Versent/go-vermock")),
							Sel: ast.NewIdent(funcName),
						},
						Indices: []ast.Expr{ast.NewIdent(structName)},
					},
					Args: []ast.Expr{
						&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", methodName)},
						ast.NewIdent("delegate"),
					},
				}},
			},
		}},
	}
	forTuple("v", sig.Params(), func(_ int, name string, t *types.Var) {
		delegateType.Params.List = append(delegateType.Params.List, &ast.Field{
			Names: []*ast.Ident{{Name: name}},
			Type:  ast.NewIdent(t.Type().String()),
		})
	})
	forTuple("", sig.Results(), func(_ int, name string, t *types.Var) {
		field := &ast.Field{
			Type: ast.NewIdent(t.Type().String()),
		}
		if name != "" {
			field.Names = []*ast.Ident{{Name: name}}
		}
		if delegateType.Results == nil {
			delegateType.Results = &ast.FieldList{}
		}
		delegateType.Results.List = append(delegateType.Results.List, field)
	})

	g.funcs[specName] = struct{}{}

	// Generate the source code for the function
	return g.addDecl(funcDecl.Name, funcDecl)
}

func forTuple(prefix string, tuple *types.Tuple, f func(int, string, *types.Var)) {
	for i := 0; i < tuple.Len(); i++ {
		param := tuple.At(i)

		name := param.Name()
		if name == "" && prefix != "" {
			name = prefix + strconv.Itoa(i)
		}

		f(i, name, param)
	}
}

// fieldList returns a field list for the given tuple.
func fieldList(prefix string, variadic bool, tuple *types.Tuple) *ast.FieldList {
	if tuple == nil {
		return nil
	}
	fields := make([]*ast.Field, tuple.Len())
	forTuple(prefix, tuple, func(i int, name string, param *types.Var) {
		fields[i] = &ast.Field{}
		if variadic && i == tuple.Len()-1 {
			fields[i].Type = &ast.Ellipsis{
				Elt: ast.NewIdent(param.Type().(*types.Slice).Elem().String()),
			}
		} else {
			fields[i].Type = ast.NewIdent(param.Type().String())
		}

		if name == "" {
			return
		}
		fields[i].Names = []*ast.Ident{{Name: name}}
	})
	return &ast.FieldList{List: fields}
}

// importInfo holds info about an import.
type importInfo struct {
	// name is the identifier that is used in the generated source.
	name string
	// differs is true if the import is given an identifier that does not
	// match the package's identifier.
	differs bool
	// copied is true if the import is copied from the spec file
	copied bool
}

// gen is the file-wide generator state.
type gen struct {
	pkg         *packages.Package
	buf         bytes.Buffer
	imports     map[string]importInfo
	anonImports map[string]bool
	values      map[ast.Expr]string
	funcs       map[string]struct{}
}

func newGen(pkg *packages.Package) *gen {
	return &gen{
		pkg:         pkg,
		anonImports: make(map[string]bool),
		imports:     make(map[string]importInfo),
		values:      make(map[ast.Expr]string),
		funcs:       make(map[string]struct{}),
	}
}

func (g *gen) addDecl(name fmt.Stringer, decl ast.Decl) error {
	if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
		for _, spec := range genDecl.Specs {
			importSpec := spec.(*ast.ImportSpec)
			var name string
			if importSpec.Name != nil {
				name = importSpec.Name.Name
			} else {
				var ok bool
				name, ok = g.resolvePackageName(importSpec.Path.Value)
				if !ok {
					continue
				}
			}
			if name != "_" {
				imp, ok := g.imports[importSpec.Path.Value]
				if ok {
					imp.copied = true
				} else {
					imp = importInfo{
						name:    name,
						differs: importSpec.Name != nil,
						copied:  true,
					}
				}
				g.imports[importSpec.Path.Value] = imp
			}
		}
	}
	g.addFunc(decl)
	var buf bytes.Buffer
	if err := format.Node(&buf, g.pkg.Fset, decl); err != nil {
		if name == nil {
			name = g.pkg.Fset.Position(decl.Pos())
		}
		return fmt.Errorf("%s: error formatting struct: %w", name, err)
	}
	g.buf.Write(buf.Bytes())
	g.buf.WriteString("\n\n") // Add some spacing between decls
	return nil
}

func (g *gen) keyForFunc(funcDecl *ast.FuncDecl) (key string) {
	if funcDecl.Recv == nil {
		return funcDecl.Name.String()
	} else if len(funcDecl.Recv.List) == 1 {
		recv := bytes.Buffer{}
		err := format.Node(&recv, g.pkg.Fset, funcDecl.Recv.List[0].Type)
		if err != nil {
			return
		}
		return recv.String() + "." + funcDecl.Name.String()
	}
	return
}

func (g *gen) addFunc(decl ast.Decl) (funcDecl *ast.FuncDecl, ok bool) {
	if funcDecl, ok = decl.(*ast.FuncDecl); ok {
		key := g.keyForFunc(funcDecl)
		if key == "" {
			ok = false
			return
		}
		g.funcs[key] = struct{}{}
	}
	return
}

func (g *gen) resolvePackageName(path string) (string, bool) {
	for _, pkg := range g.pkg.Imports {
		if pkg.PkgPath == path {
			return pkg.Name, true
		}
	}
	return "", false
}

func (g *gen) resolveImportName(name, path string) string {
	imp, ok := g.imports[fmt.Sprintf("%q", path)]
	if !ok {
		imp = importInfo{
			name:    name,
			differs: true,
		}
		g.imports[path] = imp
	}
	return imp.name
}

func (g *gen) addInterfaceAssertion(ifaceType, structName ast.Expr) error {
	varDecl := &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{{Name: "_"}},
				Type:  ifaceType,
				Values: []ast.Expr{
					&ast.CallExpr{
						Fun: &ast.ParenExpr{
							X: &ast.StarExpr{
								X: structName,
							},
						},
						Args: []ast.Expr{
							ast.NewIdent("nil"),
						},
					},
				},
			},
		},
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, g.pkg.Fset, varDecl); err != nil {
		return fmt.Errorf("%s: error formatting var: %w", ifaceType, err)
	}
	g.buf.Write(buf.Bytes())
	g.buf.WriteString("\n\n") // Add some spacing between decls
	return nil
}

// frame bakes the built up source body into an unformatted Go source file.
func (g *gen) frame(tags string) []byte {
	if g.buf.Len() == 0 {
		return nil
	}
	var buf bytes.Buffer
	if len(tags) > 0 {
		tags = fmt.Sprintf(" gen -tags %q", tags)
	}
	buf.WriteString("// Code generated by vermockgen. DO NOT EDIT.\n\n")
	buf.WriteString("//go:generate go run -mod=mod github.com/Versent/go-vermock/cmd/vermockgen" + tags + "\n")
	buf.WriteString("//+build !vermockstub\n\n")
	buf.WriteString("package ")
	buf.WriteString(g.pkg.Name)
	buf.WriteString("\n\n")
	imps := make([]string, 0, len(g.imports))
	for path, imp := range g.imports {
		if !imp.copied {
			imps = append(imps, path)
		}
	}
	if len(imps) > 0 {
		buf.WriteString("import (\n")
		sort.Strings(imps)
		for _, path := range imps {
			// Omit the local package identifier if it matches the package name.
			info := g.imports[path]
			if info.differs {
				fmt.Fprintf(&buf, "\t%s %q\n", info.name, path)
			} else {
				fmt.Fprintf(&buf, "\t%q\n", path)
			}
		}
		buf.WriteString(")\n\n")
	}
	if len(g.anonImports) > 0 {
		buf.WriteString("import (\n")
		anonImps := make([]string, 0, len(g.anonImports))
		for path := range g.anonImports {
			anonImps = append(anonImps, path)
		}
		sort.Strings(anonImps)

		for _, path := range anonImps {
			fmt.Fprintf(&buf, "\t_ %s\n", path)
		}
		buf.WriteString(")\n\n")
	}
	buf.Write(g.buf.Bytes())
	return buf.Bytes()
}

// clone returns a deep copy of v.
func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	x := new(T)
	switch cloned := any(x).(type) {
	case *[]*ast.Ident:
		v := any(v).(*[]*ast.Ident)
		*cloned = make([]*ast.Ident, len(*v))
		for i, ident := range *v {
			(*cloned)[i] = clone(ident)
		}
	case *ast.Ident:
		cloned.Name = any(v).(*ast.Ident).Name
	case *ast.StarExpr:
		switch x := any(v).(*ast.StarExpr).X.(type) {
		case *ast.Ident:
			cloned.X = clone(x)
		default:
			cloned.X = any(v).(*ast.StarExpr).X
		}
	case *ast.Comment:
		cloned.Text = any(v).(*ast.Comment).Text
	case *ast.CommentGroup:
		v := any(v).(*ast.CommentGroup)
		if v.List == nil {
			break
		}
		cloned.List = make([]*ast.Comment, len(v.List))
		for i, c := range v.List {
			cloned.List[i] = clone(c)
		}
	case *ast.BasicLit:
		cloned.Value = any(v).(*ast.BasicLit).Value
		cloned.Kind = any(v).(*ast.BasicLit).Kind
	case *[]*ast.Field:
		v := any(v).(*[]*ast.Field)
		*cloned = make([]*ast.Field, len(*v))
		for i, field := range *v {
			(*cloned)[i] = clone(field)
		}
	case *ast.Field:
		v := any(v).(*ast.Field)
		cloned.Doc = clone(v.Doc)
		cloned.Comment = clone(v.Comment)
		cloned.Tag = clone(v.Tag)
		cloned.Names = *clone(&v.Names)
		cloned.Type = *clone(&v.Type)
	case *ast.FieldList:
		v := any(v).(*ast.FieldList)
		cloned.List = *clone(&v.List)
	case *ast.TypeSpec:
		v := any(v).(*ast.TypeSpec)
		cloned.Doc = clone(v.Doc)
		cloned.Comment = clone(v.Comment)
		cloned.Name = clone(v.Name)
		cloned.TypeParams = clone(v.TypeParams)
		cloned.Type = *clone(&v.Type)
	default:
		*x = *v
	}
	return x
}
