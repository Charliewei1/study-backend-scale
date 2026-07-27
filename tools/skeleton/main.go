// Command skeleton は、模範解答のGoファイルから「中身を抜いた骨組み」を作ります。
//
// study スクリプトから呼ばれる裏方なので、学習者が直接触る必要はありません。
//
//	skeleton stub  <new.go>            新しく追加されたファイル全体を骨組みにする
//	skeleton merge <old.go> <new.go>   前日のファイルへ、その日増えた宣言だけを足す
//
// 関数の中身は panic("TODO: ...") へ置き換えます。
// doc コメント・型・定数・struct tag はそのまま残すので、
// 「何を作るか」は読めて「どう作るか」だけが空白になります。
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "skeleton:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: skeleton stub <new.go> | skeleton merge <old.go> <new.go>")
	}
	switch args[0] {
	case "stub":
		if len(args) != 2 {
			return fmt.Errorf("usage: skeleton stub <new.go>")
		}
		src, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		out, err := stub(filepath.Base(args[1]), src)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(out)
		return err

	case "merge":
		if len(args) != 3 {
			return fmt.Errorf("usage: skeleton merge <old.go> <new.go>")
		}
		oldSrc, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		newSrc, err := os.ReadFile(args[2])
		if err != nil {
			return err
		}
		out, err := merge(filepath.Base(args[1]), oldSrc, filepath.Base(args[2]), newSrc)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(out)
		return err

	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

// ---------------------------------------------------------------- stub

// stub はファイル内のすべての関数の中身を panic へ置き換えます。
// 使われなくなった import も取り除きます。
func stub(name string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var cuts []replacement
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		cuts = append(cuts, replacement{
			from: fset.Position(fn.Body.Lbrace).Offset,
			to:   fset.Position(fn.Body.Rbrace).Offset + 1,
			text: stubBody(funcKey(fn)),
		})
	}

	out := applyReplacements(src, cuts)
	out, err = dropUnusedImports(name, out)
	if err != nil {
		return nil, err
	}
	return format.Source(out)
}

// ---------------------------------------------------------------- merge

// merge は「前日のファイル」を土台に、その日はじめて登場した宣言だけを足します。
//
// 前日すでに動いていたコードはそのまま残るので、学習者は前日の自分のコードを
// 読みながら、その日の差分だけに集中できます。
func merge(oldName string, oldSrc []byte, newName string, newSrc []byte) ([]byte, error) {
	fset := token.NewFileSet()
	oldFile, err := parser.ParseFile(fset, oldName, oldSrc, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	newFset := token.NewFileSet()
	newFile, err := parser.ParseFile(newFset, newName, newSrc, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	known := declNames(oldFile)

	var additions []string
	for _, decl := range newFile.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			key := funcKey(d)
			if known[key] {
				continue
			}
			// シグネチャと doc コメントは見せ、中身だけ panic にする。
			head := sourceRange(newSrc, newFset, declStart(d), d.Body.Lbrace)
			additions = append(additions, strings.TrimRight(head, " \t\n")+" "+stubBody(key))

		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			// var / const / type は TASKS.md にそのまま載っているので、
			// 隠さずに配ってしまってよい。
			var fresh []ast.Spec
			for _, spec := range d.Specs {
				for _, n := range specNames(spec) {
					if !known[n] {
						fresh = append(fresh, spec)
						break
					}
				}
			}
			if len(fresh) == 0 {
				continue
			}
			if len(fresh) == len(d.Specs) {
				additions = append(additions, sourceRange(newSrc, newFset, declStart(d), d.End()))
				continue
			}
			// 一部だけ新しい場合は、そのspecを var(...) / const(...) で包み直す。
			var block strings.Builder
			fmt.Fprintf(&block, "%s (\n", d.Tok)
			for _, spec := range fresh {
				fmt.Fprintf(&block, "\t%s\n", sourceRange(newSrc, newFset, spec.Pos(), spec.End()))
			}
			block.WriteString(")")
			additions = append(additions, block.String())
		}
	}

	if len(additions) == 0 {
		return format.Source(oldSrc)
	}

	var buf bytes.Buffer
	buf.Write(bytes.TrimRight(oldSrc, "\n"))
	buf.WriteString("\n\n// ===== ここから今日の課題 =====\n")
	for _, addition := range additions {
		buf.WriteString("\n")
		buf.WriteString(addition)
		buf.WriteString("\n")
	}

	// 足したコードが必要とする import を引き継ぎ、余った分は最後に落とす。
	merged, err := unionImports(oldName, buf.Bytes(), newFile.Imports)
	if err != nil {
		return nil, err
	}
	merged, err = dropUnusedImports(oldName, merged)
	if err != nil {
		return nil, err
	}
	return format.Source(merged)
}

