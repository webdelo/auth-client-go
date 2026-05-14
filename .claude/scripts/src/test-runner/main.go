package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// TestEvent matches the JSON output format of `go test -json`.
type TestEvent struct {
	Time    string   `json:"Time"`
	Action  string   `json:"Action"`
	Package string   `json:"Package"`
	Test    string   `json:"Test"`
	Elapsed *float64 `json:"Elapsed"`
	Output  string   `json:"Output"`
}

// PackageResult holds per-package test statistics.
type PackageResult struct {
	Tests    int
	Passed   int
	Failed   int
	Skipped  int
	Coverage string
	Elapsed  string
}

// FailedTest stores accumulated output for a failed test.
type FailedTest struct {
	Package string
	Name    string
	Output  []string
}

// IntegrationResult holds per-parent-test statistics for integration mode.
type IntegrationResult struct {
	Cases   int
	Passed  int
	Failed  int
	Skipped int
	Elapsed string
}

// IntegrationFailedTest stores a failed subtest in integration mode.
type IntegrationFailedTest struct {
	ParentTest string
	SubTest    string
	Output     []string
}

// ANSI color codes.
const (
	red    = "\033[0;31m"
	green  = "\033[0;32m"
	yellow = "\033[0;33m"
	cyan   = "\033[0;36m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	nc     = "\033[0m"
)

// Table column widths.
const (
	colPkg     = 38
	colTests   = 7
	colPass    = 7
	colFail    = 7
	colSkip    = 7
	colTime    = 8
	colCov     = 9
	barWidth   = 40
	actionFail = "fail"
)

func main() {
	race := flag.Bool("race", false, "Enable race detector")
	verbose := flag.Bool("verbose", false, "Verbose mode")
	vShort := flag.Bool("v", false, "Verbose mode (short)")
	coverprofile := flag.String("coverprofile", "", "Write coverage profile to file")
	tags := flag.String("tags", "", "Build tags")
	title := flag.String("title", "", "Title header")
	exclude := flag.String("exclude", "", "Exclude packages matching pattern")
	integration := flag.Bool("integration", false, "Integration test output mode")
	flag.Parse()

	if *vShort {
		*verbose = true
	}

	testPath := "./..."
	if flag.NArg() > 0 {
		testPath = flag.Arg(0)
	}

	moduleName := detectModuleName()

	if *title != "" {
		header := fmt.Sprintf("━━━ %s ", *title)
		pad := 78 - len([]rune(header))
		if pad > 0 {
			header += strings.Repeat("━", pad)
		}
		fmt.Printf("%s%s%s%s\n\n", bold, cyan, header, nc)
	}

	modeDesc := "Running tests"
	if *verbose {
		modeDesc += " (verbose)"
	}
	if *race {
		modeDesc += " with race detector"
	}
	if *coverprofile != "" {
		modeDesc += " with coverage"
	}
	if *tags != "" {
		modeDesc += " [" + *tags + "]"
	}
	fmt.Printf("%s%s...%s\n\n", cyan, modeDesc, nc)

	var testTargets []string
	totalPackages := 0

	if *exclude != "" {
		testTargets = getFilteredPackages(testPath, *exclude, *tags)
		totalPackages = len(testTargets)
	} else {
		testTargets = []string{testPath}
		if !*integration {
			totalPackages = countPackages(testPath)
		}
	}

	baseFlags := buildFlags(*race, *coverprofile, *tags)

	testExit := 0

	if *verbose {
		args := append([]string{"test", "-v"}, baseFlags...)
		args = append(args, testTargets...)
		testExit = runVerbosePass(args)
		fmt.Println()
	}

	args := append([]string{"test", "-json"}, baseFlags...)
	args = append(args, testTargets...)
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "app_error creating pipe: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "app_error starting go test: %v\n", err)
		os.Exit(1)
	}

	if *integration {
		runIntegrationMode(cmd, stdout, *verbose, &testExit)
	} else {
		runUnitMode(cmd, stdout, *verbose, totalPackages, moduleName, *coverprofile, &testExit)
	}
}

