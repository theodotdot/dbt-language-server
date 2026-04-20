package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/analysis/diagnostics"
	"github.com/j-clemons/dbt-language-server/analysis/fusion"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/rpc"
	"github.com/j-clemons/dbt-language-server/util"
	"github.com/j-clemons/dbt-language-server/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "upgrade" {
		cmd := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/j-clemons/dbt-language-server/main/install | bash")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal("Upgrade failed:", err)
		}
		os.Exit(0)
	}

	showVersion := flag.BoolP("version", "v", false, "Print version")
	debug := flag.BoolP("debug", "d", false, "Enable debug logging to log.txt")

	fusion := flag.StringP("fusion", "f", "", "Enable dbt fusion features. Provide an absolute path if default value is not dbt")
	flag.Lookup("fusion").NoOptDefVal = "dbt"

	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	var logger *log.Logger
	if *debug {
		logger = util.GetLogger("log.txt")
	} else {
		logger = log.New(io.Discard, "", 0)
	}

	state := analysis.NewState()
	state.FusionEnabled = false
	state.FusionPath = *fusion

	if *fusion != "" {
		go func() {
			fusionValidation, err := util.ValidateFusion(*fusion)
			if err != nil {
				logger.Println(err)
			}
			state.SetFusionEnabled(fusionValidation)
		}()
	}

	writer := os.Stdout

	var diagEngine *diagnostics.Engine
	if *fusion == "" {
		diagEngine = diagnostics.NewEngine(writer, &state,
			diagnostics.CheckRefs,
			diagnostics.CheckSources,
			diagnostics.CheckVars,
			diagnostics.CheckMacros,
			diagnostics.CheckJinja,
			diagnostics.CheckColumns,
		)
	}

	logger.Println("dbt Language Server Started!")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(rpc.Split)

	for scanner.Scan() {
		msg := scanner.Bytes()
		method, contents, err := rpc.DecodeMessage(msg)
		if err != nil {
			logger.Printf("Got an error: %s", err)
		}
		handleMessage(logger, writer, &state, diagEngine, method, contents)
	}
}

