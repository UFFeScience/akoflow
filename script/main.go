package main

import (
	"flag"
	"github.com/ovvesley/scik8sflow/script/parser_pegasus_workflow"
)

func main() {
	parserPegasusInputFile := flag.String("parser-pegasus-input-file", "", "parser-pegasus-input-file")
	parserPegasusOutputFile := flag.String("parser-pegasus-output-file", "", "--parser-pegasus-output-file")

	flag.Parse()

	if validateParserPegasusInputFile(*parserPegasusInputFile) && validateParserPegasusOutputFile(*parserPegasusOutputFile) {
		parserPegasus(*parserPegasusInputFile, *parserPegasusOutputFile)
		return
	}
}

func validateParserPegasusInputFile(parserPegasusInputFile string) bool {
	if parserPegasusInputFile == "" {
		return false
	}

	return true
}

func parserPegasus(parserPegasusInputFile string, parserPegasusOutputFile string) {
	parserPegasusWorkflow := parser_pegasus_workflow.NewService()
	parserPegasusWorkflow.
		SetInputFile(parserPegasusInputFile).
		SetOutputFile(parserPegasusOutputFile).
		Parser()

}

func validateParserPegasusOutputFile(parserPegasusOutputFile string) bool {
	if parserPegasusOutputFile == "" {
		return false
	}

	return true
}
