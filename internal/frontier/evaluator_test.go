package frontier

import (
	"path/filepath"
	"testing"
)

func TestNormalGraphUsesParallelBatchesAndSerialCuts(t *testing.T) {
	fixture, raw, err := LoadFixture(filepath.Join("..", "..", "fixtures", "cases", "normal-independent.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateFixture(fixture, DigestBytes(raw), ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Plan.State != StateClosed {
		t.Fatalf("state = %s, want %s", evaluation.Plan.State, StateClosed)
	}
	want := [][]string{{"capability_alpha", "capability_beta", "capability_gamma", "core_dev_receipt", "denominator_migration"}, {"core_main_promotion", "denominator_receipt"}}
	if len(evaluation.Plan.ParallelBatches) != len(want) {
		t.Fatalf("batches = %v, want %v", evaluation.Plan.ParallelBatches, want)
	}
	for index := range want {
		if len(evaluation.Plan.ParallelBatches[index]) != len(want[index]) {
			t.Fatalf("batch %d = %v, want %v", index, evaluation.Plan.ParallelBatches[index], want[index])
		}
		for item := range want[index] {
			if evaluation.Plan.ParallelBatches[index][item] != want[index][item] {
				t.Fatalf("batch %d = %v, want %v", index, evaluation.Plan.ParallelBatches[index], want[index])
			}
		}
	}
	relations := map[string]bool{}
	for _, cut := range evaluation.Plan.SerialCuts {
		relations[cut.Relation] = true
	}
	for _, relation := range []string{"DEPENDENCY_CLOSURE_INTERSECTION", "MUTATION_AUTHORITY_INTERSECTION", "RESOURCE_LOCK_INTERSECTION"} {
		if !relations[relation] {
			t.Fatalf("missing serial relation %s", relation)
		}
	}
}

func TestUnknownGuardianDoesNotStopIndependentProgress(t *testing.T) {
	fixture, raw, err := LoadFixture(filepath.Join("..", "..", "fixtures", "cases", "unknown-guardian-credential.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateFixture(fixture, DigestBytes(raw), ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Plan.State != StateUnknown || len(evaluation.Plan.ParallelBatches) != 1 || evaluation.Plan.ParallelBatches[0][0] != "capability_alpha" {
		t.Fatalf("unexpected unknown plan: %+v", evaluation.Plan)
	}
	if len(evaluation.Plan.BlockedFrontiers) != 1 || evaluation.Plan.BlockedFrontiers[0].OperationID != "guardian_credential" {
		t.Fatalf("unexpected blocked frontiers: %+v", evaluation.Plan.BlockedFrontiers)
	}
	unknown := evaluation.Plan.Unknowns[0]
	if unknown.Stage != "guardian" || unknown.Step != "app_credential" || unknown.UnknownClass != "MISSING_EXTERNAL_CREDENTIAL" || unknown.NextOperation != "provide-guardian-app-credential" || len(unknown.BlockedBy) != 1 {
		t.Fatalf("unknown detail was not preserved: %+v", unknown)
	}
}

func TestRefutedPrecedesClosed(t *testing.T) {
	fixture, raw, err := LoadFixture(filepath.Join("..", "..", "fixtures", "cases", "refuted-rest-truncation.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateFixture(fixture, DigestBytes(raw), ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Plan.State != StateRefuted || len(evaluation.Plan.RefutedFrontiers) != 1 || len(evaluation.Plan.ParallelBatches) != 1 || evaluation.Plan.ParallelBatches[0][0] != "capability_alpha" {
		t.Fatalf("unexpected refuted plan: %+v", evaluation.Plan)
	}
	if evaluation.Plan.RefutedFrontiers[0].OperationID != "rest_changed_file_receipt" || len(evaluation.Plan.RefutedFrontiers[0].AffectedOperations) != 2 {
		t.Fatalf("refuted frontier did not include causal descendant: %+v", evaluation.Plan.RefutedFrontiers)
	}
}

func TestMunchhausenChoiceIsExplicitAndDoesNotInventOrder(t *testing.T) {
	fixture, raw, err := LoadFixture(filepath.Join("..", "..", "fixtures", "cases", "unknown-cycle-without-choice.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixture.Graph.Nodes[0].ProofChoice = ProofFoundation
	fixture.Graph.Nodes[1].ProofChoice = ProofFoundation
	evaluation, err := EvaluateFixture(fixture, DigestBytes(raw), ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if len(evaluation.Plan.ParallelBatches) != 0 || len(evaluation.Plan.SerialCuts) != 1 || evaluation.Plan.SerialCuts[0].Relation != "CYCLE_MUNCHHAUSEN_CHOICE" || evaluation.Plan.SerialCuts[0].ProofChoice != ProofFoundation {
		t.Fatalf("cycle was not kept as explicit proof boundary: %+v", evaluation.Plan)
	}
}

func TestMismatchedCacheIdentityCannotCloseNode(t *testing.T) {
	fixture, raw, err := LoadFixture(filepath.Join("..", "..", "fixtures", "cases", "unknown-cache-identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateFixture(fixture, DigestBytes(raw), ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Plan.State != StateUnknown || len(evaluation.Plan.BlockedFrontiers) != 1 || evaluation.Plan.BlockedFrontiers[0].UnknownClass != "IMMUTABLE_IDENTITY_MISMATCH" || len(evaluation.Plan.Evidence) != 1 || evaluation.Plan.Evidence[0].Valid {
		t.Fatalf("mismatched cache evidence was accepted: %+v", evaluation.Plan)
	}
}