// runUnitMode processes and renders unit test results.
func runUnitMode(cmd *exec.Cmd, stdout io.ReadCloser, verbose bool, totalPackages int, moduleName, coverprofile string, testExit *int) {
	pkgResults, pkgOrder, failedTests, eventsExit := processEvents(stdout, verbose, totalPackages, moduleName)
	_ = cmd.Wait()

	if eventsExit != 0 {
		*testExit = eventsExit
	}

	if !verbose {
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}
	fmt.Println()

	hasCov := coverprofile != ""
	totalTests, totalPassed, totalFailed, totalSkipped := printTable(pkgResults, pkgOrder, hasCov, coverprofile)
	fmt.Println()

	if !verbose && len(failedTests) > 0 {
		printFailedTests(failedTests)
	}

	printSummary(totalTests, totalPassed, totalFailed, totalSkipped, hasCov, coverprofile)

	if totalFailed > 0 || *testExit != 0 {
		os.Exit(1)
	}
}

// runIntegrationMode processes and renders integration test results.
func runIntegrationMode(cmd *exec.Cmd, stdout io.ReadCloser, verbose bool, testExit *int) {
	results, order, intFailedTests, eventsExit := processIntegrationEvents(stdout, verbose)
	_ = cmd.Wait()

	if eventsExit != 0 {
		*testExit = eventsExit
	}

	if !verbose {
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}
	fmt.Println()

	totalCases, totalPassed, totalFailed, totalSkipped := printIntegrationTable(results, order)
	fmt.Println()

	if !verbose && len(intFailedTests) > 0 {
		printIntegrationFailedTests(intFailedTests)
	}

	printIntegrationSummary(totalCases, totalPassed, totalFailed, totalSkipped)

	if totalFailed > 0 || *testExit != 0 {
		os.Exit(1)
	}
}

// runVerbosePass runs go test -v with live output and returns the exit code.
func runVerbosePass(args []string) int {
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// getFilteredPackages returns packages matching testPath, excluding those containing the exclude pattern.
func getFilteredPackages(testPath, exclude, tags string) []string {
	args := []string{"list"}
	if tags != "" {
		args = append(args, "-tags="+tags)
	}
	args = append(args, testPath)

	cmd := exec.Command("go", args...)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var packages []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		pkg := scanner.Text()
		if exclude != "" && strings.Contains(pkg, exclude) {
			continue
		}
		if strings.Contains(pkg, "/examples") {
			continue
		}
		packages = append(packages, pkg)
	}
	return packages
}

// processEvents reads go test -json output and accumulates per-package results.
func processEvents(
	stdout io.Reader,
	verbose bool,
	totalPackages int,
	moduleName string,
) (
	pkgResults map[string]*PackageResult,
	pkgOrder []string,
	failedTests []FailedTest,
	testExit int,
) {
	pkgResults = make(map[string]*PackageResult)
	testOutputs := make(map[string][]string)
	completedPackages := 0

	covRe := regexp.MustCompile(`coverage:\s+([0-9.]+)%`)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var ev TestEvent
		if json.Unmarshal(line, &ev) != nil {
			continue
		}

		if ev.Package == "" || strings.Contains(ev.Package, "/examples") {
			continue
		}

		shortPkg := strings.TrimPrefix(ev.Package, moduleName+"/")

		pr, exists := pkgResults[shortPkg]
		if !exists {
			pr = &PackageResult{Coverage: "-", Elapsed: "0.000s"}
			pkgResults[shortPkg] = pr
			pkgOrder = append(pkgOrder, shortPkg)
		}

		// Package-level events (no Test field).
		if ev.Test == "" {
			if ev.Elapsed != nil {
				pr.Elapsed = fmt.Sprintf("%.3fs", *ev.Elapsed)
			}
			switch ev.Action {
			case "pass", actionFail, "skip":
				completedPackages++
				if !verbose && totalPackages > 0 {
					drawProgress(completedPackages, totalPackages, shortPkg)
				}
				if ev.Action == actionFail {
					testExit = 1
				}
			}
		}

		// Per-test events.
		if ev.Test != "" {
			testKey := shortPkg + "::" + ev.Test
			switch ev.Action {
			case "run":
				pr.Tests++
				testOutputs[testKey] = nil
			case "output":
				testOutputs[testKey] = append(testOutputs[testKey], ev.Output)
			case "pass":
				pr.Passed++
				delete(testOutputs, testKey)
			case "skip":
				pr.Skipped++
				delete(testOutputs, testKey)
			case actionFail:
				pr.Failed++
				failedTests = append(failedTests, FailedTest{
					Package: shortPkg,
					Name:    ev.Test,
					Output:  testOutputs[testKey],
				})
				delete(testOutputs, testKey)
			}
		}

		// Coverage extraction from output.
		if ev.Output != "" {
			if m := covRe.FindStringSubmatch(ev.Output); m != nil {
				pr.Coverage = m[1] + "%"
			}
		}
	}

	return pkgResults, pkgOrder, failedTests, testExit
}

