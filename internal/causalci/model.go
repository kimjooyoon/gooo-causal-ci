package causalci

import "encoding/json"

const (
	InputSchema       = "gooo/causal-ci/input/v1"
	PlanSchema        = "gooo/causal-ci/activity-plan/v1"
	ReportSchema      = "gooo/causal-ci/report/v1"
	DenominatorSchema = "gooo/causal-ci/denominator/v1"

	DecisionClosed  = "CLOSED"
	DecisionUnknown = "UNKNOWN"
	DecisionRefuted = "REFUTED"

	ActionExecute = "EXECUTE"
	ActionReuse   = "REUSE"
	ActionSkip    = "SKIP"
	ActionUnknown = "UNKNOWN"

	StateClosed  = "CLOSED"
	StateUnknown = "UNKNOWN"
	StateRefuted = "REFUTED"
)

type Denominator struct {
	Schema          string            `json:"schema"`
	DenominatorID   string            `json:"denominator_id"`
	Total           int               `json:"total"`
	ProofTotals     []Bucket          `json:"proof_totals"`
	IndicatorTotals []Bucket          `json:"indicator_totals"`
	Cells           []DenominatorCell `json:"cells"`
}

type Bucket struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
}

type DenominatorCell struct {
	ID             string `json:"id"`
	Activity       string `json:"activity"`
	Stage          string `json:"stage"`
	Step           string `json:"step"`
	ProofChoice    string `json:"proof_choice"`
	IndicatorClass string `json:"indicator_class"`
	MetricPath     string `json:"metric_path"`
	ClosedReason   string `json:"closed_reason"`
	UnknownReason  string `json:"unknown_reason"`
	RefutedReason  string `json:"refuted_reason"`
	NextOperation  string `json:"next_operation"`
}

type Input struct {
	Schema          string            `json:"schema"`
	Subject         Subject           `json:"subject"`
	ContractDigest  string            `json:"contract_digest"`
	SourceBinding   SourceBinding     `json:"source_binding"`
	SemanticGraph   SemanticGraph     `json:"semantic_graph"`
	Bindings        []ActivityBinding `json:"bindings"`
	Claims          []ClaimEvidence   `json:"claims"`
	Changes         []Change          `json:"changes"`
	Tests           []TestDefinition  `json:"tests"`
	ProposedActions []TestAction      `json:"proposed_actions,omitempty"`
	FullCommand     []string          `json:"full_command"`
	Authority       Authority         `json:"authority"`
	ControlDecision string            `json:"control_decision"`
}

type Subject struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
}

type SourceBinding struct {
	Path           string `json:"path"`
	SourceDigest   string `json:"source_digest"`
	SemanticDigest string `json:"semantic_digest"`
	GraphDigest    string `json:"graph_digest"`
}

type SemanticGraph struct {
	SchemaVersion  string          `json:"schema_version"`
	SourceDigest   string          `json:"source_digest"`
	SemanticDigest string          `json:"semantic_digest"`
	GraphDigest    string          `json:"graph_digest"`
	Activities     []GraphActivity `json:"activities"`
}

type GraphActivity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ActivityBinding struct {
	Activity       string `json:"activity"`
	GraphNodeID    string `json:"graph_node_id"`
	SourceDigest   string `json:"source_digest"`
	SemanticDigest string `json:"semantic_digest"`
	BindingDigest  string `json:"binding_digest"`
}

type ClaimEvidence struct {
	ID             string              `json:"id"`
	Subject        string              `json:"subject"`
	ActivityPath   []string            `json:"activity_path"`
	SourceDigest   string              `json:"source_digest"`
	SemanticDigest string              `json:"semantic_digest"`
	GraphDigest    string              `json:"graph_digest"`
	EvidenceDigest string              `json:"evidence_digest"`
	Exclusions     []ExclusionEvidence `json:"exclusions"`
}

type ExclusionEvidence struct {
	TestID       string   `json:"test_id"`
	Reason       string   `json:"reason"`
	ActivityPath []string `json:"activity_path"`
}

