package causalci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Evaluate binds the observed full and selected fixture runs to one generated
// plan. A timing integer is emitted only when the caller supplies an exact
// input/toolchain/contract pair.
func Evaluate(plan Plan, runtime Runtime, denominator Denominator, outputDir string) (Report, error) {
	if err := ValidateDenominator(denominator); err != nil {
		return Report{}, err
	}
	if plan.Decision != DecisionClosed {
		return Report{}, fmt.Errorf("only a CLOSED plan can be evaluated")
	}
	if err := validateRuntime(plan, runtime); err != nil {
		return Report{}, err
	}
	if err := prepareOutput(outputDir); err != nil {
		return Report{}, err
	}

	fullCounts := countStatuses(runtime.TestFull.Observations)
	selectedCounts := countStatuses(runtime.TestSelected.Observations)
	paired := runtime.Pair.Exact && runtime.Pair.InputDigest == plan.InputDigest && runtime.Pair.ContractDigest == plan.ContractDigest && runtime.Pair.ToolchainDigest == runtime.Toolchain.GoooDigest
	var saved any = "UNKNOWN"
	if paired {
		saved = runtime.TestFull.WallMS - runtime.TestSelected.WallMS
	}

	report := Report{
		Schema:        ReportSchema,
		Decision:      "CAUSAL_CI_REPORT_CLOSED",
		Claim:         plan.Claim,
		SourceBinding: plan.SourceBinding,
		Metrics: ReportMetrics{
			Denominator: plan.Metrics.Denominator,
			Closed:      plan.Metrics.Closed,
			Unknown:     plan.Metrics.Unknown,
			Refuted:     plan.Metrics.Refuted,
			Executed:    selectedCounts.Executed,
			Reused:      selectedCounts.Reused,
			Skipped:     selectedCounts.Skipped,
		},
		Tests:     ReportTests{Total: len(plan.Tests), Full: fullCounts, Selected: selectedCounts},
		Inventory: runtime.Inventory,
		Performance: ReportPerformance{
			BuildWallMS:            runtime.Build.WallMS,
			BuildPeakRSSKib:        runtime.Build.PeakRSSKib,
			TestFullWallMS:         runtime.TestFull.WallMS,
			TestFullPeakRSSKib:     runtime.TestFull.PeakRSSKib,
			TestSelectedWallMS:     runtime.TestSelected.WallMS,
			TestSelectedPeakRSSKib: runtime.TestSelected.PeakRSSKib,
			ConformanceWallMS:      runtime.Conformance.WallMS,
			ConformancePeakRSSKib:  runtime.Conformance.PeakRSSKib,
			SavedTestWallMS:        saved,
			ExactPair:              paired,
		},
		Authority: ReportAuthority{RepositoryWrites: runtime.Authority.RepositoryWrites, RootReadmePolicy: "EXCLUDED", ObservationMode: "READ_ONLY"},
		Cases:     append([]CaseResult(nil), runtime.Cases...),
		Evidence:  evidenceFromPlan(plan),
	}

	if err := writeJSONFile(filepath.Join(outputDir, "activity-plan.json"), plan); err != nil {
		return Report{}, err
	}
	if err := writeJSONFile(filepath.Join(outputDir, "cases.json"), runtime.Cases); err != nil {
		return Report{}, err
	}
	if err := writeJSONFile(filepath.Join(outputDir, "metrics.json"), struct {
		Metrics     ReportMetrics      `json:"metrics"`
		Performance ReportPerformance  `json:"performance"`
		Inventory   WorkspaceInventory `json:"inventory"`
	}{report.Metrics, report.Performance, report.Inventory}); err != nil {
		return Report{}, err
	}
	if err := writeText(filepath.Join(outputDir, "dossier.md"), dossier(report)); err != nil {
		return Report{}, err
	}

	for attempt := 0; attempt < 8; attempt++ {
		report.Digest = digestReport(report)
		if err := writeJSONFile(filepath.Join(outputDir, "report.json"), report); err != nil {
			return Report{}, err
		}
		files, totalBytes, err := artifactInventory(outputDir)
		if err != nil {
			return Report{}, err
		}
		if report.Artifacts.Files == files && report.Artifacts.Bytes == totalBytes {
			break
		}
		report.Artifacts = ArtifactInventory{Files: files, Bytes: totalBytes}
	}
	report.Digest = digestReport(report)
	if err := writeJSONFile(filepath.Join(outputDir, "report.json"), report); err != nil {
		return Report{}, err
	}
	if err := writeManifest(outputDir, report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func validateRuntime(plan Plan, runtime Runtime) error {
	if runtime.Schema != "gooo/causal-ci/runtime/v1" || runtime.Toolchain.GoVersion != "go1.27.0" || !validDigest(runtime.Toolchain.GoooDigest) {
		return fmt.Errorf("runtime toolchain evidence is invalid")
	}
	if runtime.Authority.RepositoryWrites != 0 || runtime.Authority.MutationAllowed {
		return fmt.Errorf("runtime authority is escalated")
	}
	for _, measurement := range []Measurement{runtime.Build, runtime.Conformance} {
		if measurement.Status != "PASS" || measurement.WallMS < 0 || measurement.PeakRSSKib < 0 {
			return fmt.Errorf("runtime measurement is invalid")
		}
	}
	if runtime.TestFull.Status != "PASS" || runtime.TestSelected.Status != "PASS" || runtime.TestFull.WallMS < 0 || runtime.TestSelected.WallMS < 0 || runtime.TestFull.PeakRSSKib < 0 || runtime.TestSelected.PeakRSSKib < 0 {
		return fmt.Errorf("test runtime evidence is invalid")
	}
	allIDs := map[string]bool{}
	for _, test := range plan.Tests {
		if test.ID == "" || allIDs[test.ID] {
			return fmt.Errorf("plan test inventory is malformed")
		}
		allIDs[test.ID] = true
	}
	if err := validateObservations(runtime.TestFull.Observations, allIDs, "EXECUTED"); err != nil {
		return fmt.Errorf("full test observation: %w", err)
	}
	expectedSelected := map[string]string{}
	for _, test := range plan.Tests {
		switch test.Action {
		case ActionExecute:
			expectedSelected[test.ID] = "EXECUTED"
		case ActionReuse:
			expectedSelected[test.ID] = "REUSED"
		case ActionSkip:
			expectedSelected[test.ID] = "SKIPPED"
		default:
			return fmt.Errorf("plan contains non-executable action")
		}
	}
	if err := validateObservations(runtime.TestSelected.Observations, expectedSelected, ""); err != nil {
		return fmt.Errorf("selected test observation: %w", err)
	}
	return nil
}

func validateObservations(observations []TestObservation, expected map[string]string, defaultStatus string) error {
	seen := map[string]bool{}
	if len(observations) != len(expected) {
		return fmt.Errorf("got %d observations, want %d", len(observations), len(expected))
	}
	for _, observation := range observations {
		want, ok := expected[observation.ID]
		if !ok || seen[observation.ID] {
			return fmt.Errorf("unexpected or duplicate test %q", observation.ID)
		}
		seen[observation.ID] = true
		if defaultStatus != "" {
			want = defaultStatus
		}
		if observation.Status != want {
			return fmt.Errorf("test %q status=%q want=%q", observation.ID, observation.Status, want)
		}
	}
	return nil
}

func countStatuses(values []TestObservation) TestCounts {
	counts := TestCounts{}
	for _, value := range values {
		switch value.Status {
		case "EXECUTED":
			counts.Executed++
		case "REUSED":
			counts.Reused++
		case "SKIPPED":
			counts.Skipped++
		}
	}
	return counts
}

func evidenceFromPlan(plan Plan) []ActivityEvidence {
	claims := make([]string, 0)
	for _, test := range plan.Tests {
		claims = append(claims, test.CausalClaimIDs...)
	}
	claims = uniqueSorted(claims)
	result := make([]ActivityEvidence, 0, len(plan.Activities))
	for _, activity := range plan.Activities {
		result = append(result, ActivityEvidence{CellID: activity.CellID, Activity: activity.Activity, GoooSource: activity.SourcePath, SemanticGraph: activity.GraphNodeID, BindingDigest: activity.BindingDigest, ClaimEvidence: claims, GeneratedArtifact: activity.ArtifactPath, Evaluator: activity.Evaluator})
	}
	return result
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func prepareOutput(path string) error {
	if path == "" {
		return fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory must be empty")
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeText(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o644)
}

func digestReport(report Report) string {
	report.Digest = ""
	return digestJSON(report)
}

func artifactInventory(root string) (int, int64, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, 0, err
	}
	count := 0
	var total int64
	for _, entry := range entries {
		if entry.Name() == "manifest.json" || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, 0, err
		}
		count++
		total += info.Size()
	}
	return count, total, nil
}

func writeManifest(root string, report Report) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	type manifestEntry struct {
		Path   string `json:"path"`
		Bytes  int64  `json:"bytes"`
		Digest string `json:"digest"`
	}
	files := make([]manifestEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == "manifest.json" || entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, manifestEntry{Path: entry.Name(), Bytes: int64(len(data)), Digest: digestBytes(data)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return writeJSONFile(filepath.Join(root, "manifest.json"), struct {
		Schema        string          `json:"schema"`
		ReportDigest  string          `json:"report_digest"`
		ArtifactFiles int             `json:"artifact_files"`
		ArtifactBytes int64           `json:"artifact_bytes"`
		Files         []manifestEntry `json:"files"`
	}{"gooo/causal-ci/artifact-manifest/v1", report.Digest, report.Artifacts.Files, report.Artifacts.Bytes, files})
}

func dossier(report Report) string {
	var b strings.Builder
	b.WriteString("# Gooo causal CI evidence\n\n")
	fmt.Fprintf(&b, "Decision: `%s`\n\n", report.Decision)
	fmt.Fprintf(&b, "Fixed denominator: `%d/%d CLOSED`, `%d UNKNOWN`, `%d REFUTED`.\n\n", report.Metrics.Closed, report.Metrics.Denominator, report.Metrics.Unknown, report.Metrics.Refuted)
	b.WriteString("## Test-set observation\n\n")
	fmt.Fprintf(&b, "Full set: executed=%d reused=%d skipped=%d; selected set: executed=%d reused=%d skipped=%d.\n\n", report.Tests.Full.Executed, report.Tests.Full.Reused, report.Tests.Full.Skipped, report.Tests.Selected.Executed, report.Tests.Selected.Reused, report.Tests.Selected.Skipped)
	fmt.Fprintf(&b, "Exact pair: `%t`; full test wall_ms=%d, selected test wall_ms=%d, saved_test_wall_ms=%v.\n\n", report.Performance.ExactPair, report.Performance.TestFullWallMS, report.Performance.TestSelectedWallMS, report.Performance.SavedTestWallMS)
	b.WriteString("## Resource and authority observation\n\n")
	fmt.Fprintf(&b, "build wall_ms=%d peak_rss_kib=%d; conformance wall_ms=%d peak_rss_kib=%d.\n", report.Performance.BuildWallMS, report.Performance.BuildPeakRSSKib, report.Performance.ConformanceWallMS, report.Performance.ConformancePeakRSSKib)
	fmt.Fprintf(&b, "repository_writes=%d; root README policy=%s; descendant_dirs=%d; regular_files=%d; Go files/lines=%d/%d; Gooo files/lines=%d/%d.\n\n", report.Authority.RepositoryWrites, report.Authority.RootReadmePolicy, report.Inventory.DescendantDirs, report.Inventory.RegularFiles, report.Inventory.GoFiles, report.Inventory.GoLines, report.Inventory.GoooFiles, report.Inventory.GoooLines)
	b.WriteString("## Cases\n\n")
	for _, value := range report.Cases {
		fmt.Fprintf(&b, "- `%s`: decision=%s state=%s stage=%s step=%s reason=%s\n", value.ID, value.Decision, value.State, value.Stage, value.Step, value.Reason)
	}
	b.WriteString("\n`artifact files/bytes` counts generated evidence files except this manifest; manifest.json lists every generated file and digest.\n")
	return b.String()
}
