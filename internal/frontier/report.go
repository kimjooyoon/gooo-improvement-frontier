package frontier

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func WriteEvaluation(outputDir string, evaluation Evaluation) error {
	if !filepath.IsAbs(outputDir) {
		return errors.New("output directory must be an absolute caller-owned path")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	planRaw, err := json.MarshalIndent(evaluation.Plan, "", "  ")
	if err != nil {
		return err
	}
	receiptRaw, err := json.MarshalIndent(evaluation.Receipt, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "plan.json"), append(planRaw, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "receipt.json"), append(receiptRaw, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "human-report.md"), []byte(evaluation.Plan.Dossier), 0o644)
}

func renderDossier(plan Plan, fixture Fixture) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Improvement frontier dossier: `%s`\n\n", plan.CaseID)
	fmt.Fprintf(&builder, "- Decision: **%s**\n- Reason: %s\n- Graph: `%s`\n- Input digest: `%s`\n- Fixed denominator: `%d` activities (`%s`)\n\n", plan.State, plan.DecisionReason, plan.GraphID, plan.InputDigest, fixture.Graph.MetaActivityCount, fixture.Graph.DenominatorID)
	builder.WriteString("## Parallel batches\n\n")
	if len(plan.ParallelBatches) == 0 {
		builder.WriteString("No operation is schedulable in a parallel batch.\n\n")
	} else {
		for index, batch := range plan.ParallelBatches {
			fmt.Fprintf(&builder, "%d. `%s`\n", index+1, strings.Join(batch, "`, `"))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Serialized cuts\n\n")
	if len(plan.SerialCuts) == 0 {
		builder.WriteString("No causal or authority serialization cut was required.\n\n")
	} else {
		for _, cut := range plan.SerialCuts {
			choice := ""
			if cut.ProofChoice != "" {
				choice = fmt.Sprintf(" proof_choice=%s.", cut.ProofChoice)
			}
			shared := ""
			if len(cut.Shared) > 0 {
				shared = fmt.Sprintf(" Shared: `%s`.", strings.Join(cut.Shared, "`, `"))
			}
			fmt.Fprintf(&builder, "- `%s` — %s.%s%s\n", strings.Join(cut.Between, "` ↔ `"), cut.Relation, choice, shared)
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Blocked frontiers\n\n")
	if len(plan.BlockedFrontiers) == 0 {
		builder.WriteString("None.\n\n")
	} else {
		for _, frontier := range plan.BlockedFrontiers {
			fmt.Fprintf(&builder, "- `%s` (%s/%s, %s): %s Next: `%s`. Affected: `%s`.\n", frontier.OperationID, frontier.Stage, frontier.Step, frontier.UnknownClass, frontier.Reason, frontier.NextOperation, strings.Join(frontier.AffectedOperations, "`, `"))
			if len(frontier.BlockedBy) > 0 {
				fmt.Fprintf(&builder, "  Blocked by: `%s`.\n", strings.Join(frontier.BlockedBy, "`, `"))
			}
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Refuted frontiers\n\n")
	if len(plan.RefutedFrontiers) == 0 {
		builder.WriteString("None.\n\n")
	} else {
		for _, frontier := range plan.RefutedFrontiers {
			fmt.Fprintf(&builder, "- `%s` (%s/%s): %s Affected: `%s`.\n", frontier.OperationID, frontier.Stage, frontier.Step, frontier.Reason, strings.Join(frontier.AffectedOperations, "`, `"))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Evidence and authority\n\n")
	for _, evidence := range plan.Evidence {
		fmt.Fprintf(&builder, "- `%s` (%s): valid=%t — %s.\n", evidence.EvidenceID, evidence.Kind, evidence.Valid, evidence.Reason)
	}
	if len(plan.Evidence) == 0 {
		builder.WriteString("No cache, green-check, or previous-execution evidence was supplied.\n")
	}
	builder.WriteString("\nThe scheduler emits this plan and its receipts only. It does not mutate repositories, merge pull requests, alter CI, or execute downstream operations. Product authority is fixed at repository_writes=0, local_test_executions=0, cross_project_required_gates=0.\n\n")
	builder.WriteString("## Denominator migrations\n\n")
	if len(plan.DenominatorMigrations) == 0 {
		builder.WriteString("None.\n")
	} else {
		for _, migration := range plan.DenominatorMigrations {
			fmt.Fprintf(&builder, "- `%s`: `%s` → `%s`; retired `%s`; added `%s`; reason: %s.\n", migration.MigrationID, migration.FromID, migration.ToID, strings.Join(migration.RetiredCells, "`, `"), strings.Join(migration.AddedCells, "`, `"), migration.Reason)
		}
	}
	return builder.String()
}