type Change struct {
	ID      string `json:"id"`
	ClaimID string `json:"claim_id"`
}

type TestDefinition struct {
	ID             string        `json:"id"`
	Command        []string      `json:"command"`
	Activities     []string      `json:"activities"`
	ReuseCandidate bool          `json:"reuse_candidate"`
	PriorReceipt   *PriorReceipt `json:"prior_receipt,omitempty"`
}

type PriorReceipt struct {
	Status          string `json:"status"`
	SourceDigest    string `json:"source_digest"`
	SemanticDigest  string `json:"semantic_digest"`
	GraphDigest     string `json:"graph_digest"`
	ContractDigest  string `json:"contract_digest"`
	ToolchainDigest string `json:"toolchain_digest"`
	ResultDigest    string `json:"result_digest"`
}

type Authority struct {
	RepositoryWrites int  `json:"repository_writes"`
	MutationAllowed  bool `json:"mutation_allowed"`
}

type WorkspaceInventory struct {
	DescendantDirs     int  `json:"descendant_dirs"`
	RegularFiles       int  `json:"regular_files"`
	GoFiles            int  `json:"go_files"`
	GoLines            int  `json:"go_lines"`
	GoooFiles          int  `json:"gooo_files"`
	GoooLines          int  `json:"gooo_lines"`
	RootReadmeExcluded bool `json:"root_readme_excluded"`
}

type Plan struct {
	Schema          string            `json:"schema"`
	Decision        string            `json:"decision"`
	SelectionMode   string            `json:"selection_mode"`
	ContractDigest  string            `json:"contract_digest"`
	InputDigest     string            `json:"input_digest"`
	SourceBinding   SourceBinding     `json:"source_binding"`
	Activities      []ActivityReceipt `json:"activities"`
	Tests           []TestAction      `json:"tests"`
	Unknowns        []CausalState     `json:"unknowns"`
	Refutations     []CausalState     `json:"refutations"`
	Claim           CausalState       `json:"claim"`
	Metrics         PlanMetrics       `json:"metrics"`
}

type ActivityReceipt struct {
	CellID         string   `json:"cell_id"`
	Activity       string   `json:"activity"`
	State          string   `json:"state"`
	ProofChoice    string   `json:"proof_choice"`
	IndicatorClass string   `json:"indicator_class"`
	SourcePath     string   `json:"source_path"`
	GraphNodeID    string   `json:"graph_node_id"`
	BindingDigest  string   `json:"binding_digest"`
	ArtifactPath   string   `json:"artifact_path"`
	Evaluator      string   `json:"evaluator"`
}

type TestAction struct {
	ID                string              `json:"id"`
	Command           []string            `json:"command"`
	Action            string              `json:"action"`
	CausalClaimIDs    []string            `json:"causal_claim_ids"`
	ExclusionEvidence []ExclusionEvidence `json:"exclusion_evidence"`
	Reason            string              `json:"reason"`
}

