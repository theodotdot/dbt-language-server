package analysis

import (
	"os"

	"github.com/j-clemons/dbt-language-server/analysis/jinja"
	"github.com/j-clemons/dbt-language-server/util"
)

func getDocsFiles(dbtProjectYaml DbtProjectYaml) []string {
	docsFiles := []string{}

	for _, path := range dbtProjectYaml.DocsPaths.Value {
		_, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		docsPaths, err := util.WalkFilepath(path, ".md")
		if err != nil {
			continue
		}
		docsFiles = append(docsFiles, docsPaths...)
	}
	return docsFiles
}

func processDocsFiles(docsFilesUri []string) map[string]string {
	docsMap := make(map[string]string)
	for _, docsFile := range docsFilesUri {
		docsContents, err := util.ReadFileContents(docsFile)
		if err != nil {
			continue
		}
		for name, content := range jinja.ExtractDocsBlocks(docsContents) {
			docsMap[name] = content
		}
	}
	return docsMap
}