// unionImports は、足りない import を既存のimportブロックへ加えます。
func unionImports(name string, src []byte, want []*ast.ImportSpec) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	have := map[string]bool{}
	for _, imp := range file.Imports {
		have[imp.Path.Value] = true
	}

	lines := map[string]bool{}
	var all []string
	add := func(imp *ast.ImportSpec) {
		line := imp.Path.Value
		if imp.Name != nil {
			line = imp.Name.Name + " " + line
		}
		if !lines[line] {
			lines[line] = true
			all = append(all, line)
		}
	}
	missing := false
	for _, imp := range file.Imports {
		add(imp)
	}
	for _, imp := range want {
		if !have[imp.Path.Value] {
			missing = true
		}
		add(imp)
	}
	if !missing {
		return src, nil
	}
	sort.Strings(all)

	var block strings.Builder
	block.WriteString("import (\n")
	for _, line := range all {
		fmt.Fprintf(&block, "\t%s\n", line)
	}
	block.WriteString(")")

	// 既存のimport宣言があれば置き換え、無ければpackage行の直後へ入れる。
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			return applyReplacements(src, []replacement{{
				from: fset.Position(gen.Pos()).Offset,
				to:   fset.Position(gen.End()).Offset,
				text: block.String(),
			}}), nil
		}
	}
	at := fset.Position(file.Name.End()).Offset
	return applyReplacements(src, []replacement{{
		from: at,
		to:   at,
		text: "\n\n" + block.String(),
	}}), nil
}

// ---------------------------------------------------------------- 部品

type replacement struct {
	from int
	to   int
	text string
}

func applyReplacements(src []byte, cuts []replacement) []byte {
	// 後ろから置換すると、前方のオフセットがずれない。
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].from > cuts[j].from })
	out := append([]byte(nil), src...)
	for _, cut := range cuts {
		merged := make([]byte, 0, len(out))
		merged = append(merged, out[:cut.from]...)
		merged = append(merged, cut.text...)
		merged = append(merged, out[cut.to:]...)
		out = merged
	}
	return out
}

func stubBody(key string) string {
	return fmt.Sprintf("{\n\tpanic(\"TODO: %s を実装しよう\")\n}", key)
}

// funcKey はメソッドと関数を同じ土俵で比べるための名前です。
func funcKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return receiverType(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverType(t.X)
	default:
		return "?"
	}
}

func declNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			names[funcKey(d)] = true
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				for _, n := range specNames(spec) {
					names[n] = true
				}
			}
		}
	}
	return names
}

func specNames(spec ast.Spec) []string {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return []string{s.Name.Name}
	case *ast.ValueSpec:
		var names []string
		for _, n := range s.Names {
			names = append(names, n.Name)
		}
		return names
	default:
		return nil
	}
}

// declStart は doc コメントを含めた宣言の開始位置を返します。
func declStart(decl ast.Decl) token.Pos {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Doc != nil {
			return d.Doc.Pos()
		}
		return d.Pos()
	case *ast.GenDecl:
		if d.Doc != nil {
			return d.Doc.Pos()
		}
		return d.Pos()
	default:
		return decl.Pos()
	}
}

func sourceRange(src []byte, fset *token.FileSet, from, to token.Pos) string {
	start := fset.Position(from).Offset
	end := fset.Position(to).Offset
	if end > len(src) {
		end = len(src)
	}
	return string(src[start:end])
}

// dropUnusedImports は、中身を抜いたせいで使われなくなった import を消します。
func dropUnusedImports(name string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	used := map[string]bool{}
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				used[ident.Name] = true
			}
			return true
		})
	}

	var cuts []replacement
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		var dead []ast.Spec
		for _, spec := range gen.Specs {
			imp := spec.(*ast.ImportSpec)
			if !used[importName(imp)] {
				dead = append(dead, spec)
			}
		}
		if len(dead) == 0 {
			continue
		}
		if len(dead) == len(gen.Specs) {
			cuts = append(cuts, replacement{
				from: fset.Position(gen.Pos()).Offset,
				to:   fset.Position(gen.End()).Offset,
			})
			continue
		}
		for _, spec := range dead {
			cuts = append(cuts, replacement{
				from: fset.Position(spec.Pos()).Offset,
				to:   fset.Position(spec.End()).Offset,
			})
		}
	}
	return applyReplacements(src, cuts), nil
}

func importName(imp *ast.ImportSpec) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	path := strings.Trim(imp.Path.Value, `"`)
	return path[strings.LastIndex(path, "/")+1:]
}
