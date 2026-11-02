package dispatch

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// ExtractASTOutline parses the source code in content and generates a compact AST outline
// preserving package declaration, imports, types, structs, interfaces, and function/method
// signatures while stripping function bodies. Currently supports Go (.go) via stdlib
// go/parser and go/ast, and TypeScript/JavaScript (.ts, .tsx, .js) via declaration extraction.
// Returns an error if the language is unsupported or parsing fails.
func ExtractASTOutline(filePath string, content []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return extractGoASTOutline(filePath, content)
	case ".ts", ".tsx", ".js", ".jsx":
		return extractTypeScriptOutline(content)
	default:
		return "", fmt.Errorf("unsupported file extension for AST outline: %s", ext)
	}
}

func extractGoASTOutline(filePath string, content []byte) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, 0)
	if err != nil {
		return "", fmt.Errorf("failed to parse Go source %s: %w", filePath, err)
	}

	// Strip function and method bodies
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			fn.Body = nil
		}
	}

	// Remove comments to save tokens and avoid orphaned comment placement
	file.Comments = nil

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return "", fmt.Errorf("failed to format Go AST outline %s: %w", filePath, err)
	}

	return strings.TrimSpace(buf.String()), nil
}

var tsOutlineDeclRegex = regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:async\s+)?(?:abstract\s+)?(?:declare\s+)?(?:function|class|interface|type|enum|const|let|var)\b`)

func extractTypeScriptOutline(content []byte) (string, error) {
	lines := strings.Split(string(content), "\n")
	var outlineLines []string
	inBlock := false
	blockBraces := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		if strings.HasPrefix(trimmed, "import ") {
			outlineLines = append(outlineLines, line)
			continue
		}

		if !inBlock {
			if tsOutlineDeclRegex.MatchString(trimmed) {
				openCount := strings.Count(line, "{")
				closeCount := strings.Count(line, "}")
				if openCount > closeCount {
					inBlock = true
					blockBraces = openCount - closeCount
					outlineLines = append(outlineLines, line)
				} else {
					outlineLines = append(outlineLines, line)
				}
			}
		} else {
			outlineLines = append(outlineLines, line)
			blockBraces += strings.Count(line, "{") - strings.Count(line, "}")
			if blockBraces <= 0 {
				inBlock = false
				blockBraces = 0
			}
		}
	}

	if len(outlineLines) == 0 {
		return "", fmt.Errorf("no TypeScript declarations found for outline")
	}

	return strings.TrimSpace(strings.Join(outlineLines, "\n")), nil
}
