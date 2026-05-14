//go:build ignore

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

// Table column widths — must match test-runner.sh exactly.
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
	// ── CLI flags ────────────────────────────────────────────────────────
	race := flag.Bool("race", false, "Enable race detector")
	verbose := flag.Bool("verbose", false, "Verbose mode")
	vShort := flag.Bool("v", false, "Verbose mode (short)")
	coverprofile := flag.String("coverprofile", "", "Write coverage profile to file")
	tags := flag.String("tags", "", "Build tags")
	title := flag.String("title", "", "Title header")
	flag.Parse()

	if *vShort {
		*verbose = true
	}

	testPath := "./..."
	if flag.NArg() > 0 {
		testPath = flag.Arg(0)
	}

	// ── Module name ─────────────────────────────────────────────────────
	moduleName := detectModuleName()

	// ── Title header ────────────────────────────────────────────────────
	if *title != "" {
		header := fmt.Sprintf("━━━ %s ", *title)
		pad := 78 - len([]rune(header))
		if pad > 0 {
			header += strings.Repeat("━", pad)
		}
		fmt.Printf("%s%s%s%s\n\n", bold, cyan, header, nc)
	}

	// ── Mode description ────────────────────────────────────────────────
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

	// ── Count packages for progress bar ─────────────────────────────────
	totalPackages := countPackages(testPath)

	// ── Build shared flags ──────────────────────────────────────────────
	baseFlags := buildFlags(*race, *coverprofile, *tags)

	testExit := 0

	// ── Verbose: first pass with live output ────────────────────────────
	if *verbose {
		args := append([]string{"test", "-v"}, baseFlags...)
		args = append(args, testPath)
		testExit = runVerbosePass(args)
		fmt.Println()
	}

	// ── JSON pass ───────────────────────────────────────────────────────
	args := append([]string{"test", "-json"}, baseFlags...)
	args = append(args, testPath)
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating pipe: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting go test: %v\n", err)
		os.Exit(1)
	}

	// ── Process JSON events ─────────────────────────────────────────────
	pkgResults, pkgOrder, failedTests, eventsExit := processEvents(stdout, *verbose, totalPackages, moduleName)
	// Wait releases process resources; exit status is tracked via JSON events.
	_ = cmd.Wait() //nolint:errcheck // exit status handled via JSON event stream

	if eventsExit != 0 {
		testExit = eventsExit
	}

	// Clear progress bar line.
	if !*verbose {
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}
	fmt.Println()

	// ── Table ───────────────────────────────────────────────────────────
	hasCov := *coverprofile != ""
	totalTests, totalPassed, totalFailed, totalSkipped := printTable(pkgResults, pkgOrder, hasCov, *coverprofile)
	fmt.Println()

	// ── Failed tests section (non-verbose only) ─────────────────────────
	if !*verbose && len(failedTests) > 0 {
		printFailedTests(failedTests)
	}

	// ── Final summary ───────────────────────────────────────────────────
	printSummary(totalTests, totalPassed, totalFailed, totalSkipped, hasCov, *coverprofile)

	// Exit with correct code.
	if totalFailed > 0 || testExit != 0 {
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

// printTable renders the results table and returns totals.
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

// printFailedTests renders the failed tests section.
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

// printSummary renders the final summary line.
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

// drawProgress renders the progress bar on stderr.
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