// processIntegrationEvents reads go test -json output and groups results by parent test function.
func processIntegrationEvents(
	stdout io.Reader,
	verbose bool,
) (
	results map[string]*IntegrationResult,
	order []string,
	failedTests []IntegrationFailedTest,
	testExit int,
) {
	results = make(map[string]*IntegrationResult)
	testOutputs := make(map[string][]string)
	completedTests := 0

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var ev TestEvent
		if json.Unmarshal(line, &ev) != nil {
			continue
		}

		// Package-level events — only check for failures.
		if ev.Test == "" {
			if ev.Action == actionFail {
				testExit = 1
			}
			continue
		}

		slashIdx := strings.Index(ev.Test, "/")

		if slashIdx == -1 {
			// Parent test (e.g. "TestBFFBalance").
			groupKey := strings.TrimPrefix(ev.Test, "Test")

			if ev.Action == "run" {
				if _, exists := results[groupKey]; !exists {
					results[groupKey] = &IntegrationResult{Elapsed: "0.000s"}
					order = append(order, groupKey)
				}
			}

			if r, exists := results[groupKey]; exists && ev.Elapsed != nil {
				r.Elapsed = fmt.Sprintf("%.3fs", *ev.Elapsed)
			}

			switch ev.Action {
			case "pass", actionFail, "skip":
				completedTests++
				if !verbose {
					drawIntegrationProgress(completedTests, groupKey)
				}
				if ev.Action == actionFail {
					testExit = 1
				}
			}
		} else {
			// Subtest (e.g. "TestBFFBalance/case_name_#1").
			parentTest := ev.Test[:slashIdx]
			groupKey := strings.TrimPrefix(parentTest, "Test")
			subtestName := ev.Test[slashIdx+1:]

			r, exists := results[groupKey]
			if !exists {
				r = &IntegrationResult{Elapsed: "0.000s"}
				results[groupKey] = r
				order = append(order, groupKey)
			}

			outputKey := ev.Test

			switch ev.Action {
			case "run":
				r.Cases++
				testOutputs[outputKey] = nil
			case "output":
				testOutputs[outputKey] = append(testOutputs[outputKey], ev.Output)
			case "pass":
				r.Passed++
				delete(testOutputs, outputKey)
			case "skip":
				r.Skipped++
				delete(testOutputs, outputKey)
			case actionFail:
				r.Failed++
				failedTests = append(failedTests, IntegrationFailedTest{
					ParentTest: groupKey,
					SubTest:    subtestName,
					Output:     testOutputs[outputKey],
				})
				delete(testOutputs, outputKey)
			}
		}
	}

	return results, order, failedTests, testExit
}

