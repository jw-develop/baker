package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ANSI color codes
const (
	reset   = "\033[0m"
	cyan    = "\033[36m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	magenta = "\033[35m"
	blue    = "\033[34m"
	white   = "\033[97m"
)

// Unicode symbols
const (
	checkmark = "✓"
	xmark     = "✗"
)

var colorMap = map[string]string{
	"cyan":    cyan,
	"green":   green,
	"yellow":  yellow,
	"magenta": magenta,
	"blue":    blue,
	"white":   white,
}

// JobResult holds the result of running make in a single directory
type JobResult struct {
	Dir      string
	ExitCode int
	Duration time.Duration
	Output   []byte
}

// BenchmarkResult holds timing results for a single concurrency level
type BenchmarkResult struct {
	Concurrency int  // 0 means unlimited
	NumDirs     int  // for display purposes
	Duration    time.Duration
	FailedJobs  int
}

func main() {
	target := flag.String("target", "", "Make target to run")
	title := flag.String("title", "", "Title to display in header")
	color := flag.String("color", "", "Color name (green, yellow, magenta, blue, cyan, white)")
	concurrency := flag.Int("concurrency", 0, "Max concurrent jobs (0 = unlimited)")
	benchmark := flag.Bool("benchmark", false, "Benchmark all concurrency levels (1 to N, plus unlimited)")
	flag.Parse()

	dirs := flag.Args()

	if *target == "" || *title == "" || len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: baker -target <target> -title <title> [-color <color>] [-concurrency <n>] <subdirs...>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	resolvedColor := resolveColor(*color)

	if *benchmark {
		printBenchmarkWarning(*target, len(dirs))
		results := runBenchmark(*target, dirs)
		printBenchmarkResults(*target, results)
		_ = resolvedColor

		for _, r := range results {
			if r.FailedJobs > 0 {
				os.Exit(1)
			}
		}
		return
	}

	startTime := time.Now()

	printHeader(resolvedColor, *title)

	results := runJobs(*target, dirs, *concurrency)

	failed := false
	for result := range results {
		printResult(resolvedColor, result)
		if result.ExitCode != 0 {
			failed = true
		}
	}

	printFooter(resolvedColor, time.Since(startTime))

	if failed {
		os.Exit(1)
	}
}

// resolveColor converts a color name to ANSI code, or passes through raw ANSI codes
func resolveColor(input string) string {
	if strings.Contains(input, "\033") || strings.Contains(input, "[") {
		return input
	}
	if c, ok := colorMap[strings.ToLower(input)]; ok {
		return c
	}
	return ""
}

var runMake = func(target, dir string) JobResult {
	start := time.Now()

	cmd := exec.Command("make", "-C", dir, target)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return JobResult{
		Dir:      dir,
		ExitCode: exitCode,
		Duration: time.Since(start),
		Output:   output,
	}
}

// runJobs launches make jobs with optional concurrency limit and streams results.
// If concurrency <= 0, all jobs run in parallel (unlimited).
func runJobs(target string, dirs []string, concurrency int) <-chan JobResult {
	results := make(chan JobResult)
	var wg sync.WaitGroup

	var sem chan struct{}
	if concurrency > 0 {
		sem = make(chan struct{}, concurrency)
	}

	for _, dir := range dirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			result := runMake(target, d)
			results <- result
		}(dir)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// printHeader prints the decorated title bar
func printHeader(color, title string) {
	bar := "══════════════════════════════════════════════════════════════════════════════"
	fmt.Printf("%s%s%s\n", color, bar, reset)
	fmt.Printf("%s   ▶  %s%s\n", color, title, reset)
	fmt.Printf("%s%s%s\n", color, bar, reset)
}

// printResult prints success/failure line for one directory
func printResult(color string, result JobResult) {
	elapsed := fmt.Sprintf("%.1fs", result.Duration.Seconds())
	if result.ExitCode == 0 {
		fmt.Printf("%s%s %s%s%s (%s)\n", green, checkmark, color, result.Dir, reset, elapsed)
	} else {
		fmt.Printf("%s%s %s%s%s (%s)\n", cyan, xmark, color, result.Dir, reset, elapsed)
		_, _ = os.Stdout.Write(result.Output)
	}
}

// printFooter prints the closing bar with total time
func printFooter(color string, duration time.Duration) {
	bar := "══════════════════════════════════════════════════════════════════════════════"
	elapsed := fmt.Sprintf("%.1fs", duration.Seconds())
	fmt.Printf("%s%s%s\n", color, bar, reset)
	fmt.Printf("%s   Total time: %s%s\n", color, elapsed, reset)
	fmt.Printf("%s%s%s\n", color, bar, reset)
}

func runBenchmark(target string, dirs []string) []BenchmarkResult {
	numDirs := len(dirs)
	var results []BenchmarkResult

	// Test concurrency numDirs down to 1
	// First entry (numDirs) is labeled as "Unlimited" since all run in parallel
	for c := numDirs; c >= 1; c-- {
		start := time.Now()
		jobResults := runJobs(target, dirs, c)
		failedJobs := 0
		for result := range jobResults {
			if result.ExitCode != 0 {
				failedJobs++
			}
		}
		results = append(results, BenchmarkResult{
			Concurrency: c,
			NumDirs:     numDirs,
			Duration:    time.Since(start),
			FailedJobs:  failedJobs,
		})
	}

	return results
}

func printBenchmarkWarning(target string, numDirs int) {
	numLevels := numDirs
	totalRuns := numLevels * numDirs
	fmt.Printf("%sWARNING:%s This benchmark will run '%s' across %d directories\n",
		yellow, reset, target, numDirs)
	fmt.Printf("         %d times total (%d concurrency levels x %d dirs)\n",
		totalRuns, numLevels, numDirs)
	fmt.Printf("         Ensure your target is safe to run repeatedly.\n\n")
}

func printBenchmarkResults(target string, results []BenchmarkResult) {
	fastestIdx := 0
	for i, r := range results {
		if r.Duration < results[fastestIdx].Duration {
			fastestIdx = i
		}
	}

	// Build column header with target name
	targetTitle := strings.Title(target)
	timeHeader := fmt.Sprintf("%s Time", targetTitle)

	// Calculate column widths
	concurrencyWidth := len("Concurrency")
	for _, r := range results {
		label := formatConcurrencyLabel(r)
		if len(label) > concurrencyWidth {
			concurrencyWidth = len(label)
		}
	}

	timeWidth := len(timeHeader)
	for i, r := range results {
		label := formatTimeLabel(r.Duration, i == fastestIdx)
		if len(label) > timeWidth {
			timeWidth = len(label)
		}
	}

	fmt.Println("Benchmark results:")

	// Header row
	fmt.Printf("  | %-*s | %-*s |\n", concurrencyWidth, "Concurrency", timeWidth, timeHeader)
	// Separator row
	fmt.Printf("  |-%s-|-%s-|\n", strings.Repeat("-", concurrencyWidth), strings.Repeat("-", timeWidth))

	// Data rows
	for i, r := range results {
		concLabel := formatConcurrencyLabel(r)
		timeLabel := formatTimeLabel(r.Duration, i == fastestIdx)
		fmt.Printf("  | %-*s | %-*s |\n", concurrencyWidth, concLabel, timeWidth, timeLabel)
	}
}

func formatConcurrencyLabel(r BenchmarkResult) string {
	if r.Concurrency == r.NumDirs {
		return fmt.Sprintf("%d (all parallel)", r.NumDirs)
	}
	return fmt.Sprintf("%d", r.Concurrency)
}

func formatTimeLabel(d time.Duration, isFastest bool) string {
	timeStr := fmt.Sprintf("%.1fs", d.Seconds())
	if isFastest {
		return timeStr + " ← fastest"
	}
	return timeStr
}
