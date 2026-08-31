package frontier

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

func LoadFixture(path string) (Fixture, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, nil, err
	}
	var fixture Fixture
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return Fixture{}, nil, err
	}
	if err := ValidateFixture(fixture); err != nil {
		return Fixture{}, nil, err
	}
	return fixture, raw, nil
}

func ValidateFixture(fixture Fixture) error {
	if fixture.Schema != ProtocolSchema+"/fixture/v1" || fixture.CaseID == "" || fixture.Graph.Schema != ProtocolSchema+"/graph/v1" || fixture.Graph.GraphID == "" || fixture.Graph.DenominatorID == "" || fixture.Graph.MetaActivityCount != 12 || len(fixture.Graph.Nodes) == 0 {
		return errors.New("INVALID_FIXTURE_HEADER")
	}
	if fixture.Expected.State != StateClosed && fixture.Expected.State != StateUnknown && fixture.Expected.State != StateRefuted {
		return errors.New("INVALID_EXPECTED_STATE")
	}
	byID := map[string]Node{}
	byClaim := map[string]bool{}
	for _, node := range fixture.Graph.Nodes {
		if node.ClaimID == "" || node.OperationID == "" || byID[node.OperationID].OperationID != "" || byClaim[node.ClaimID] || !validState(node.Current) || !validProofChoice(node.ProofChoice) || node.Stage == "" || node.Step == "" || node.NextOperation == "" || len(node.ImmutableInputs) == 0 {
			return fmt.Errorf("INVALID_NODE_%s", node.OperationID)
		}
		for identity, digest := range node.ImmutableInputs {
			if identity == "" {
				return fmt.Errorf("INVALID_NODE_INPUT_IDENTITY_%s", node.OperationID)
			}
			if err := ValidateDigest(digest); err != nil {
				return fmt.Errorf("node %s: %w", node.OperationID, err)
			}
		}
		if node.Current == StateUnknown && (node.Stage == "" || node.Step == "" || node.Reason == "" || node.UnknownClass == "" || node.NextOperation == "") {
			return fmt.Errorf("UNKNOWN_NODE_MISSING_PRESERVED_FIELDS_%s", node.OperationID)
		}
		if node.Current == StateClosed && len(node.BlockedBy) != 0 {
			return fmt.Errorf("CLOSED_NODE_HAS_BLOCKER_%s", node.OperationID)
		}
		byID[node.OperationID] = node
		byClaim[node.ClaimID] = true
	}
	for _, node := range fixture.Graph.Nodes {
		for _, dependency := range node.DependsOn {
			if _, ok := byID[dependency]; !ok {
				return fmt.Errorf("UNKNOWN_DEPENDENCY_%s_%s", node.OperationID, dependency)
			}
		}
	}
	evidenceByID := map[string]bool{}
	for _, evidence := range fixture.Graph.Evidence {
		if evidence.EvidenceID == "" || evidence.Kind == "" || evidence.NodeOperationID == "" || evidence.ObservedIdentity == "" || evidence.ExpectedIdentity == "" || evidence.ObservedDigest == "" || evidence.ExpectedDigest == "" || evidenceByID[evidence.EvidenceID] {
			return errors.New("INVALID_EVIDENCE")
		}
		if _, ok := byID[evidence.NodeOperationID]; !ok {
			return fmt.Errorf("EVIDENCE_NODE_NOT_FOUND_%s", evidence.EvidenceID)
		}
		if err := ValidateDigest(evidence.ObservedDigest); err != nil {
			return fmt.Errorf("evidence %s: %w", evidence.EvidenceID, err)
		}
		if err := ValidateDigest(evidence.ExpectedDigest); err != nil {
			return fmt.Errorf("evidence %s: %w", evidence.EvidenceID, err)
		}
		evidenceByID[evidence.EvidenceID] = true
	}
	for _, node := range fixture.Graph.Nodes {
		for _, evidenceID := range node.EvidenceIDs {
			if !evidenceByID[evidenceID] {
				return fmt.Errorf("NODE_EVIDENCE_NOT_FOUND_%s_%s", node.OperationID, evidenceID)
			}
		}
	}
	for _, migration := range fixture.Graph.DenominatorMigrations {
		if migration.MigrationID == "" || migration.FromID == "" || migration.ToID == "" || migration.Reason == "" || len(migration.RetiredCells) == 0 || len(migration.AddedCells) == 0 {
			return errors.New("INVALID_DENOMINATOR_MIGRATION")
		}
		if err := ValidateDigest(migration.EvidenceDigest); err != nil {
			return fmt.Errorf("migration %s: %w", migration.MigrationID, err)
		}
	}
	return nil
}