// printTable renders the unit test results table and returns totals.
func printTable(
	pkgResults map[string]*PackageResult,
	pkgOrder []string,
	hasCov bool,
	coverFile string,
) (totalTests, totalPassed, totalFailed, totalSkipped int) {
	drawLine("┌", "┬", "┐", hasCov)

	// Header row.
	fmt.Printf("%s│%s %s%-*s%s ", dim, nc, bold, colPkg, "Package", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colTests, "Tests", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colPass, "Passed", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colFail, "Failed", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colSkip, "Skip", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colTime, "Time", nc)
	if hasCov {
		fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colCov, "Coverage", nc)
	}
	fmt.Printf("%s│%s\n", dim, nc)

	drawLine("├", "┼", "┤", hasCov)

	// Data rows.
	for _, pkg := range pkgOrder {
		pr := pkgResults[pkg]
		totalTests += pr.Tests
		totalPassed += pr.Passed
		totalFailed += pr.Failed
		totalSkipped += pr.Skipped

		nameDisplay := pkg
		if len(pkg) > colPkg-2 {
			nameDisplay = pkg[:colPkg-5] + "..."
		}

		indicator := green + "\u2714" + nc
		if pr.Failed > 0 {
			indicator = red + "\u2718" + nc
		}

		fmt.Printf("%s│%s %s %-*s ", dim, nc, indicator, colPkg-2, nameDisplay)
		fmt.Printf("%s│%s %*d ", dim, nc, colTests, pr.Tests)
		fmt.Printf("%s│%s %s%*d%s ", dim, nc, green, colPass, pr.Passed, nc)

		if pr.Failed > 0 {
			fmt.Printf("%s│%s %s%s%*d%s ", dim, nc, red, bold, colFail, pr.Failed, nc)
		} else {
			fmt.Printf("%s│%s %s%*d%s ", dim, nc, dim, colFail, pr.Failed, nc)
		}

		if pr.Skipped > 0 {
			fmt.Printf("%s│%s %s%*d%s ", dim, nc, yellow, colSkip, pr.Skipped, nc)
		} else {
			fmt.Printf("%s│%s %s%*d%s ", dim, nc, dim, colSkip, pr.Skipped, nc)
		}

		fmt.Printf("%s│%s %s%*s%s ", dim, nc, dim, colTime, pr.Elapsed, nc)

		if hasCov {
			if pr.Coverage != "-" {
				fmt.Printf("%s│%s %s%*s%s ", dim, nc, cyan, colCov, pr.Coverage, nc)
			} else {
				fmt.Printf("%s│%s %s%*s%s ", dim, nc, dim, colCov, pr.Coverage, nc)
			}
		}
		fmt.Printf("%s│%s\n", dim, nc)
	}

	drawLine("├", "┼", "┤", hasCov)

	// Totals row.
	fmt.Printf("%s│%s %s%-*s%s ", dim, nc, bold, colPkg, "TOTAL", nc)
	fmt.Printf("%s│%s %s%*d%s ", dim, nc, bold, colTests, totalTests, nc)
	fmt.Printf("%s│%s %s%s%*d%s ", dim, nc, green, bold, colPass, totalPassed, nc)

	if totalFailed > 0 {
		fmt.Printf("%s│%s %s%s%*d%s ", dim, nc, red, bold, colFail, totalFailed, nc)
	} else {
		fmt.Printf("%s│%s %s%*d%s ", dim, nc, dim, colFail, totalFailed, nc)
	}

	if totalSkipped > 0 {
		fmt.Printf("%s│%s %s%*d%s ", dim, nc, yellow, colSkip, totalSkipped, nc)
	} else {
		fmt.Printf("%s│%s %s%*d%s ", dim, nc, dim, colSkip, totalSkipped, nc)
	}

	fmt.Printf("%s│%s %*s ", dim, nc, colTime, "")

	if hasCov {
		totalCov := getTotalCoverage(coverFile)
		fmt.Printf("%s│%s %s%s%*s%s ", dim, nc, cyan, bold, colCov, totalCov, nc)
	}
	fmt.Printf("%s│%s\n", dim, nc)

	drawLine("└", "┴", "┘", hasCov)

	return totalTests, totalPassed, totalFailed, totalSkipped
}

// printIntegrationTable renders the integration test results table.
func printIntegrationTable(
	results map[string]*IntegrationResult,
	order []string,
) (totalCases, totalPassed, totalFailed, totalSkipped int) {
	drawLine("┌", "┬", "┐", false)

	// Header row.
	fmt.Printf("%s│%s %s%-*s%s ", dim, nc, bold, colPkg, "Test", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colTests, "Cases", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colPass, "Passed", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colFail, "Failed", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colSkip, "Skip", nc)
	fmt.Printf("%s│%s %s%*s%s ", dim, nc, bold, colTime, "Time", nc)
	fmt.Printf("%s│%s\n", dim, nc)

	drawLine("├", "┼", "┤", false)

	// Data rows.
	for _, name := range order {
		r := results[name]
		totalCases += r.Cases
		totalPassed += r.Passed
		totalFailed += r.Failed
		totalSkipped += r.Skipped

		nameDisplay := name
		if len(name) > colPkg-2 {
			nameDisplay = name[:colPkg-5] + "..."
		}

		indicator := green + "\u2714" + nc
		if r.Failed > 0 {
			indicator = red + "\u2718" + nc
		}

		fmt.Printf("%s│%s %s %-*s ", dim, nc, indicator, colPkg-2, nameDisplay)
		fmt.Printf("%s│%s %*d ", dim, nc, colTests, r.Cases)
		fmt.Printf("%s│%s %s%*d%s ", dim, nc, green, colPass, r.Passed, nc)

		if r.Failed > 0 {
			fmt.Printf("%s│%s %s%s%*d%s ", dim, nc, red, bold, colFail, r.Failed, nc)
		} else {
			fmt.Printf("%s│%s %s%*d%s ", dim, nc, dim, colFail, r.Failed, nc)
		}

		if r.Skipped > 0 {
			fmt.Printf("%s│%s %s%*d%s ", dim, nc, yellow, colSkip, r.Skipped, nc)
		} else {
			fmt.Printf("%s│%s %s%*d%s ", dim, nc, dim, colSkip, r.Skipped, nc)
		}

		fmt.Printf("%s│%s %s%*s%s ", dim, nc, dim, colTime, r.Elapsed, nc)
		fmt.Printf("%s│%s\n", dim, nc)
	}

	drawLine("├", "┼", "┤", false)

	// Totals row.
	fmt.Printf("%s│%s %s%-*s%s ", dim, nc, bold, colPkg, "TOTAL", nc)
	fmt.Printf("%s│%s %s%*d%s ", dim, nc, bold, colTests, totalCases, nc)
	fmt.Printf("%s│%s %s%s%*d%s ", dim, nc, green, bold, colPass, totalPassed, nc)

	if totalFailed > 0 {
		fmt.Printf("%s│%s %s%s%*d%s ", dim, nc, red, bold, colFail, totalFailed, nc)
	} else {
		fmt.Printf("%s│%s %s%*d%s ", dim, nc, dim, colFail, totalFailed, nc)
	}

	if totalSkipped > 0 {
		fmt.Printf("%s│%s %s%*d%s ", dim, nc, yellow, colSkip, totalSkipped, nc)
	} else {
		fmt.Printf("%s│%s %s%*d%s ", dim, nc, dim, colSkip, totalSkipped, nc)
	}

	fmt.Printf("%s│%s %*s ", dim, nc, colTime, "")
	fmt.Printf("%s│%s\n", dim, nc)

	drawLine("└", "┴", "┘", false)

	return totalCases, totalPassed, totalFailed, totalSkipped
}