type CausalState struct {
	State         string   `json:"state"`
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type PlanMetrics struct {
	Denominator int `json:"denominator"`
	Closed      int `json:"closed"`
	Unknown     int `json:"unknown"`
	Refuted     int `json:"refuted"`
	Executed    int `json:"executed"`
	Reused      int `json:"reused"`
	Skipped     int `json:"skipped"`
}

type Runtime struct {
	Schema       string             `json:"schema"`
	SubjectSHA   string             `json:"subject_sha"`
	Toolchain    Toolchain          `json:"toolchain"`
	Inventory    WorkspaceInventory `json:"inventory"`
	Build        Measurement        `json:"build"`
	TestFull     TestRun            `json:"test_full"`
	TestSelected TestRun            `json:"test_selected"`
	Conformance  Measurement       `json:"conformance"`
	Pair         ExactPair          `json:"exact_pair"`
	Authority    Authority          `json:"authority"`
	Cases        []CaseResult       `json:"cases"`
}

type Toolchain struct {
	GoVersion   string `json:"go_version"`
	GoooVersion string `json:"gooo_version"`
	GoooDigest  string `json:"gooo_digest"`
}

type Measurement struct {
	Status     string `json:"status"`
	WallMS     int    `json:"wall_ms"`
	PeakRSSKib int    `json:"peak_rss_kib"`
}

type TestRun struct {
	Status      string            `json:"status"`
	WallMS      int               `json:"wall_ms"`
	PeakRSSKib  int               `json:"peak_rss_kib"`
	Observations []TestObservation `json:"observations"`
}

type TestObservation struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type ExactPair struct {
	Exact          bool   `json:"exact"`
	InputDigest    string `json:"input_digest"`
	ToolchainDigest string `json:"toolchain_digest"`
	ContractDigest string `json:"contract_digest"`
}

type CaseResult struct {
	ID       string `json:"id"`
	Decision string `json:"decision"`
	State    string `json:"state"`
	Stage    string `json:"stage"`
	Step     string `json:"step"`
	Reason   string `json:"reason"`
}

type Report struct {
	Schema        string             `json:"schema"`
	Decision      string             `json:"decision"`
	Claim         CausalState        `json:"claim"`
	SourceBinding SourceBinding      `json:"source_binding"`
	Metrics       ReportMetrics      `json:"metrics"`
	Tests         ReportTests        `json:"tests"`
	Inventory     WorkspaceInventory `json:"inventory"`
	Performance   ReportPerformance  `json:"performance"`
	Authority     ReportAuthority    `json:"authority"`
	Cases         []CaseResult       `json:"cases"`
	Artifacts     ArtifactInventory  `json:"artifacts"`
	Evidence      []ActivityEvidence `json:"evidence"`
	Digest        string             `json:"digest"`
}

type ReportMetrics struct {
	Denominator int `json:"denominator"`
	Closed      int `json:"closed"`
	Unknown     int `json:"unknown"`
	Refuted     int `json:"refuted"`
	Executed    int `json:"executed"`
	Reused      int `json:"reused"`
	Skipped     int `json:"skipped"`
}

type ReportTests struct {
	Total      int `json:"total"`
	Full       TestCounts `json:"full"`
	Selected   TestCounts `json:"selected"`
}

type TestCounts struct {
	Executed int `json:"executed"`
	Reused   int `json:"reused"`
	Skipped  int `json:"skipped"`
}

type ReportPerformance struct {
	BuildWallMS          int    `json:"build_wall_ms"`
	BuildPeakRSSKib      int    `json:"build_peak_rss_kib"`
	TestFullWallMS       int    `json:"test_full_wall_ms"`
	TestFullPeakRSSKib   int    `json:"test_full_peak_rss_kib"`
	TestSelectedWallMS   int    `json:"test_selected_wall_ms"`
	TestSelectedPeakRSSKib int  `json:"test_selected_peak_rss_kib"`
	ConformanceWallMS    int    `json:"conformance_wall_ms"`
	ConformancePeakRSSKib int   `json:"conformance_peak_rss_kib"`
	SavedTestWallMS      any    `json:"saved_test_wall_ms"`
	ExactPair            bool   `json:"exact_pair"`
}

type ReportAuthority struct {
	RepositoryWrites int    `json:"repository_writes"`
	RootReadmePolicy string `json:"root_readme_policy"`
	ObservationMode  string `json:"observation_mode"`
}

type ArtifactInventory struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

type ActivityEvidence struct {
	CellID          string   `json:"cell_id"`
	Activity        string   `json:"activity"`
	GoooSource      string   `json:"gooo_source"`
	SemanticGraph   string   `json:"semantic_graph"`
	BindingDigest   string   `json:"binding_digest"`
	ClaimEvidence   []string `json:"claim_evidence"`
	GeneratedArtifact string  `json:"generated_artifact"`
	Evaluator       string   `json:"evaluator"`
}

func (i Input) MarshalJSON() ([]byte, error) {
	type alias Input
	return json.Marshal(alias(i))
}
