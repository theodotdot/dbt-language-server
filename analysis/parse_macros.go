package analysis

import (
	"os"
	"strings"

	"github.com/j-clemons/dbt-language-server/analysis/jinja"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/util"
)

type MacroArg struct {
	Name    string
	Default string
}

type Macro struct {
	Name        string
	ProjectName Package
	Description string
	Arguments   []MacroArg
	URI         string
	Range       lsp.Range
}

func parseMacroArgs(argsStr string) []MacroArg {
	argsStr = strings.TrimSpace(argsStr)
	if argsStr == "" {
		return nil
	}

	parts := strings.Split(argsStr, ",")
	args := make([]MacroArg, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if eqIdx := strings.Index(part, "="); eqIdx != -1 {
			name := strings.TrimSpace(part[:eqIdx])
			def := strings.TrimSpace(part[eqIdx+1:])
			args = append(args, MacroArg{Name: name, Default: def})
		} else {
			args = append(args, MacroArg{Name: part})
		}
	}
	return args
}

func formatMacroSignature(name string, args []MacroArg) string {
	var b strings.Builder
	b.WriteString(name)
	b.WriteByte('(')
	for i, arg := range args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(arg.Name)
		if arg.Default != "" {
			b.WriteByte('=')
			b.WriteString(arg.Default)
		}
	}
	b.WriteByte(')')
	return b.String()
}

func getMacrosFromFile(fileStr string, fileUri string, dbtProjectYaml DbtProjectYaml) []Macro {
	macroMatches := jinja.MacroDefRegex.FindAllStringSubmatchIndex(fileStr, -1)

	macros := []Macro{}
	for _, m := range macroMatches {
		macroName := fileStr[m[2]:m[3]]
		argsStr := fileStr[m[4]:m[5]]
		args := parseMacroArgs(argsStr)

		startLine, startCol := util.GetLineAndColumn(fileStr, m[2])
		endLine, endCol := util.GetLineAndColumn(fileStr, m[5]+1) // end after closing paren
		macros = append(
			macros,
			Macro{
				Name:        macroName,
				ProjectName: Package(dbtProjectYaml.ProjectName.Value),
				Description: formatMacroSignature(macroName, args),
				Arguments:   args,
				URI:         fileUri,
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      startLine,
						Character: startCol,
					},
					End: lsp.Position{
						Line:      endLine,
						Character: endCol,
					},
				},
			},
		)
	}
	return macros
}

func parseMacros(projectRoot string, dbtProjectYaml DbtProjectYaml) ([]Macro, error) {
	macros := []Macro{}

	var err error
	for _, p := range dbtProjectYaml.MacroPaths.Value {
		path := projectRoot + "/" + p
		_, err = os.ReadDir(path)
		if err != nil {
			continue
		}
		macroFilePaths, err := util.WalkFilepath(path, ".sql")
		if err != nil {
			continue
		}

		for _, macroFilePath := range macroFilePaths {
			fileContents, err := util.ReadFileContents(macroFilePath)
			if err != nil {
				continue
			}
			macros = append(
				macros,
				getMacrosFromFile(fileContents, macroFilePath, dbtProjectYaml)...,
			)
		}
	}
	return macros, err
}