// printFailedTests renders the failed tests section for unit mode.
func printFailedTests(failedTests []FailedTest) {
	fmt.Printf("%s%s━━━ Failed Tests ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", red, bold, nc)
	fmt.Println()

	for _, ft := range failedTests {
		fmt.Printf("  %s\u2718%s %s%s%s\n", red, nc, bold, ft.Name, nc)
		fmt.Printf("    %sPackage: %s%s\n", dim, ft.Package, nc)

		if len(ft.Output) > 0 {
			fmt.Printf("    %sOutput:%s\n", dim, nc)
			printed := 0
			for _, line := range ft.Output {
				trimmed := strings.TrimRight(line, "\n")
				if strings.HasPrefix(trimmed, "=== ") || strings.HasPrefix(trimmed, "--- ") {
					continue
				}
				if printed >= 25 {
					break
				}
				fmt.Printf("      %s\n", trimmed)
				printed++
			}
		}
		fmt.Println()
	}
}

// printIntegrationFailedTests renders the failed tests section for integration mode.
func printIntegrationFailedTests(failedTests []IntegrationFailedTest) {
	fmt.Printf("%s%s━━━ Failed Tests ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", red, bold, nc)
	fmt.Println()

	for _, ft := range failedTests {
		displayName := strings.ReplaceAll(ft.SubTest, "_", " ")
		fmt.Printf("  %s\u2718%s %s%s%s\n", red, nc, bold, displayName, nc)
		fmt.Printf("    %sTest: %s%s\n", dim, ft.ParentTest, nc)

		if len(ft.Output) > 0 {
			fmt.Printf("    %sOutput:%s\n", dim, nc)
			printed := 0
			for _, line := range ft.Output {
				trimmed := strings.TrimRight(line, "\n")
				if strings.HasPrefix(trimmed, "=== ") || strings.HasPrefix(trimmed, "--- ") {
					continue
				}
				if printed >= 25 {
					break
				}
				fmt.Printf("      %s\n", trimmed)
				printed++
			}
		}
		fmt.Println()
	}
}

// printSummary renders the final summary line for unit mode.
func printSummary(totalTests, totalPassed, totalFailed, totalSkipped int, hasCov bool, coverFile string) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if totalFailed > 0 {
		fmt.Printf("%s%s  \u2718 FAILED%s  │  Tests: %d  │  Passed: %s%d%s  │  Failed: %s%s%d%s  │  Skipped: %s%d%s\n",
			red, bold, nc,
			totalTests,
			green, totalPassed, nc,
			red, bold, totalFailed, nc,
			yellow, totalSkipped, nc,
		)
	} else {
		fmt.Printf("%s%s  \u2714 PASSED%s  │  Tests: %d  │  Passed: %s%d%s  │  Failed: %s%d%s  │  Skipped: %s%d%s\n",
			green, bold, nc,
			totalTests,
			green, totalPassed, nc,
			dim, totalFailed, nc,
			yellow, totalSkipped, nc,
		)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if hasCov && fileExists(coverFile) {
		fmt.Println()
		fmt.Printf("%s\U0001f4c1 Coverage profile: %s%s\n", cyan, coverFile, nc)
	}
}

