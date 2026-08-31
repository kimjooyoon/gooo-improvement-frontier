package frontier

const (
	ProtocolSchema = "gooo/improvement-frontier/protocol/v1"
	SourceSchema   = "gooo/improvement-frontier/v1"
	IRSchema       = "gooo/improvement-frontier/ir/v1"
	PlanSchema     = "gooo/improvement-frontier/plan/v1"
	ReceiptSchema  = "gooo/improvement-frontier/receipt/v1"
	CorpusSchema   = "gooo/improvement-frontier/corpus/v1"

	StateClosed  State = "CLOSED"
	StateUnknown State = "UNKNOWN"
	StateRefuted State = "REFUTED"

	ProofFoundation ProofChoice = "FOUNDATION"
	ProofCoherence  ProofChoice = "COHERENCE"
	ProofRegression ProofChoice = "REGRESSION"
)

var StatePrecedence = []State{StateRefuted, StateUnknown, StateClosed}

type State string

type ProofChoice string

type Graph struct {
	Schema                string                 `json:"schema"`
	GraphID               string                 `json:"graph_id"`
	DenominatorID         string                 `json:"denominator_id"`
	MetaActivityCount     int                    `json:"meta_activity_count"`
	Nodes                 []Node                 `json:"nodes"`
	Evidence              []Evidence             `json:"evidence"`
	DenominatorMigrations []DenominatorMigration `json:"denominator_migrations"`
}

type Node struct {
	ClaimID           string            `json:"claim_id"`
	OperationID       string            `json:"operation_id"`
	Current           State             `json:"current"`
	Stage             string            `json:"stage"`
	Step              string            `json:"step"`
	ProofChoice       ProofChoice       `json:"proof_choice"`
	DependsOn         []string          `json:"depends_on"`
	BlockedBy         []string          `json:"blocked_by"`
	MutationAuthority []string          `json:"mutation_authority"`
	ResourceLocks     []string          `json:"resource_locks"`
	ImmutableInputs   map[string]string `json:"immutable_inputs"`
	EvidenceIDs       []string          `json:"evidence_ids"`
	NextOperation     string            `json:"next_operation"`
	Reason            string            `json:"reason,omitempty"`
	UnknownClass      string            `json:"unknown_class,omitempty"`
}

type Evidence struct {
	EvidenceID       string `json:"evidence_id"`
	Kind             string `json:"kind"`
	NodeOperationID  string `json:"node_operation_id"`
	ObservedIdentity string `json:"observed_identity"`
	ExpectedIdentity string `json:"expected_identity"`
	ObservedDigest   string `json:"observed_digest"`
	ExpectedDigest   string `json:"expected_digest"`
}

type DenominatorMigration struct {
	MigrationID    string   `json:"migration_id"`
	FromID         string   `json:"from_id"`
	ToID           string   `json:"to_id"`
	RetiredCells   []string `json:"retired_cells"`
	AddedCells     []string `json:"added_cells"`
	Reason         string   `json:"reason"`
	EvidenceDigest string   `json:"evidence_digest"`
}

type Contract struct {
	Schema            string         `json:"schema"`
	DenominatorID     string         `json:"denominator_id"`
	FixedDenominator  int            `json:"fixed_denominator"`
	MetaActivityCount int            `json:"meta_activity_count"`
	Cells             []ContractCell `json:"cells"`
	StatePrecedence   []State        `json:"state_precedence"`
	ForbiddenRanking  []string       `json:"forbidden_ranking"`
}

type ContractCell struct {
	Ordinal     int    `json:"ordinal"`
	ClaimID     string `json:"claim_id"`
	OperationID string `json:"operation_id"`
	Stage       string `json:"stage"`
	Step        string `json:"step"`
	ProofChoice string `json:"proof_choice"`
}

type SourceActivity struct {
	ClaimID     string `json:"claim_id"`
	OperationID string `json:"operation_id"`
	Stage       string `json:"stage"`
	Step        string `json:"step"`
	ProofChoice string `json:"proof_choice"`
	SourceLine  int    `json:"source_line"`
}

type SemanticIR struct {
	Schema            string           `json:"schema"`
	SourcePath        string           `json:"source_path"`
	SourceDigest      string           `json:"source_digest"`
	ContractDigest    string           `json:"contract_digest"`
	MetaActivityCount int              `json:"meta_activity_count"`
	Activities        []SourceActivity `json:"activities"`
}

type Fixture struct {
	Schema      string       `json:"schema"`
	CaseID      string       `json:"case_id"`
	Description string       `json:"description"`
	Graph       Graph        `json:"graph"`
	Expected    Expectations `json:"expected"`
}