func EvaluateFixture(fixture Fixture, inputDigest string, source, semanticIR, generatedGo, evaluator, contract ArtifactBinding) (Evaluation, error) {
	if err := ValidateFixture(fixture); err != nil {
		return Evaluation{}, err
	}
	plan, err := makePlan(fixture, inputDigest)
	if err != nil {
		return Evaluation{}, err
	}
	validEvidence := 0
	for _, evidence := range plan.Evidence {
		if evidence.Valid {
			validEvidence++
		}
	}
	counts := countsForPlan(plan, len(fixture.Graph.Nodes), validEvidence)
	receipt := Receipt{
		Schema: ReceiptSchema, CaseID: fixture.CaseID, InputDigest: inputDigest, State: plan.State, DecisionReason: plan.DecisionReason,
		Source: source, SemanticIR: semanticIR, GeneratedGo: generatedGo, Evaluator: evaluator, Contract: contract,
		Authority: Authority{RepositoryWrites: 0, LocalTestExecutions: 0, CrossProjectRequiredGates: 0},
		Counts:    counts, DenominatorID: fixture.Graph.DenominatorID, FixedDenominator: fixture.Graph.MetaActivityCount,
	}
	return Evaluation{Plan: plan, Receipt: receipt}, nil
}

func makePlan(fixture Fixture, inputDigest string) (Plan, error) {
	graph := fixture.Graph
	byID := map[string]Node{}
	ids := make([]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		byID[node.OperationID] = node
		ids = append(ids, node.OperationID)
	}
	sort.Strings(ids)
	closures := dependencyClosures(byID, ids)
	cycles := findCycles(byID, ids)
	cycleMembers := map[string]bool{}
	serialCuts := make([]SerialCut, 0)
	unknownCycleMembers := map[string]bool{}
	for _, cycle := range cycles {
		for _, operationID := range cycle {
			cycleMembers[operationID] = true
		}
		choice := cycleChoice(cycle, byID)
		if choice == "" {
			for _, operationID := range cycle {
				unknownCycleMembers[operationID] = true
			}
			serialCuts = append(serialCuts, SerialCut{Between: cycle, Relation: "CYCLE_REQUIRES_MUNCHHAUSEN_CHOICE", Reason: "The causal graph contains a cycle without one explicit FOUNDATION, COHERENCE, or REGRESSION choice."})
		} else {
			serialCuts = append(serialCuts, SerialCut{Between: cycle, Relation: "CYCLE_MUNCHHAUSEN_CHOICE", ProofChoice: choice, Reason: "The cycle is recorded as an explicit proof boundary; the scheduler invents no internal order."})
		}
	}

	effective, evidenceResults := resolveStates(graph, byID, ids, unknownCycleMembers)
	blockedFrontiers := makeBlockedFrontiers(graph, byID, ids, closures, effective, unknownCycleMembers, evidenceResults)
	refutedFrontiers := makeRefutedFrontiers(graph, byID, ids, closures, effective)
	unknowns := makeUnknownDetails(graph, byID, ids, effective, unknownCycleMembers, evidenceResults)

	eligible := make([]string, 0)
	for _, operationID := range ids {
		if effective[operationID] != StateClosed || cycleMembers[operationID] {
			continue
		}
		ready := true
		for _, dependency := range byID[operationID].DependsOn {
			if effective[dependency] != StateClosed || cycleMembers[dependency] {
				ready = false
				break
			}
		}
		if ready {
			eligible = append(eligible, operationID)
		}
	}
	for index, leftID := range eligible {
		for _, rightID := range eligible[index+1:] {
			left, right := byID[leftID], byID[rightID]
			if shared := intersect(closures[leftID], closures[rightID]); len(shared) > 0 {
				serialCuts = append(serialCuts, SerialCut{Between: []string{leftID, rightID}, Relation: "DEPENDENCY_CLOSURE_INTERSECTION", Shared: shared, Reason: "Intersecting causal dependency closures require serialization."})
			}
			if shared := intersect(left.MutationAuthority, right.MutationAuthority); len(shared) > 0 {
				serialCuts = append(serialCuts, SerialCut{Between: []string{leftID, rightID}, Relation: "MUTATION_AUTHORITY_INTERSECTION", Shared: shared, Reason: "Operations with the same mutation authority must be serialized."})
			}
			if shared := intersect(left.ResourceLocks, right.ResourceLocks); len(shared) > 0 {
				serialCuts = append(serialCuts, SerialCut{Between: []string{leftID, rightID}, Relation: "RESOURCE_LOCK_INTERSECTION", Shared: shared, Reason: "Operations with the same protected resource lock must be serialized."})
			}
		}
	}

	parallelBatches := makeBatches(eligible, byID, closures)
	sortSerialCuts(serialCuts)
	state := StateClosed
	decisionReason := "CLOSED: every emitted transition is justified by explicit causal dependencies and disjoint authorities."
	for _, operationID := range ids {
		if effective[operationID] == StateRefuted {
			state = StateRefuted
			decisionReason = "REFUTED: known contradiction takes precedence over UNKNOWN and CLOSED; affected operations are not schedulable."
			break
		}
	}
	if state != StateRefuted && len(unknowns) > 0 {
		state = StateUnknown
		decisionReason = "UNKNOWN: only the minimal causal blockers remain unresolved; unrelated operations continue in their own batches."
	}
	plan := Plan{
		Schema: PlanSchema, CaseID: fixture.CaseID, GraphID: graph.GraphID, InputDigest: inputDigest, State: state, DecisionReason: decisionReason,
		ParallelBatches: parallelBatches, SerialCuts: serialCuts, BlockedFrontiers: blockedFrontiers, RefutedFrontiers: refutedFrontiers,
		Unknowns: unknowns, Evidence: evidenceResults, DenominatorMigrations: sortedMigrations(graph.DenominatorMigrations),
	}
	plan.Dossier = renderDossier(plan, fixture)
	return plan, nil
}

