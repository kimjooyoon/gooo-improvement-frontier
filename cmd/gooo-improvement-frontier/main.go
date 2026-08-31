package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/kimjooyoon/gooo-improvement-frontier/internal/frontier"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "compile":
		return compile(args[1:], stdout, stderr)
	case "evaluate":
		return evaluate(args[1:], stdout, stderr)
	case "conformance":
		return conformance(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "gooo-improvement-frontier/v0.1.0")
		return 0
	default:
		usage(stderr)
		return 2
	}
}

func compile(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sourcePath := flags.String("source", "examples/improvement-frontier.gooo", "Gooo source path")
	contractPath := flags.String("contract", "contracts/improvement-frontier-denominator-v1.json", "fixed denominator path")
	outputIR := flags.String("output-ir", "", "caller-owned absolute semantic IR output")
	outputGo := flags.String("output-go", "", "caller-owned absolute generated Go output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !absolute(*outputIR) || !absolute(*outputGo) {
		fmt.Fprintln(stderr, "compile requires absolute -output-ir and -output-go paths")
		return 2
	}
	meta, err := compileMeta(*sourcePath, *contractPath)
	if err != nil {
		fmt.Fprintf(stderr, "compile: %v\n", err)
		return 1
	}
	if err := writeOutput(*outputIR, meta.irRaw); err != nil {
		fmt.Fprintf(stderr, "write semantic IR: %v\n", err)
		return 1
	}
	if err := writeOutput(*outputGo, meta.generatedGo); err != nil {
		fmt.Fprintf(stderr, "write generated Go: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "compiled source=%s semantic_ir=%s generated_go=%s\n", *sourcePath, *outputIR, *outputGo)
	return 0
}

func evaluate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("evaluate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	sourcePath := flags.String("source", "examples/improvement-frontier.gooo", "Gooo source path")
	contractPath := flags.String("contract", "contracts/improvement-frontier-denominator-v1.json", "fixed denominator path")
	fixturePath := flags.String("fixture", "", "causal graph fixture JSON")
	outputDir := flags.String("output-dir", "", "caller-owned absolute output directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *fixturePath == "" || !absolute(*outputDir) {
		fmt.Fprintln(stderr, "evaluate requires -fixture and absolute -output-dir")
		return 2
	}
	meta, err := compileMeta(*sourcePath, *contractPath)
	if err != nil {
		fmt.Fprintf(stderr, "compile authority chain: %v\n", err)
		return 1
	}
	fixture, fixtureRaw, err := frontier.LoadFixture(*fixturePath)
	if err != nil {
		fmt.Fprintf(stderr, "read fixture: %v\n", err)
		return 1
	}
	evaluatorPath := filepath.Join(*root, "internal", "frontier", "evaluate.go")
	evaluatorRaw, err := os.ReadFile(evaluatorPath)
	if err != nil {
		fmt.Fprintf(stderr, "read evaluator: %v\n", err)
		return 1
	}
	evaluation, err := frontier.EvaluateFixture(fixture, frontier.DigestBytes(fixtureRaw),
		frontier.ArtifactBinding{Path: *sourcePath, Digest: meta.sourceDigest},
		frontier.ArtifactBinding{Path: "internal/generated/semantic-ir.json", Digest: frontier.DigestBytes(meta.irRaw)},
		frontier.ArtifactBinding{Path: "internal/generated/semantic.gooo.go", Digest: frontier.DigestBytes(meta.generatedGo)},
		frontier.ArtifactBinding{Path: evaluatorPath, Digest: frontier.DigestBytes(evaluatorRaw)},
		frontier.ArtifactBinding{Path: *contractPath, Digest: meta.contractDigest},
	)
	if err != nil {
		fmt.Fprintf(stderr, "evaluate: %v\n", err)
		return 1
	}
	if err := frontier.WriteEvaluation(*outputDir, evaluation); err != nil {
		fmt.Fprintf(stderr, "write evaluation: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s state=%s output=%s\n", fixture.CaseID, evaluation.Plan.State, *outputDir)
	return 0
}

func conformance(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	sourcePath := flags.String("source", "examples/improvement-frontier.gooo", "Gooo source path")
	contractPath := flags.String("contract", "contracts/improvement-frontier-denominator-v1.json", "fixed denominator path")
	corpusPath := flags.String("corpus", "examples/canonical-corpus.json", "exact canonical corpus")
	outputDir := flags.String("output-dir", "", "caller-owned absolute output directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !absolute(*outputDir) {
		fmt.Fprintln(stderr, "conformance requires absolute -output-dir")
		return 2
	}
	meta, err := compileMeta(*sourcePath, *contractPath)
	if err != nil {
		fmt.Fprintf(stderr, "compile authority chain: %v\n", err)
		return 1
	}
	corpusRaw, err := os.ReadFile(*corpusPath)
	if err != nil {
		fmt.Fprintf(stderr, "read corpus: %v\n", err)
		return 1
	}
	var corpus frontier.Corpus
	decoder := json.NewDecoder(strings.NewReader(string(corpusRaw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		fmt.Fprintf(stderr, "decode corpus: %v\n", err)
		return 1
	}
	if corpus.Schema != frontier.CorpusSchema || corpus.CorpusID == "" || corpus.DenominatorID != "improvement-frontier-v1" || corpus.FixedDenominator != 12 || len(corpus.Cases) != 6 {
		fmt.Fprintln(stderr, "canonical corpus header or exact case count is invalid")
		return 1
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "create output: %v\n", err)
		return 1
	}
	index := conformanceIndex{Schema: "gooo/improvement-frontier/conformance/v1", CorpusID: corpus.CorpusID, DenominatorID: corpus.DenominatorID, FixedDenominator: corpus.FixedDenominator, Cases: make([]conformanceCase, 0, len(corpus.Cases))}
	for _, corpusCase := range corpus.Cases {
		fixturePath := filepath.Join(*root, corpusCase.Path)
		fixture, fixtureRaw, loadErr := frontier.LoadFixture(fixturePath)
		if loadErr != nil {
			fmt.Fprintf(stderr, "load %s: %v\n", corpusCase.CaseID, loadErr)
			return 1
		}
		if fixture.CaseID != corpusCase.CaseID {
			fmt.Fprintf(stderr, "%s: fixture ID mismatch\n", corpusCase.CaseID)
			return 1
		}
		if fixture.Expected.State != corpusCase.State {
			fmt.Fprintf(stderr, "%s: corpus state does not match fixture expectation\n", corpusCase.CaseID)
			return 1
		}
		evaluatorRaw, readErr := os.ReadFile(filepath.Join(*root, "internal", "frontier", "evaluate.go"))
		if readErr != nil {
			fmt.Fprintf(stderr, "read evaluator: %v\n", readErr)
			return 1
		}
		evaluation, evalErr := frontier.EvaluateFixture(fixture, frontier.DigestBytes(fixtureRaw),
			frontier.ArtifactBinding{Path: *sourcePath, Digest: meta.sourceDigest},
			frontier.ArtifactBinding{Path: "internal/generated/semantic-ir.json", Digest: frontier.DigestBytes(meta.irRaw)},
			frontier.ArtifactBinding{Path: "internal/generated/semantic.gooo.go", Digest: frontier.DigestBytes(meta.generatedGo)},
			frontier.ArtifactBinding{Path: "internal/frontier/evaluate.go", Digest: frontier.DigestBytes(evaluatorRaw)},
			frontier.ArtifactBinding{Path: *contractPath, Digest: meta.contractDigest},
		)
		if evalErr != nil {
			fmt.Fprintf(stderr, "evaluate %s: %v\n", corpusCase.CaseID, evalErr)
			return 1
		}
		if err := assertExpectations(fixture, evaluation); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", corpusCase.CaseID, err)
			return 1
		}
		caseDir := filepath.Join(*outputDir, corpusCase.CaseID)
		if err := frontier.WriteEvaluation(caseDir, evaluation); err != nil {
			fmt.Fprintf(stderr, "write %s: %v\n", corpusCase.CaseID, err)
			return 1
		}
		index.Cases = append(index.Cases, conformanceCase{Ordinal: corpusCase.Ordinal, CaseID: corpusCase.CaseID, State: evaluation.Plan.State, ParallelBatches: len(evaluation.Plan.ParallelBatches), BlockedFrontiers: len(evaluation.Plan.BlockedFrontiers), RefutedFrontiers: len(evaluation.Plan.RefutedFrontiers)})
		fmt.Fprintf(stdout, "%s state=%s\n", corpusCase.CaseID, evaluation.Plan.State)
	}
	index.States = map[string]int{string(frontier.StateClosed): 0, string(frontier.StateUnknown): 0, string(frontier.StateRefuted): 0}
	for _, item := range index.Cases {
		index.States[string(item.State)]++
	}
	return writeJSON(filepath.Join(*outputDir, "conformance-index.json"), index, stderr)
}

type compiledMeta struct {
	sourceDigest   string
	contractDigest string
	irRaw          []byte
	generatedGo    []byte
}

func compileMeta(sourcePath, contractPath string) (compiledMeta, error) {
	sourceRaw, err := os.ReadFile(sourcePath)
	if err != nil {
		return compiledMeta{}, err
	}
	contract, contractRaw, err := frontier.LoadContract(contractPath)
	if err != nil {
		return compiledMeta{}, err
	}
	contractDigest := frontier.DigestBytes(contractRaw)
	ir, err := frontier.CompileSource(sourcePath, sourceRaw, contract, contractDigest)
	if err != nil {
		return compiledMeta{}, err
	}
	irRaw, err := frontier.SemanticIRBytes(ir)
	if err != nil {
		return compiledMeta{}, err
	}
	return compiledMeta{sourceDigest: frontier.DigestBytes(sourceRaw), contractDigest: contractDigest, irRaw: irRaw, generatedGo: frontier.GenerateGo(ir, frontier.DigestBytes(irRaw))}, nil
}

type conformanceIndex struct {
	Schema           string            `json:"schema"`
	CorpusID         string            `json:"corpus_id"`
	DenominatorID    string            `json:"denominator_id"`
	FixedDenominator int               `json:"fixed_denominator"`
	Cases            []conformanceCase `json:"cases"`
	States           map[string]int    `json:"states"`
}

type conformanceCase struct {
	Ordinal          int            `json:"ordinal"`
	CaseID           string         `json:"case_id"`
	State            frontier.State `json:"state"`
	ParallelBatches  int            `json:"parallel_batches"`
	BlockedFrontiers int            `json:"blocked_frontiers"`
	RefutedFrontiers int            `json:"refuted_frontiers"`
}

func assertExpectations(fixture frontier.Fixture, evaluation frontier.Evaluation) error {
	expected := fixture.Expected
	if evaluation.Plan.State != expected.State {
		return fmt.Errorf("expected state %s, got %s", expected.State, evaluation.Plan.State)
	}
	if len(expected.ParallelBatches) > 0 && !reflect.DeepEqual(expected.ParallelBatches, evaluation.Plan.ParallelBatches) {
		return fmt.Errorf("parallel batches differ: expected %v, got %v", expected.ParallelBatches, evaluation.Plan.ParallelBatches)
	}
	if !reflect.DeepEqual(expected.BlockedFrontierRoots, frontierRoots(evaluation.Plan.BlockedFrontiers)) {
		return fmt.Errorf("blocked frontier roots differ: expected %v, got %v", expected.BlockedFrontierRoots, frontierRoots(evaluation.Plan.BlockedFrontiers))
	}
	if !reflect.DeepEqual(expected.RefutedFrontierRoots, frontierRoots(evaluation.Plan.RefutedFrontiers)) {
		return fmt.Errorf("refuted frontier roots differ: expected %v, got %v", expected.RefutedFrontierRoots, frontierRoots(evaluation.Plan.RefutedFrontiers))
	}
	relations := make([]string, 0, len(evaluation.Plan.SerialCuts))
	for _, cut := range evaluation.Plan.SerialCuts {
		relations = append(relations, cut.Relation)
	}
	for _, required := range expected.SerialCutRelations {
		found := false
		for _, actual := range relations {
			if actual == required {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing serial cut relation %s", required)
		}
	}
	valid, invalid := make([]string, 0), make([]string, 0)
	for _, evidence := range evaluation.Plan.Evidence {
		if evidence.Valid {
			valid = append(valid, evidence.EvidenceID)
		} else {
			invalid = append(invalid, evidence.EvidenceID)
		}
	}
	if !reflect.DeepEqual(expected.ValidEvidenceIDs, valid) || !reflect.DeepEqual(expected.InvalidEvidenceIDs, invalid) {
		return fmt.Errorf("evidence validity differs: expected valid=%v invalid=%v, got valid=%v invalid=%v", expected.ValidEvidenceIDs, expected.InvalidEvidenceIDs, valid, invalid)
	}
	return nil
}

func frontierRoots(frontiers []frontier.Frontier) []string {
	roots := make([]string, 0, len(frontiers))
	for _, item := range frontiers {
		roots = append(roots, item.OperationID)
	}
	sort.Strings(roots)
	return roots
}

func writeOutput(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func writeJSON(path string, value any, stderr io.Writer) int {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode JSON: %v\n", err)
		return 1
	}
	if err := writeOutput(path, append(raw, '\n')); err != nil {
		fmt.Fprintf(stderr, "write JSON: %v\n", err)
		return 1
	}
	return 0
}

func absolute(path string) bool { return path != "" && filepath.IsAbs(path) }

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: gooo-improvement-frontier <compile|evaluate|conformance|version>")
}
