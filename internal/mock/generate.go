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
	// output to. The suffix will be "mock_gen.go".
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

// Generate generates a code file for each package matching the given patterns.
// The code file will contain mock implementations for each struct type in any
// file in the package that has the mockstub build tag.  As a consequence, the
// generated files will not be included in the package's build when using the
// mockstub build tag.  An implementation for each method of each interface
// type that the struct type embeds will be generated, unless an implementation
// already exists elsewhere in the package.
// The generated files will be named mock_gen.go, with an optional prefix.
// The generated files will also include a go:generate comment that can be used
// to regenerate the file.
func Generate(ctx context.Context, patterns []string, opts GenerateOptions) ([]GenerateResult, []error) {
	tags := "-tags=mockstub"
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
		outputFile := opts.PrefixOutputFile + "mock_gen"
		if strings.HasSuffix(pkg.Name, "_test") {
			outputFile += "_test"
		}
		outputFile += ".go"
		generated[i].OutputPath = filepath.Join(outDir, outputFile)
		g := newGen(pkg)
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
			if comment.Text == "// +build mockstub" {
				return true
			}
			if strings.HasPrefix(comment.Text, "//go:build mockstub") {
				return true
			}
		}
	}
	return false
}

func generateMocks(g *gen, pkg *packages.Package) (errs []error) {
	for _, syntax := range pkg.Syntax {
		if !isMockStub(syntax) {
			continue
		}
		// filename := pkg.Fset.File(syntax.Pos()).Name()
		// ast.Print(pkg.Fset, syntax)

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

		methDecl := makeMockMethod(g, structName, methodName, sig)
		expDecl := makeExpectFunc(g, "Expect", structName, methodName, sig)
		manyDecl := makeExpectFunc(g, "ExpectMany", structName, methodName, sig)

		// Generate the source code for the function
		if err := g.addDecl(expDecl.Name, expDecl); err != nil {
			return err
		}
		if err := g.addDecl(manyDecl.Name, manyDecl); err != nil {
			return err
		}
		if err := g.addDecl(methDecl.Name, methDecl); err != nil {
			return err
		}
	}

	return nil
}

func makeMockMethod(g *gen, structName, methodName string, sig *types.Signature) (methDecl *ast.FuncDecl) {
	// Start building the function declaration
	methDecl = &ast.FuncDecl{
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

	methDecl.Type.Params = fieldList("v", sig.Variadic(), sig.Params())
	methDecl.Type.Results = fieldList("", false, sig.Results())

	// Create a function body (block statement)
	methDecl.Body = &ast.BlockStmt{List: []ast.Stmt{}}
	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(g.resolveImportName("mock", "github.com/Versent/go-mock")),
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

	return
}

func makeExpectFunc(g *gen, funcName, structName, methodName string, sig *types.Signature) (funcDecl *ast.FuncDecl) {
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
				X:   ast.NewIdent(g.resolveImportName("mock", "github.com/Versent/go-mock")),
				Sel: ast.NewIdent("CallCount"),
			},
		})
	}
	funcDecl = &ast.FuncDecl{
		Name: ast.NewIdent(funcName + methodName),
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
							X:   ast.NewIdent(g.resolveImportName("mock", "github.com/Versent/go-mock")),
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
	return
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
}

func newGen(pkg *packages.Package) *gen {
	return &gen{
		pkg:         pkg,
		anonImports: make(map[string]bool),
		imports:     make(map[string]importInfo),
		values:      make(map[ast.Expr]string),
	}
}

func (g *gen) addDecl(name fmt.Stringer, decl ast.Decl) error {
	genDecl, ok := decl.(*ast.GenDecl)
	if ok && genDecl.Tok == token.IMPORT {
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
	buf.WriteString("// Code generated by mockgen. DO NOT EDIT.\n\n")
	buf.WriteString("//go:generate go run -mod=mod github.com/Versent/go-mock/cmd/mockgen" + tags + "\n")
	buf.WriteString("//+build !mockstub\n\n")
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