func resolveStates(graph Graph, byID map[string]Node, ids []string, unknownCycleMembers map[string]bool) (map[string]State, []EvidenceResult) {
	effective := map[string]State{}
	for _, operationID := range ids {
		effective[operationID] = byID[operationID].Current
		if unknownCycleMembers[operationID] {
			effective[operationID] = StateUnknown
		}
	}
	evidenceResults := assessEvidence(graph, byID)
	invalidEvidence := map[string]bool{}
	for _, evidence := range evidenceResults {
		if !evidence.Valid {
			for _, evidenceInput := range graph.Evidence {
				if evidenceInput.EvidenceID == evidence.EvidenceID {
					invalidEvidence[evidenceInput.NodeOperationID] = true
				}
			}
		}
	}
	for operationID := range invalidEvidence {
		if effective[operationID] != StateRefuted {
			effective[operationID] = StateUnknown
		}
	}
	for iteration := 0; iteration < len(ids)+1; iteration++ {
		changed := false
		for _, operationID := range ids {
			node := byID[operationID]
			next := effective[operationID]
			for _, dependency := range node.DependsOn {
				if effective[dependency] == StateRefuted {
					next = StateRefuted
					break
				}
				if effective[dependency] == StateUnknown && next == StateClosed {
					next = StateUnknown
				}
			}
			for _, blocker := range node.BlockedBy {
				if effective[blocker] == StateRefuted {
					next = StateRefuted
				}
			}
			if next != effective[operationID] {
				effective[operationID] = next
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return effective, evidenceResults
}

func assessEvidence(graph Graph, byID map[string]Node) []EvidenceResult {
	results := make([]EvidenceResult, 0, len(graph.Evidence))
	evidence := append([]Evidence(nil), graph.Evidence...)
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].EvidenceID < evidence[j].EvidenceID })
	for _, item := range evidence {
		node := byID[item.NodeOperationID]
		valid := item.ObservedIdentity == item.ExpectedIdentity && item.ObservedDigest == item.ExpectedDigest && node.ImmutableInputs[item.ExpectedIdentity] == item.ExpectedDigest
		reason := "exact immutable identity and digest match"
		if !valid {
			reason = "ignored: observed identity or digest does not exactly match the node's immutable input binding"
		}
		results = append(results, EvidenceResult{EvidenceID: item.EvidenceID, Kind: item.Kind, Valid: valid, Reason: reason})
	}
	return results
}

