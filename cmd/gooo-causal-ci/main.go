package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kimjooyoon/gooo-causal-ci/internal/causalci"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: gooo-causal-ci prepare|plan|evaluate")
		return 2
	}
	switch args[0] {
	case "prepare":
		return prepare(args[1:], stdout, stderr)
	case "plan":
		return plan(args[1:], stdout, stderr)
	case "evaluate":
		return evaluate(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func prepare(args []string, stdout, stderr io.Writer) int {
	values, ok := parseFlags(args, stderr, map[string]bool{"source": true, "semantic": true, "graph": true, "contract": true, "claims": true, "tests": true, "changes": true, "toolchain-digest": true, "subject": true, "output": true})
	if !ok {
		return 2
	}
	input, err := causalci.Prepare(values["source"], values["semantic"], values["graph"], values["contract"], values["claims"], values["tests"], values["changes"], values["toolchain-digest"], values["subject"])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := writeJSON(values["output"], input); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "prepared source=%s semantic=%s graph=%s bindings=%d claims=%d tests=%d\n", input.SourceBinding.SourceDigest, input.SourceBinding.SemanticDigest, input.SourceBinding.GraphDigest, len(input.Bindings), len(input.Claims), len(input.Tests))
	return 0
}

func plan(args []string, stdout, stderr io.Writer) int {
	values, ok := parseFlags(args, stderr, map[string]bool{"input": true, "contract": true, "output": true})
	if !ok {
		return 2
	}
	var input causalci.Input
	var denominator causalci.Denominator
	if err := readJSON(values["input"], &input); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := readJSON(values["contract"], &denominator); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result := causalci.BuildPlan(input, denominator)
	if err := writeJSON(values["output"], result); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "plan decision=%s selection=%s closed=%d unknown=%d refuted=%d execute=%d reuse=%d skip=%d\n", result.Decision, result.SelectionMode, result.Metrics.Closed, result.Metrics.Unknown, result.Metrics.Refuted, result.Metrics.Executed, result.Metrics.Reused, result.Metrics.Skipped)
	return 0
}

func evaluate(args []string, stdout, stderr io.Writer) int {
	values, ok := parseFlags(args, stderr, map[string]bool{"plan": true, "runtime": true, "contract": true, "output": true})
	if !ok {
		return 2
	}
	var planValue causalci.Plan
	var runtime causalci.Runtime
	var denominator causalci.Denominator
	if err := readJSON(values["plan"], &planValue); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := readJSON(values["runtime"], &runtime); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := readJSON(values["contract"], &denominator); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	report, err := causalci.Evaluate(planValue, runtime, denominator, values["output"])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "report decision=%s denominator=%d/%d full=%d selected=%d/%d/%d saved_test_wall_ms=%v cases=%d artifacts=%d/%d\n", report.Decision, report.Metrics.Closed, report.Metrics.Denominator, report.Tests.Full.Executed, report.Tests.Selected.Executed, report.Tests.Selected.Reused, report.Tests.Selected.Skipped, report.Performance.SavedTestWallMS, len(report.Cases), report.Artifacts.Files, report.Artifacts.Bytes)
	return 0
}

func parseFlags(args []string, stderr io.Writer, known map[string]bool) (map[string]string, bool) {
	values := map[string]string{}
	for index := 0; index < len(args); index++ {
		value := args[index]
		if !strings.HasPrefix(value, "--") || index+1 >= len(args) {
			fmt.Fprintf(stderr, "expected --name value, got %q\n", value)
			return nil, false
		}
		name := strings.TrimPrefix(value, "--")
		if !known[name] {
			fmt.Fprintf(stderr, "unknown flag %q\n", value)
			return nil, false
		}
		index++
		values[name] = args[index]
	}
	for name := range known {
		if values[name] == "" {
			fmt.Fprintf(stderr, "missing --%s\n", name)
			return nil, false
		}
	}
	return values, true
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}
