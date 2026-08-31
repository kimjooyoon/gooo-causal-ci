package causalci

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const evaluatorID = "github.com/kimjooyoon/gooo-causal-ci/internal/causalci"

type semanticObservation struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	SemanticHash  string `json:"semantic_hash"`
}

type graphObservation struct {
	SchemaVersion string `json:"schema_version"`
	GraphHash     string `json:"graph_hash"`
	SourceDigest  string `json:"source_digest"`
	IR            struct {
		Status         string `json:"status"`
		SemanticDigest string `json:"semantic_digest"`
	} `json:"ir"`
	Nodes []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"nodes"`
}

type rawClaims struct {
	Schema string     `json:"schema"`
	Claims []rawClaim `json:"claims"`
}

type rawClaim struct {
	ID           string              `json:"id"`
	Subject      string              `json:"subject"`
	ActivityPath []string            `json:"activity_path"`
	Exclusions   []ExclusionEvidence `json:"exclusions"`
}

type rawChanges struct {
	Schema   string   `json:"schema"`
	ClaimIDs []string `json:"claim_ids"`
}

type rawTests struct {
	Schema      string           `json:"schema"`
	FullCommand []string         `json:"full_command"`
	Tests       []TestDefinition `json:"tests"`
}

// Prepare binds the raw Gooo compiler outputs and raw claim/test evidence into
// an input for the planner. The planner never derives work from file names.
func Prepare(sourcePath, semanticPath, graphPath, contractPath, claimsPath, testsPath, changesPath, toolchainDigest, subject string) (Input, error) {
	contractRaw, err := os.ReadFile(contractPath)
	if err != nil {
		return Input{}, fmt.Errorf("read denominator: %w", err)
	}
	var denominator Denominator
	if err := decodeJSON(contractRaw, &denominator); err != nil {
		return Input{}, fmt.Errorf("decode denominator: %w", err)
	}
	if err := ValidateDenominator(denominator); err != nil {
		return Input{}, err
	}

	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return Input{}, fmt.Errorf("read Gooo source: %w", err)
	}
	semanticRaw, err := os.ReadFile(semanticPath)
	if err != nil {
		return Input{}, fmt.Errorf("read semantic receipt: %w", err)
	}
	var semantic semanticObservation
	if err := decodeLooseJSON(semanticRaw, &semantic); err != nil {
		return Input{}, fmt.Errorf("decode semantic receipt: %w", err)
	}
	graphRaw, err := os.ReadFile(graphPath)
	if err != nil {
		return Input{}, fmt.Errorf("read semantic graph: %w", err)
	}
	var graph graphObservation
	if err := decodeLooseJSON(graphRaw, &graph); err != nil {
		return Input{}, fmt.Errorf("decode semantic graph: %w", err)
	}

	sourceDigest := digestBytes(source)
	semanticDigest, err := normalizeDigest(semantic.SemanticHash)
	if err != nil {
		return Input{}, fmt.Errorf("semantic digest: %w", err)
	}
	graphSourceDigest, err := normalizeDigest(graph.SourceDigest)
	if err != nil {
		return Input{}, fmt.Errorf("graph source digest: %w", err)
	}
	graphSemanticDigest, err := normalizeDigest(graph.IR.SemanticDigest)
	if err != nil {
		return Input{}, fmt.Errorf("graph semantic digest: %w", err)
	}
	graphDigest, err := normalizeDigest(graph.GraphHash)
	if err != nil {
		return Input{}, fmt.Errorf("graph digest: %w", err)
	}
	toolchain, err := normalizeDigest(toolchainDigest)
	if err != nil {
		return Input{}, fmt.Errorf("toolchain digest: %w", err)
	}
	if semantic.Status != "ok" || semantic.SchemaVersion != "gooo/diagnostics/v1" {
		return Input{}, fmt.Errorf("semantic receipt is not successful")
	}
	if graph.SchemaVersion != "gooo-graph/v1" || graph.IR.Status != "available" {
		return Input{}, fmt.Errorf("semantic graph is not available")
	}
	if sourceDigest != graphSourceDigest || semanticDigest != graphSemanticDigest {
		return Input{}, fmt.Errorf("source and semantic graph digests disagree")
	}

	activities := make([]GraphActivity, 0)
	seenActivities := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Kind != "Activity" {
			continue
		}
		if node.ID == "" || node.Name == "" || seenActivities[node.Name] {
			return Input{}, fmt.Errorf("semantic graph has malformed or duplicate activity")
		}
		seenActivities[node.Name] = true
		activities = append(activities, GraphActivity{ID: node.ID, Name: node.Name})
	}
	sort.Slice(activities, func(i, j int) bool { return activities[i].Name < activities[j].Name })

	claims, err := readClaims(claimsPath)
	if err != nil {
		return Input{}, err
	}
	changes, err := readChanges(changesPath)
	if err != nil {
		return Input{}, err
	}
	tests, fullCommand, err := readTests(testsPath)
	if err != nil {
		return Input{}, err
	}
	contractDigest := digestBytes(contractRaw)
	input := Input{
		Schema:         InputSchema,
		Subject:        Subject{Repository: "kimjooyoon/gooo-causal-ci", Commit: subject},
		ContractDigest: contractDigest,
		SourceBinding: SourceBinding{
			Path:           sourcePath,
			SourceDigest:   sourceDigest,
			SemanticDigest: semanticDigest,
			GraphDigest:    graphDigest,
		},
		SemanticGraph: SemanticGraph{
			SchemaVersion:  graph.SchemaVersion,
			SourceDigest:   graphSourceDigest,
			SemanticDigest: graphSemanticDigest,
			GraphDigest:    graphDigest,
			Activities:     activities,
		},
		FullCommand:     fullCommand,
		Authority:       Authority{RepositoryWrites: 0, MutationAllowed: false},
		ControlDecision: "PLAN",
	}
	for _, cell := range denominator.Cells {
		var graphID string
		for _, activity := range activities {
			if activity.Name == cell.Activity {
				graphID = activity.ID
				break
			}
		}
		if graphID == "" {
			return Input{}, fmt.Errorf("semantic graph is missing activity %q", cell.Activity)
		}
		binding := ActivityBinding{Activity: cell.Activity, GraphNodeID: graphID, SourceDigest: sourceDigest, SemanticDigest: semanticDigest}
		binding.BindingDigest = digestBinding(binding)
		input.Bindings = append(input.Bindings, binding)
	}
	for _, claim := range claims {
		claim.SourceDigest = sourceDigest
		claim.SemanticDigest = semanticDigest
		claim.GraphDigest = graphDigest
		claim.EvidenceDigest = digestClaim(claim)
		input.Claims = append(input.Claims, claim)
	}
	input.Changes = changes
	for index := range tests {
		if tests[index].ReuseCandidate {
			tests[index].PriorReceipt = &PriorReceipt{
				Status:          "PASS",
				SourceDigest:    sourceDigest,
				SemanticDigest:  semanticDigest,
				GraphDigest:     graphDigest,
				ContractDigest:  contractDigest,
				ToolchainDigest: toolchain,
				ResultDigest:    digestBytes([]byte("fixture-prior-result\x00" + tests[index].ID + "\x00" + sourceDigest + "\x00" + semanticDigest)),
			}
		}
	}
	input.Tests = tests
	if err := validateInputShape(input, denominator); err != nil {
		return Input{}, err
	}
	return input, nil
}

func readClaims(path string) ([]ClaimEvidence, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read claims: %w", err)
	}
	var value rawClaims
	if err := decodeJSON(raw, &value); err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}
	if value.Schema != "gooo/causal-ci/claim-evidence/v1" || len(value.Claims) == 0 {
		return nil, fmt.Errorf("claim evidence is empty or malformed")
	}
	result := make([]ClaimEvidence, 0, len(value.Claims))
	seen := map[string]bool{}
	for _, claim := range value.Claims {
		if claim.ID == "" || claim.Subject == "" || len(claim.ActivityPath) == 0 || seen[claim.ID] {
			return nil, fmt.Errorf("claim evidence has a malformed or duplicate claim")
		}
		seen[claim.ID] = true
		result = append(result, ClaimEvidence{ID: claim.ID, Subject: claim.Subject, ActivityPath: append([]string(nil), claim.ActivityPath...), Exclusions: append([]ExclusionEvidence(nil), claim.Exclusions...)})
	}
	return result, nil
}

func readChanges(path string) ([]Change, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read changes: %w", err)
	}
	var value rawChanges
	if err := decodeJSON(raw, &value); err != nil {
		return nil, fmt.Errorf("decode changes: %w", err)
	}
	if value.Schema != "gooo/causal-ci/change-observation/v1" || len(value.ClaimIDs) == 0 {
		return nil, fmt.Errorf("change observation is empty or malformed")
	}
	result := make([]Change, 0, len(value.ClaimIDs))
	seen := map[string]bool{}
	for _, id := range value.ClaimIDs {
		if id == "" || seen[id] {
			return nil, fmt.Errorf("change observation has a malformed or duplicate claim")
		}
		seen[id] = true
		result = append(result, Change{ID: "change:" + id, ClaimID: id})
	}
	return result, nil
}

func readTests(path string) ([]TestDefinition, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read tests: %w", err)
	}
	var value rawTests
	if err := decodeJSON(raw, &value); err != nil {
		return nil, nil, fmt.Errorf("decode tests: %w", err)
	}
	if value.Schema != "gooo/causal-ci/test-registry/v1" || len(value.FullCommand) == 0 || len(value.Tests) == 0 {
		return nil, nil, fmt.Errorf("test registry is empty or malformed")
	}
	seen := map[string]bool{}
	for _, test := range value.Tests {
		if test.ID == "" || len(test.Command) == 0 || len(test.Activities) == 0 || seen[test.ID] {
			return nil, nil, fmt.Errorf("test registry has a malformed or duplicate test")
		}
		seen[test.ID] = true
	}
	return value.Tests, value.FullCommand, nil
}

// Plan generates CI activities from the semantic graph and claim paths.
func BuildPlan(input Input, denominator Denominator) Plan {
	inputDigest := digestJSON(input)
	plan := Plan{
		Schema:         PlanSchema,
		Decision:       DecisionUnknown,
		SelectionMode:  "NO_EXECUTION",
		ContractDigest: input.ContractDigest,
		InputDigest:    inputDigest,
		SourceBinding:  input.SourceBinding,
		Activities:     activityReceipts(input, denominator, StateUnknown),
		Tests:          unknownTestActions(input.Tests),
		Claim:          stateUnknown("CAUSAL_SELECTION", "validate-input", "INPUT_VALIDATION_NOT_COMPLETE", "DIRECT_MISSING", "REPAIR_CAUSAL_INPUT", []string{"source-binding", "semantic-graph", "claim-evidence"}),
		Metrics:        PlanMetrics{Denominator: denominator.Total},
	}
	if err := ValidateDenominator(denominator); err != nil {
		return failClosedPlan(plan, "CONFORMANCE", "validate-denominator", "MALFORMED_DENOMINATOR", "REJECT_MALFORMED_DENOMINATOR")
	}
	if state, ok := validateInput(input, denominator); !ok {
		if state.State == StateRefuted {
			return failClosedPlan(plan, state.Stage, state.Step, state.Reason, state.NextOperation)
		}
		plan.Unknowns = []CausalState{state}
		plan.Claim = state
		plan.Metrics.Unknown = denominator.Total
		return plan
	}

	claims, affected, err := activeClaims(input)
	if err != nil {
		return failClosedPlan(plan, "CAUSAL_SELECTION", "validate-claim-path", "CLAIM_PATH_CONTRADICTED", "REJECT_CONTRADICTED_CLAIM_PATH")
	}
	actions := make([]TestAction, 0, len(input.Tests))
	for _, test := range input.Tests {
		if intersects(test.Activities, affected) {
			action := ActionExecute
			reason := "CLAIM_ACTIVITY_PATH_INTERSECTS_TEST_ACTIVITY"
			if reusable(test.PriorReceipt, input) {
				action = ActionReuse
				reason = "EXACT_PRIOR_RECEIPT_BOUND_TO_CURRENT_SCOPE"
			}
			actions = append(actions, TestAction{ID: test.ID, Command: append([]string(nil), test.Command...), Action: action, CausalClaimIDs: claimIDs(claims), Reason: reason})
			continue
		}
		exclusions := exclusionsFor(claims, test.ID)
		if len(exclusions) != 1 || intersectsExclusion(exclusions[0], affected, test.Activities) {
			reason := "EXCLUSION_EVIDENCE_MISSING"
			if len(exclusions) == 1 {
				reason = "EXCLUSION_EVIDENCE_CONTRADICTS_CAUSAL_PATH"
			}
			state := stateRefuted("CAUSAL_SELECTION", "preserve-exclusion-evidence", reason, "REJECT_INCORRECT_EXCLUSION", []string{test.ID})
			return failClosedPlan(plan, state.Stage, state.Step, state.Reason, state.NextOperation)
		}
		actions = append(actions, TestAction{ID: test.ID, Command: append([]string(nil), test.Command...), Action: ActionSkip, ExclusionEvidence: exclusions, Reason: exclusions[0].Reason})
	}
	if state, incorrect := incorrectProposedExclusion(input.ProposedActions, actions); incorrect {
		return failClosedPlan(plan, state.Stage, state.Step, state.Reason, state.NextOperation)
	}

	plan.Decision = DecisionClosed
	plan.SelectionMode = "CAUSAL_SELECT"
	plan.Activities = activityReceipts(input, denominator, StateClosed)
	plan.Tests = actions
	plan.Claim = CausalState{State: StateClosed, Stage: "CAUSAL_SELECTION", Step: "generate-activities", Reason: "CAUSAL_TEST_ACTIVITY_SET_GENERATED", UnknownClass: "", NextOperation: "NONE", BlockedBy: []string{}}
	plan.Metrics.Closed = denominator.Total
	for _, action := range actions {
		switch action.Action {
		case ActionExecute:
			plan.Metrics.Executed++
		case ActionReuse:
			plan.Metrics.Reused++
		case ActionSkip:
			plan.Metrics.Skipped++
		}
	}
	return plan
}

func incorrectProposedExclusion(proposed, actual []TestAction) (CausalState, bool) {
	if len(proposed) == 0 {
		return CausalState{}, false
	}
	actualByID := map[string]TestAction{}
	for _, action := range actual {
		actualByID[action.ID] = action
	}
	for _, candidate := range proposed {
		resolved, ok := actualByID[candidate.ID]
		if !ok {
			return stateRefuted("CAUSAL_SELECTION", "adjudicate-proposed-actions", "PROPOSED_TEST_NOT_IN_REGISTRY", "REJECT_PROPOSED_ACTIONS", []string{candidate.ID}), true
		}
		if candidate.Action == ActionSkip && resolved.Action != ActionSkip {
			return stateRefuted("CAUSAL_SELECTION", "adjudicate-proposed-actions", "INCORRECT_TEST_EXCLUSION_REFUTED", "REJECT_INCORRECT_EXCLUSION", []string{candidate.ID}), true
		}
	}
	return CausalState{}, false
}

func activeClaims(input Input) ([]ClaimEvidence, map[string]bool, error) {
	claimByID := map[string]ClaimEvidence{}
	for _, claim := range input.Claims {
		claimByID[claim.ID] = claim
	}
	activities := map[string]bool{}
	var active []ClaimEvidence
	for _, change := range input.Changes {
		claim, ok := claimByID[change.ClaimID]
		if !ok {
			return nil, nil, fmt.Errorf("missing changed claim %q", change.ClaimID)
		}
		active = append(active, claim)
		for _, activity := range claim.ActivityPath {
			activities[activity] = true
		}
	}
	if len(active) == 0 {
		return nil, nil, fmt.Errorf("no active claims")
	}
	return active, activities, nil
}

func validateInput(input Input, denominator Denominator) (CausalState, bool) {
	if input.Schema != InputSchema || input.Subject.Repository == "" || input.Subject.Commit == "" || input.SourceBinding.Path == "" {
		return stateRefuted("CONFORMANCE", "validate-input", "MALFORMED_INPUT", "REJECT_MALFORMED_INPUT", []string{"input"}), false
	}
	if input.ControlDecision != "PLAN" {
		return stateRefuted("CONFORMANCE", "validate-control-decision", "UNRECOGNIZED_DECISION_"+input.ControlDecision, "REJECT_UNRECOGNIZED_DECISION", []string{"control-decision"}), false
	}
	if input.Authority.RepositoryWrites != 0 || input.Authority.MutationAllowed {
		return stateRefuted("AUTHORITY", "validate-mutation-authority", "REPOSITORY_WRITE_AUTHORITY_ESCALATED", "FAIL_CLOSED_ON_AUTHORITY_ESCALATION", []string{"authority"}), false
	}
	if !validDigest(input.ContractDigest) || !validDigest(input.SourceBinding.SourceDigest) || !validDigest(input.SourceBinding.SemanticDigest) || !validDigest(input.SourceBinding.GraphDigest) {
		return stateUnknown("CONFORMANCE", "validate-digest-shapes", "REQUIRED_DIGEST_NOT_OBSERVED", "LOWER_RESOLUTION", "PROVIDE_SOURCE_GRAPH_DIGESTS", []string{"source-binding", "semantic-graph"}), false
	}
	graph := input.SemanticGraph
	if graph.SchemaVersion != "gooo-graph/v1" || graph.SourceDigest != input.SourceBinding.SourceDigest || graph.SemanticDigest != input.SourceBinding.SemanticDigest || graph.GraphDigest != input.SourceBinding.GraphDigest {
		return stateUnknown("CONFORMANCE", "bind-semantic-graph", "STALE_SEMANTIC_GRAPH", "STALE_EVIDENCE", "REGENERATE_SEMANTIC_GRAPH", []string{"semantic-graph"}), false
	}
	if err := validateInputShape(input, denominator); err != nil {
		return stateUnknown("CONFORMANCE", "bind-source-and-claims", "MISSING_OR_INVALID_ACTIVITY_BINDING", "DIRECT_MISSING", "REGENERATE_ACTIVITY_BINDINGS", []string{"bindings", "claims"}), false
	}
	if _, _, err := activeClaims(input); err != nil {
		return stateUnknown("CAUSAL_SELECTION", "observe-change-claim", "CHANGED_CLAIM_EVIDENCE_UNAVAILABLE", "DIRECT_MISSING", "PROVIDE_CHANGED_CLAIM_EVIDENCE", []string{"changes", "claims"}), false
	}
	return CausalState{}, true
}

func validateInputShape(input Input, denominator Denominator) error {
	if len(input.Bindings) != len(denominator.Cells) || len(input.Claims) == 0 || len(input.Tests) == 0 || len(input.FullCommand) == 0 {
		return fmt.Errorf("input cardinality is incomplete")
	}
	activities := map[string]GraphActivity{}
	for _, activity := range input.SemanticGraph.Activities {
		if activity.Name == "" || activity.ID == "" || activities[activity.Name].Name != "" {
			return fmt.Errorf("semantic graph activity inventory is malformed")
		}
		activities[activity.Name] = activity
	}
	bindings := map[string]ActivityBinding{}
	for _, binding := range input.Bindings {
		if binding.Activity == "" || binding.GraphNodeID == "" || bindings[binding.Activity].Activity != "" {
			return fmt.Errorf("activity binding inventory is malformed")
		}
		graphActivity, ok := activities[binding.Activity]
		if !ok || graphActivity.ID != binding.GraphNodeID || binding.SourceDigest != input.SourceBinding.SourceDigest || binding.SemanticDigest != input.SourceBinding.SemanticDigest || binding.BindingDigest != digestBinding(ActivityBinding{Activity: binding.Activity, GraphNodeID: binding.GraphNodeID, SourceDigest: binding.SourceDigest, SemanticDigest: binding.SemanticDigest}) {
			return fmt.Errorf("activity binding digest or graph node mismatch")
		}
		bindings[binding.Activity] = binding
	}
	for _, cell := range denominator.Cells {
		if _, ok := bindings[cell.Activity]; !ok {
			return fmt.Errorf("denominator activity binding missing")
		}
	}
	claimIDs := map[string]bool{}
	for _, claim := range input.Claims {
		if claim.ID == "" || claim.SourceDigest != input.SourceBinding.SourceDigest || claim.SemanticDigest != input.SourceBinding.SemanticDigest || claim.GraphDigest != input.SourceBinding.GraphDigest || claim.EvidenceDigest != digestClaim(claim) {
			return fmt.Errorf("claim evidence digest mismatch")
		}
		if claimIDs[claim.ID] {
			return fmt.Errorf("duplicate claim evidence")
		}
		claimIDs[claim.ID] = true
		for _, activity := range claim.ActivityPath {
			if _, ok := activities[activity]; !ok {
				return fmt.Errorf("claim path activity missing from semantic graph")
			}
		}
	}
	for _, change := range input.Changes {
		if !claimIDs[change.ClaimID] {
			return fmt.Errorf("change references missing claim")
		}
	}
	testIDs := map[string]bool{}
	for _, test := range input.Tests {
		if test.ID == "" || testIDs[test.ID] || len(test.Command) == 0 || len(test.Activities) == 0 {
			return fmt.Errorf("test registry is malformed")
		}
		testIDs[test.ID] = true
		for _, activity := range test.Activities {
			if _, ok := activities[activity]; !ok {
				return fmt.Errorf("test activity is not in semantic graph")
			}
		}
	}
	return nil
}

func ValidateDenominator(value Denominator) error {
	if value.Schema != DenominatorSchema || value.Total != 12 || len(value.Cells) != value.Total {
		return fmt.Errorf("malformed fixed denominator")
	}
	proofs := map[string]int{}
	indicators := map[string]int{}
	seenIDs := map[string]bool{}
	seenActivities := map[string]bool{}
	for _, cell := range value.Cells {
		if cell.ID == "" || cell.Activity == "" || cell.Stage == "" || cell.Step == "" || cell.ProofChoice == "" || cell.IndicatorClass == "" || seenIDs[cell.ID] || seenActivities[cell.Activity] {
			return fmt.Errorf("malformed denominator cell")
		}
		seenIDs[cell.ID] = true
		seenActivities[cell.Activity] = true
		proofs[cell.ProofChoice]++
		indicators[cell.IndicatorClass]++
	}
	if len(value.ProofTotals) != 3 || len(value.IndicatorTotals) != 3 {
		return fmt.Errorf("denominator buckets are incomplete")
	}
	for _, bucket := range value.ProofTotals {
		if bucket.Total != 4 || proofs[bucket.Name] != 4 {
			return fmt.Errorf("proof denominator is not balanced")
		}
	}
	for _, bucket := range value.IndicatorTotals {
		if bucket.Total != 4 || indicators[bucket.Name] != 4 {
			return fmt.Errorf("indicator denominator is not balanced")
		}
	}
	return nil
}

func activityReceipts(input Input, denominator Denominator, state string) []ActivityReceipt {
	byActivity := map[string]ActivityBinding{}
	for _, binding := range input.Bindings {
		byActivity[binding.Activity] = binding
	}
	result := make([]ActivityReceipt, 0, len(denominator.Cells))
	for _, cell := range denominator.Cells {
		binding := byActivity[cell.Activity]
		result = append(result, ActivityReceipt{CellID: cell.ID, Activity: cell.Activity, State: state, ProofChoice: cell.ProofChoice, IndicatorClass: cell.IndicatorClass, SourcePath: input.SourceBinding.Path, GraphNodeID: binding.GraphNodeID, BindingDigest: binding.BindingDigest, ArtifactPath: "activity-plan.json", Evaluator: evaluatorID})
	}
	return result
}

func unknownTestActions(tests []TestDefinition) []TestAction {
	result := make([]TestAction, 0, len(tests))
	for _, test := range tests {
		result = append(result, TestAction{ID: test.ID, Action: ActionUnknown, Reason: "CAUSAL_PLAN_NOT_AUTHORIZED"})
	}
	return result
}

func reusable(receipt *PriorReceipt, input Input) bool {
	return receipt != nil && receipt.Status == "PASS" && receipt.SourceDigest == input.SourceBinding.SourceDigest && receipt.SemanticDigest == input.SourceBinding.SemanticDigest && receipt.GraphDigest == input.SourceBinding.GraphDigest && receipt.ContractDigest == input.ContractDigest && validDigest(receipt.ToolchainDigest) && receipt.ResultDigest != ""
}

func exclusionsFor(claims []ClaimEvidence, testID string) []ExclusionEvidence {
	var result []ExclusionEvidence
	for _, claim := range claims {
		for _, exclusion := range claim.Exclusions {
			if exclusion.TestID == testID {
				result = append(result, exclusion)
			}
		}
	}
	return result
}

func intersects(left []string, right map[string]bool) bool {
	for _, value := range left {
		if right[value] {
			return true
		}
	}
	return false
}

func intersectsExclusion(exclusion ExclusionEvidence, affected map[string]bool, testActivities []string) bool {
	if intersects(exclusion.ActivityPath, affected) || intersects(testActivities, affected) {
		return true
	}
	return false
}

func claimIDs(claims []ClaimEvidence) []string {
	result := make([]string, 0, len(claims))
	for _, claim := range claims {
		result = append(result, claim.ID)
	}
	sort.Strings(result)
	return result
}

func failClosedPlan(plan Plan, stage, step, reason, nextOperation string) Plan {
	state := stateRefuted(stage, step, reason, nextOperation, []string{"input"})
	plan.Decision = DecisionRefuted
	plan.SelectionMode = "NO_EXECUTION"
	plan.Claim = state
	plan.Refutations = []CausalState{state}
	plan.Unknowns = nil
	plan.Metrics.Refuted = plan.Metrics.Denominator
	plan.Activities = activityReceiptsWithState(plan.Activities, StateRefuted)
	return plan
}

func activityReceiptsWithState(values []ActivityReceipt, state string) []ActivityReceipt {
	result := append([]ActivityReceipt(nil), values...)
	for index := range result {
		result[index].State = state
	}
	return result
}

func stateUnknown(stage, step, reason, class, next string, blocked []string) CausalState {
	return CausalState{State: StateUnknown, Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: blocked}
}

func stateRefuted(stage, step, reason, next string, blocked []string) CausalState {
	return CausalState{State: StateRefuted, Stage: stage, Step: step, Reason: reason, UnknownClass: "", NextOperation: next, BlockedBy: blocked}
}

func digestBinding(binding ActivityBinding) string {
	binding.BindingDigest = ""
	return digestJSON(binding)
}

func digestClaim(claim ClaimEvidence) string {
	claim.EvidenceDigest = ""
	return digestJSON(claim)
}

func digestJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "sha256:" + strings.Repeat("0", 64)
	}
	return digestBytes(raw)
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func normalizeDigest(value string) (string, error) {
	value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "sha256:")
	if len(value) != 64 {
		return "", fmt.Errorf("digest must have 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("digest is not hexadecimal")
	}
	return "sha256:" + value, nil
}

func validDigest(value string) bool {
	_, err := normalizeDigest(value)
	return err == nil
}

func decodeJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func decodeLooseJSON(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
}