func makeBlockedFrontiers(graph Graph, byID map[string]Node, ids []string, closures map[string][]string, effective map[string]State, cycleMembers map[string]bool, evidence []EvidenceResult) []Frontier {
	frontier := make([]Frontier, 0)
	for _, operationID := range ids {
		node := byID[operationID]
		if effective[operationID] != StateUnknown || (node.Current != StateUnknown && !cycleMembers[operationID] && !nodeHasInvalidEvidence(node, evidence)) {
			continue
		}
		root := true
		for _, dependency := range node.DependsOn {
			if effective[dependency] == StateUnknown {
				root = false
				break
			}
		}
		if !root {
			continue
		}
		affected := make([]string, 0)
		for _, candidate := range ids {
			if effective[candidate] == StateUnknown && contains(closures[candidate], operationID) {
				affected = append(affected, candidate)
			}
		}
		reason := node.Reason
		unknownClass := node.UnknownClass
		if reason == "" {
			if nodeHasInvalidEvidence(node, evidence) {
				for _, item := range evidence {
					if !item.Valid && contains(node.EvidenceIDs, item.EvidenceID) {
						reason = item.Reason
						unknownClass = "IMMUTABLE_IDENTITY_MISMATCH"
						break
					}
				}
			} else {
				reason = "Causal cycle requires an explicit Münchhausen proof choice before scheduling."
				unknownClass = "CYCLE_REQUIRES_PROOF_CHOICE"
			}
		}
		frontier = append(frontier, Frontier{OperationID: operationID, ClaimID: node.ClaimID, State: StateUnknown, Stage: node.Stage, Step: node.Step, Reason: reason, UnknownClass: unknownClass, NextOperation: node.NextOperation, BlockedBy: sortedStrings(node.BlockedBy), AffectedOperations: affected})
	}
	return frontier
}

func makeRefutedFrontiers(graph Graph, byID map[string]Node, ids []string, closures map[string][]string, effective map[string]State) []Frontier {
	frontier := make([]Frontier, 0)
	for _, operationID := range ids {
		node := byID[operationID]
		if effective[operationID] != StateRefuted || node.Current != StateRefuted {
			continue
		}
		root := true
		for _, dependency := range node.DependsOn {
			if effective[dependency] == StateRefuted {
				root = false
				break
			}
		}
		if !root {
			continue
		}
		affected := make([]string, 0)
		for _, candidate := range ids {
			if effective[candidate] == StateRefuted && contains(closures[candidate], operationID) {
				affected = append(affected, candidate)
			}
		}
		reason := node.Reason
		if reason == "" {
			reason = "Explicit contradiction in the causal graph."
		}
		frontier = append(frontier, Frontier{OperationID: operationID, ClaimID: node.ClaimID, State: StateRefuted, Stage: node.Stage, Step: node.Step, Reason: reason, NextOperation: node.NextOperation, BlockedBy: sortedStrings(node.BlockedBy), AffectedOperations: affected})
	}
	return frontier
}