func handleMessage(logger *log.Logger, writer io.Writer, state *analysis.State, diagEngine *diagnostics.Engine, method string, contents []byte) {
	logger.Printf("Received msg with method: %s", method)

	switch method {
	case "initialize":
		var request lsp.InitializeRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("Could not parse: %s", err)
		}

		logger.Printf("Connected to: %s %s",
			request.Params.ClientInfo.Name,
			request.Params.ClientInfo.Version)

		msg := lsp.NewInitializeResponse(request.ID)
		util.WriteResponse(writer, msg)

		logger.Print("Sent the reply")
	case "textDocument/didOpen":
		var request lsp.DidOpenTextDocumentNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didOpen: %s", err)
			return
		}

		state.OpenDocument(request.Params.TextDocument.URI, request.Params.TextDocument.Text)
		logger.Printf("Opened: %s", request.Params.TextDocument.URI)

		fusion.FusionCompile(state, request.Params.TextDocument.URI, logger, writer)

		if diagEngine != nil && strings.HasSuffix(request.Params.TextDocument.URI, ".sql") {
			diagEngine.RunImmediate(request.Params.TextDocument.URI)
		}
	case "textDocument/didSave":
		logger.Print("textDocument/didSave")
		var request lsp.DidSaveTextDocumentNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didSave: %s", err)
			return
		}

		logger.Printf("Saved: %s", request.Params.TextDocument.URI)
		state.SaveDocument(request.Params.TextDocument.URI)

		fusion.FusionCompile(state, request.Params.TextDocument.URI, logger, writer)

		if diagEngine != nil && strings.HasSuffix(request.Params.TextDocument.URI, ".sql") {
			diagEngine.RunImmediate(request.Params.TextDocument.URI)
		}
	case "textDocument/didChange":
		var request lsp.TextDocumentDidChangeNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didChange: %s", err)
			return
		}

		logger.Printf("Changed: %s", request.Params.TextDocument.URI)
		state.UpdateDocumentIncremental(request.Params.TextDocument.URI, request.Params.ContentChanges)

		if diagEngine != nil && strings.HasSuffix(request.Params.TextDocument.URI, ".sql") {
			diagEngine.RunDebounced(request.Params.TextDocument.URI)
		}
	case "textDocument/didClose":
		var request lsp.DidCloseTextDocumentNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didClose: %s", err)
			return
		}
		logger.Printf("Closed: %s", request.Params.TextDocument.URI)
		if diagEngine != nil {
			diagEngine.Clear(request.Params.TextDocument.URI)
		}
		state.CloseDocument(request.Params.TextDocument.URI)
	case "textDocument/hover":
		var request lsp.HoverRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/hover: %s", err)
			return
		}

		response := state.Hover(request.ID, request.Params.TextDocument.URI, request.Params.Position)

		util.WriteResponse(writer, response)
	case "textDocument/definition":
		logger.Print("textDocument/definition")
		var request lsp.DefinitionRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/definition: %s", err)
			return
		}

		response := state.Definition(request.ID, request.Params.TextDocument.URI, request.Params.Position)

		util.WriteResponse(writer, response)
	case "textDocument/completion":
		logger.Print("textDocument/completion")
		var request lsp.CompletionRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/completion: %s", err)
			return
		}

		response := state.TextDocumentCompletion(request.ID, request.Params.TextDocument.URI, request.Params.Position)

		util.WriteResponse(writer, response)
	case "textDocument/codeAction":
		var request lsp.CodeActionRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/codeAction: %s", err)
			return
		}

		response := state.TextDocumentCodeAction(request.ID, request.Params.TextDocument.URI, request.Params.Range)
		util.WriteResponse(writer, response)
	case "textDocument/prepareRename":
		var request lsp.PrepareRenameRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/prepareRename: %s", err)
			return
		}

		response := state.PrepareRename(request.ID, request.Params.TextDocument.URI, request.Params.Position)
		util.WriteResponse(writer, response)
	case "textDocument/rename":
		var request lsp.RenameRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/rename: %s", err)
			return
		}

		response := state.Rename(request.ID, request.Params.TextDocument.URI, request.Params.Position, request.Params.NewName)
		util.WriteResponse(writer, response)
	case "textDocument/semanticTokens/full":
		logger.Print("textDocument/semanticTokens/full")
		var request lsp.SemanticTokensRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/semanticTokens/full: %s", err)
			return
		}

		response := state.SemanticTokensFull(request.ID, request.Params.TextDocument.URI)

		util.WriteResponse(writer, response)
	case "shutdown":
		var request lsp.Request
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("shutdown: %s", err)
			return
		}

		logger.Print("Received shutdown request")
		response := lsp.Response{
			RPC: "2.0",
			ID:  &request.ID,
		}
		util.WriteResponse(writer, response)
	case "exit":
		logger.Print("Received exit notification")
		os.Exit(0)
	case "workspace/executeCommand":
		logger.Print("workspace/executeCommand")
		var request lsp.ExecuteCommandRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("workspace/executeCommand: %s", err)
			return
		}

		if request.Params.Command == "dbt.goToSchema" {
			// Parse arguments to get URI and position
			if len(request.Params.Arguments) >= 1 {
				argMap, ok := request.Params.Arguments[0].(map[string]interface{})
				if ok {
					uri, _ := argMap["uri"].(string)
					positionMap, _ := argMap["position"].(map[string]interface{})
					line, _ := positionMap["line"].(float64)
					character, _ := positionMap["character"].(float64)

					position := lsp.Position{
						Line:      int(line),
						Character: int(character),
					}

					response := state.GoToSchema(request.ID, uri, position)
					util.WriteResponse(writer, response)
				}
			}
		}
	}
}
