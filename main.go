package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func fieldTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.ArrayType:
		return "[]" + fieldTypeString(t.Elt)
	case *ast.StarExpr:
		return "*" + fieldTypeString(t.X)
	case *ast.SelectorExpr:
		return fieldTypeString(t.X) + "." + t.Sel.Name
	default:
		return fmt.Sprintf("%T", t)
	}
}

func generateBuilderForStruct(filePath, structName, packageName string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	var fields []string
	var fieldTypes []string

	ast.Inspect(node, func(n ast.Node) bool {
		t, ok := n.(*ast.TypeSpec)
		if ok && t.Name.Name == structName {
			s, ok := t.Type.(*ast.StructType)
			if ok {
				for _, field := range s.Fields.List {
					if field.Names != nil {
						for _, name := range field.Names {
							// Only consider public fields
							if name.IsExported() {
								fields = append(fields, name.Name)
								fieldTypes = append(fieldTypes, fieldTypeString(field.Type))
							}
						}
					}
				}
			}
		}
		return true
	})

	if len(fields) == 0 {
		return "", fmt.Errorf("no public fields found for struct %s", structName)
	}

	var builderCode strings.Builder
	builderCode.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	builderName := structName + "Builder"

	builderCode.WriteString(fmt.Sprintf("type %s struct {\n", builderName))
	builderCode.WriteString(fmt.Sprintf("    target %s\n", structName))
	builderCode.WriteString("}\n\n")

	builderCode.WriteString(fmt.Sprintf("func New%s() *%s {\n", builderName, builderName))
	builderCode.WriteString(fmt.Sprintf("    return &%s{}\n", builderName))
	builderCode.WriteString("}\n\n")

	for i, field := range fields {
		builderCode.WriteString(fmt.Sprintf("func (b *%s) With%s(value %s) *%s {\n", builderName, field, fieldTypes[i], builderName))
		builderCode.WriteString(fmt.Sprintf("    b.target.%s = value\n", field))
		builderCode.WriteString(fmt.Sprintf("    return b\n"))
		builderCode.WriteString("}\n\n")
	}

	builderCode.WriteString(fmt.Sprintf("func (b *%s) Build() %s {\n", builderName, structName))
	builderCode.WriteString(fmt.Sprintf("    return b.target\n"))
	builderCode.WriteString("}\n")

	return builderCode.String(), nil
}

func determineOutputFileName(inputFileName string) string {
	base := strings.TrimSuffix(inputFileName, filepath.Ext(inputFileName))
	builderFileName := base + "_builder.go"
	return builderFileName
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go-builder-gen <file_path> <struct_name> <output_path> <package_name>")
		return
	}

	filePath := os.Args[1]
	structName := os.Args[2]
	outputPath := os.Args[3]
	packageName := os.Args[4]

	code, err := generateBuilderForStruct(filePath, structName, packageName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return
	}

	// Check if outputPath is a directory
	info, err := os.Stat(outputPath)
	if err == nil && info.IsDir() {
		outputFileName := determineOutputFileName(filepath.Base(filePath))
		outputPath = filepath.Join(outputPath, outputFileName)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directories: %s\n", err)
		return
	}

	if err := os.WriteFile(outputPath, []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to file: %s\n", err)
		return
	}

	fmt.Printf("Builder generated and saved to: %s\n", outputPath)
}