func makeUnknownDetails(graph Graph, byID map[string]Node, ids []string, effective map[string]State, cycleMembers map[string]bool, evidence []EvidenceResult) []UnknownDetail {
	unknowns := make([]UnknownDetail, 0)
	for _, operationID := range ids {
		node := byID[operationID]
		if effective[operationID] != StateUnknown || (node.Current != StateUnknown && !cycleMembers[operationID] && !nodeHasInvalidEvidence(node, evidence)) {
			continue
		}
		reason, unknownClass := node.Reason, node.UnknownClass
		if reason == "" {
			reason = "Causal cycle requires an explicit Münchhausen proof choice before scheduling."
			unknownClass = "CYCLE_REQUIRES_PROOF_CHOICE"
		}
		if len(node.EvidenceIDs) > 0 {
			for _, evidenceID := range node.EvidenceIDs {
				for _, item := range evidence {
					if item.EvidenceID == evidenceID && !item.Valid {
						reason = item.Reason
						unknownClass = "IMMUTABLE_IDENTITY_MISMATCH"
					}
				}
			}
		}
		unknowns = append(unknowns, UnknownDetail{OperationID: operationID, Stage: node.Stage, Step: node.Step, Reason: reason, UnknownClass: unknownClass, NextOperation: node.NextOperation, BlockedBy: sortedStrings(node.BlockedBy)})
	}
	return unknowns
}

func makeBatches(eligible []string, byID map[string]Node, closures map[string][]string) [][]string {
	pending := map[string]bool{}
	for _, operationID := range eligible {
		pending[operationID] = true
	}
	batches := make([][]string, 0)
	for len(pending) > 0 {
		ready := make([]string, 0)
		for _, operationID := range eligible {
			if !pending[operationID] {
				continue
			}
			readyNow := true
			for _, dependency := range byID[operationID].DependsOn {
				if pending[dependency] {
					readyNow = false
					break
				}
			}
			if readyNow {
				ready = append(ready, operationID)
			}
		}
		if len(ready) == 0 {
			break
		}
		batch := make([]string, 0)
		for _, operationID := range ready {
			compatible := true
			for _, selected := range batch {
				if len(intersect(closures[operationID], closures[selected])) > 0 || len(intersect(authorities(byID[operationID]), authorities(byID[selected]))) > 0 {
					compatible = false
					break
				}
			}
			if compatible {
				batch = append(batch, operationID)
				delete(pending, operationID)
			}
		}
		if len(batch) == 0 {
			break
		}
		batches = append(batches, batch)
	}
	return batches
}

func countsForPlan(plan Plan, nodeCount, validEvidence int) Counts {
	schedulable := 0
	for _, batch := range plan.ParallelBatches {
		schedulable += len(batch)
	}
	blocked, refuted := 0, 0
	for _, frontier := range plan.BlockedFrontiers {
		blocked += len(frontier.AffectedOperations)
	}
	for _, frontier := range plan.RefutedFrontiers {
		refuted += len(frontier.AffectedOperations)
	}
	notObserved := nodeCount - schedulable - validEvidence
	if notObserved < 0 {
		notObserved = 0
	}
	return Counts{ParallelBatches: len(plan.ParallelBatches), Schedulable: schedulable, Blocked: blocked, Refuted: refuted, Unknown: len(plan.Unknowns), Executed: 0, Reused: validEvidence, Skipped: 0, NotObserved: notObserved}
}

func dependencyClosures(byID map[string]Node, ids []string) map[string][]string {
	closures := map[string][]string{}
	var visit func(string, map[string]bool) []string
	visit = func(operationID string, visiting map[string]bool) []string {
		if cached, ok := closures[operationID]; ok {
			return cached
		}
		if visiting[operationID] {
			return []string{operationID}
		}
		visiting[operationID] = true
		seen := map[string]bool{operationID: true}
		for _, dependency := range byID[operationID].DependsOn {
			for _, item := range visit(dependency, visiting) {
				seen[item] = true
			}
		}
		delete(visiting, operationID)
		items := make([]string, 0, len(seen))
		for item := range seen {
			items = append(items, item)
		}
		sort.Strings(items)
		closures[operationID] = items
		return items
	}
	for _, operationID := range ids {
		visit(operationID, map[string]bool{})
	}
	return closures
}