type Expectations struct {
	State                State      `json:"state"`
	ParallelBatches      [][]string `json:"parallel_batches"`
	BlockedFrontierRoots []string   `json:"blocked_frontier_roots"`
	RefutedFrontierRoots []string   `json:"refuted_frontier_roots"`
	SerialCutRelations   []string   `json:"serial_cut_relations"`
	ValidEvidenceIDs     []string   `json:"valid_evidence_ids"`
	InvalidEvidenceIDs   []string   `json:"invalid_evidence_ids"`
}

type Corpus struct {
	Schema           string       `json:"schema"`
	CorpusID         string       `json:"corpus_id"`
	DenominatorID    string       `json:"denominator_id"`
	FixedDenominator int          `json:"fixed_denominator"`
	Cases            []CorpusCase `json:"cases"`
}

type CorpusCase struct {
	Ordinal int    `json:"ordinal"`
	CaseID  string `json:"case_id"`
	Path    string `json:"path"`
	State   State  `json:"state"`
}

type SerialCut struct {
	Between     []string    `json:"between"`
	Relation    string      `json:"relation"`
	Shared      []string    `json:"shared"`
	ProofChoice ProofChoice `json:"proof_choice,omitempty"`
	Reason      string      `json:"reason"`
}

type Frontier struct {
	OperationID        string   `json:"operation_id"`
	ClaimID            string   `json:"claim_id"`
	State              State    `json:"state"`
	Stage              string   `json:"stage"`
	Step               string   `json:"step"`
	Reason             string   `json:"reason"`
	UnknownClass       string   `json:"unknown_class,omitempty"`
	NextOperation      string   `json:"next_operation"`
	BlockedBy          []string `json:"blocked_by"`
	AffectedOperations []string `json:"affected_operations"`
}

type UnknownDetail struct {
	OperationID   string   `json:"operation_id"`
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type EvidenceResult struct {
	EvidenceID string `json:"evidence_id"`
	Kind       string `json:"kind"`
	Valid      bool   `json:"valid"`
	Reason     string `json:"reason"`
}

type Authority struct {
	RepositoryWrites          int `json:"repository_writes"`
	LocalTestExecutions       int `json:"local_test_executions"`
	CrossProjectRequiredGates int `json:"cross_project_required_gates"`
}

type Plan struct {
	Schema                string                 `json:"schema"`
	CaseID                string                 `json:"case_id"`
	GraphID               string                 `json:"graph_id"`
	InputDigest           string                 `json:"input_digest"`
	State                 State                  `json:"state"`
	DecisionReason        string                 `json:"decision_reason"`
	ParallelBatches       [][]string             `json:"parallel_batches"`
	SerialCuts            []SerialCut            `json:"serial_cuts"`
	BlockedFrontiers      []Frontier             `json:"blocked_frontiers"`
	RefutedFrontiers      []Frontier             `json:"refuted_frontiers"`
	Unknowns              []UnknownDetail        `json:"unknowns"`
	Evidence              []EvidenceResult       `json:"evidence"`
	DenominatorMigrations []DenominatorMigration `json:"denominator_migrations"`
	Dossier               string                 `json:"human_dossier"`
}

type ArtifactBinding struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type Receipt struct {
	Schema           string          `json:"schema"`
	CaseID           string          `json:"case_id"`
	InputDigest      string          `json:"input_digest"`
	State            State           `json:"state"`
	DecisionReason   string          `json:"decision_reason"`
	Source           ArtifactBinding `json:"source"`
	SemanticIR       ArtifactBinding `json:"semantic_ir"`
	GeneratedGo      ArtifactBinding `json:"generated_go"`
	Evaluator        ArtifactBinding `json:"evaluator"`
	Contract         ArtifactBinding `json:"contract"`
	Authority        Authority       `json:"product_authority"`
	Counts           Counts          `json:"counts"`
	DenominatorID    string          `json:"denominator_id"`
	FixedDenominator int             `json:"fixed_denominator"`
}

type Counts struct {
	ParallelBatches int `json:"parallel_batches"`
	Schedulable     int `json:"schedulable"`
	Blocked         int `json:"blocked"`
	Refuted         int `json:"refuted"`
	Unknown         int `json:"unknown"`
	Executed        int `json:"executed"`
	Reused          int `json:"reused"`
	Skipped         int `json:"skipped"`
	NotObserved     int `json:"not_observed"`
}

type Evaluation struct {
	Plan    Plan    `json:"plan"`
	Receipt Receipt `json:"receipt"`
}