// printIntegrationSummary renders the final summary line for integration mode.
func printIntegrationSummary(totalCases, totalPassed, totalFailed, totalSkipped int) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if totalFailed > 0 {
		fmt.Printf("%s%s  \u2718 FAILED%s  │  Cases: %d  │  Passed: %s%d%s  │  Failed: %s%s%d%s  │  Skipped: %s%d%s\n",
			red, bold, nc,
			totalCases,
			green, totalPassed, nc,
			red, bold, totalFailed, nc,
			yellow, totalSkipped, nc,
		)
	} else {
		fmt.Printf("%s%s  \u2714 PASSED%s  │  Cases: %d  │  Passed: %s%d%s  │  Failed: %s%d%s  │  Skipped: %s%d%s\n",
			green, bold, nc,
			totalCases,
			green, totalPassed, nc,
			dim, totalFailed, nc,
			yellow, totalSkipped, nc,
		)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// detectModuleName reads the module name from go.mod.
func detectModuleName() string {
	f, err := os.Open("go.mod")
	if err != nil {
		return "unknown"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return "unknown"
}

// countPackages runs `go list` and counts the packages.
func countPackages(testPath string) int {
	cmd := exec.Command("go", "list", testPath)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "/examples") {
			count++
		}
	}
	return count
}

// buildFlags constructs the common flags for go test.
func buildFlags(race bool, coverprofile, tags string) []string {
	var flags []string
	if race {
		flags = append(flags, "-race")
	}
	if coverprofile != "" {
		flags = append(flags, "-coverprofile="+coverprofile)
	}
	if tags != "" {
		flags = append(flags, "-tags="+tags)
	}
	return flags
}

// drawProgress renders the progress bar on stderr for unit mode.
func drawProgress(completed, total int, pkg string) {
	percent := completed * 100 / total
	if percent > 100 {
		percent = 100
	}

	filled := percent * barWidth / 100
	empty := barWidth - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	maxPkgLen := 35
	if len(pkg) > maxPkgLen {
		pkg = pkg[:maxPkgLen-3] + "..."
	}

	fmt.Fprintf(os.Stderr, "\r\033[K  %s[%s]%s %s%3d%%%s  %s%s%s",
		cyan, bar, nc, bold, percent, nc, dim, pkg, nc)
}

// drawIntegrationProgress renders a simple counter progress for integration mode.
func drawIntegrationProgress(completed int, testName string) {
	maxLen := 35
	if len(testName) > maxLen {
		testName = testName[:maxLen-3] + "..."
	}
	fmt.Fprintf(os.Stderr, "\r\033[K  %sRunning...%s  %s[%d completed]%s  %s%s%s",
		cyan, nc, bold, completed, nc, dim, testName, nc)
}

// drawLine draws a horizontal table border.
func drawLine(left, mid, right string, hasCov bool) {
	fmt.Printf("%s%s", dim, left)
	fmt.Print(strings.Repeat("─", colPkg+2))
	fmt.Print(mid)
	fmt.Print(strings.Repeat("─", colTests+2))
	fmt.Print(mid)
	fmt.Print(strings.Repeat("─", colPass+2))
	fmt.Print(mid)
	fmt.Print(strings.Repeat("─", colFail+2))
	fmt.Print(mid)
	fmt.Print(strings.Repeat("─", colSkip+2))
	fmt.Print(mid)
	fmt.Print(strings.Repeat("─", colTime+2))
	if hasCov {
		fmt.Print(mid)
		fmt.Print(strings.Repeat("─", colCov+2))
	}
	fmt.Printf("%s%s\n", right, nc)
}

// getTotalCoverage parses the coverage profile and computes the total percentage.
func getTotalCoverage(coverFile string) string {
	f, err := os.Open(coverFile)
	if err != nil {
		return ""
	}
	defer f.Close()

	var totalStmts, coveredStmts int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		// format: name:startLine.startCol,endLine.endCol numStatements count
		parts := strings.Fields(line)
		if len(parts) != 3 {
			continue
		}
		stmts, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		count, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		totalStmts += stmts
		if count > 0 {
			coveredStmts += stmts
		}
	}

	if totalStmts == 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", float64(coveredStmts)*100.0/float64(totalStmts))
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