func findCycles(byID map[string]Node, ids []string) [][]string {
	index := 0
	indices := map[string]int{}
	lowlink := map[string]int{}
	onStack := map[string]bool{}
	stack := make([]string, 0)
	cycles := make([][]string, 0)
	var strongConnect func(string)
	strongConnect = func(operationID string) {
		indices[operationID] = index
		lowlink[operationID] = index
		index++
		stack = append(stack, operationID)
		onStack[operationID] = true
		dependencies := append([]string(nil), byID[operationID].DependsOn...)
		sort.Strings(dependencies)
		for _, dependency := range dependencies {
			if _, seen := indices[dependency]; !seen {
				strongConnect(dependency)
				if lowlink[dependency] < lowlink[operationID] {
					lowlink[operationID] = lowlink[dependency]
				}
			} else if onStack[dependency] && indices[dependency] < lowlink[operationID] {
				lowlink[operationID] = indices[dependency]
			}
		}
		if lowlink[operationID] != indices[operationID] {
			return
		}
		component := make([]string, 0)
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			component = append(component, last)
			if last == operationID {
				break
			}
		}
		if len(component) > 1 || (len(component) == 1 && contains(byID[component[0]].DependsOn, component[0])) {
			sort.Strings(component)
			cycles = append(cycles, component)
		}
	}
	for _, operationID := range ids {
		if _, seen := indices[operationID]; !seen {
			strongConnect(operationID)
		}
	}
	sort.Slice(cycles, func(i, j int) bool { return strings.Join(cycles[i], "\x00") < strings.Join(cycles[j], "\x00") })
	return cycles
}

func cycleChoice(cycle []string, byID map[string]Node) ProofChoice {
	if len(cycle) == 0 {
		return ""
	}
	choice := byID[cycle[0]].ProofChoice
	for _, operationID := range cycle[1:] {
		if byID[operationID].ProofChoice != choice {
			return ""
		}
	}
	if !validProofChoice(choice) {
		return ""
	}
	return choice
}

func authorities(node Node) []string {
	return append(append([]string{}, node.MutationAuthority...), node.ResourceLocks...)
}

func intersect(left, right []string) []string {
	leftSet := map[string]bool{}
	for _, item := range left {
		leftSet[item] = true
	}
	seen := map[string]bool{}
	result := make([]string, 0)
	for _, item := range right {
		if leftSet[item] && !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	sort.Strings(result)
	return result
}

func contains(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}

func sortedStrings(items []string) []string {
	copyItems := append([]string(nil), items...)
	sort.Strings(copyItems)
	return copyItems
}

func sortedMigrations(items []DenominatorMigration) []DenominatorMigration {
	copyItems := append([]DenominatorMigration(nil), items...)
	sort.Slice(copyItems, func(i, j int) bool { return copyItems[i].MigrationID < copyItems[j].MigrationID })
	return copyItems
}

func sortSerialCuts(cuts []SerialCut) {
	sort.Slice(cuts, func(i, j int) bool {
		left, right := strings.Join(cuts[i].Between, "\x00"), strings.Join(cuts[j].Between, "\x00")
		if left != right {
			return left < right
		}
		if cuts[i].Relation != cuts[j].Relation {
			return cuts[i].Relation < cuts[j].Relation
		}
		return strings.Join(cuts[i].Shared, "\x00") < strings.Join(cuts[j].Shared, "\x00")
	})
}

func validState(state State) bool {
	return state == StateClosed || state == StateUnknown || state == StateRefuted
}

func nodeHasInvalidEvidence(node Node, evidence []EvidenceResult) bool {
	for _, evidenceID := range node.EvidenceIDs {
		for _, item := range evidence {
			if item.EvidenceID == evidenceID && !item.Valid {
				return true
			}
		}
	}
	return false
}
