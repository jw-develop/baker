package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
	Concurrency int
	Durations   []time.Duration
	AvgDuration time.Duration
	MinDuration time.Duration
	MaxDuration time.Duration
	FailedJobs  int
}

func main() {
	target := flag.String("target", "", "Make target to run")
	title := flag.String("title", "", "Title to display in header")
	color := flag.String("color", "", "Color name (green, yellow, magenta, blue, cyan, white)")
	concurrency := flag.Int("concurrency", 0, "Max concurrent jobs (0 = unlimited)")
	benchmark := flag.Int("benchmark", 0, "Max concurrency to benchmark (0 = NumCPU when flag present)")
	iterations := flag.Int("iterations", 1, "Iterations per concurrency level in benchmark mode")
	flag.Parse()

	dirs := flag.Args()

	if *target == "" || *title == "" || len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: baker -target <target> -title <title> [-color <color>] [-concurrency <n>] <subdirs...>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	resolvedColor := resolveColor(*color)

	if *benchmark > 0 || hasBenchmarkFlag() {
		maxConcurrency := *benchmark
		if maxConcurrency == 0 {
			maxConcurrency = runtime.NumCPU()
		}
		if *iterations < 1 {
			fmt.Fprintf(os.Stderr, "Error: -iterations must be >= 1\n")
			os.Exit(1)
		}

		printBenchmarkWarning(*target, len(dirs), maxConcurrency, *iterations)
		results := runBenchmark(*target, dirs, maxConcurrency, *iterations)
		printBenchmarkResults(resolvedColor, *title, results)

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

func hasBenchmarkFlag() bool {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-benchmark") {
			return true
		}
	}
	return false
}

func runBenchmark(target string, dirs []string, maxConcurrency int, iterations int) []BenchmarkResult {
	results := make([]BenchmarkResult, maxConcurrency)

	for c := 1; c <= maxConcurrency; c++ {
		br := BenchmarkResult{
			Concurrency: c,
			Durations:   make([]time.Duration, iterations),
		}

		for i := 0; i < iterations; i++ {
			start := time.Now()
			jobResults := runJobs(target, dirs, c)

			for result := range jobResults {
				if result.ExitCode != 0 {
					br.FailedJobs++
				}
			}

			br.Durations[i] = time.Since(start)
		}

		var total time.Duration
		br.MinDuration = br.Durations[0]
		br.MaxDuration = br.Durations[0]
		for _, d := range br.Durations {
			total += d
			if d < br.MinDuration {
				br.MinDuration = d
			}
			if d > br.MaxDuration {
				br.MaxDuration = d
			}
		}
		br.AvgDuration = total / time.Duration(iterations)

		results[c-1] = br
	}

	return results
}

func printBenchmarkWarning(target string, numDirs int, maxConcurrency int, iterations int) {
	totalRuns := maxConcurrency * iterations * numDirs
	fmt.Printf("%sWARNING:%s This benchmark will run '%s' across %d directories\n",
		yellow, reset, target, numDirs)
	fmt.Printf("         %d times total (%d concurrency levels x %d iterations x %d dirs)\n",
		totalRuns, maxConcurrency, iterations, numDirs)
	fmt.Printf("         Ensure your target is safe to run repeatedly.\n\n")
}

func printBenchmarkResults(color string, title string, results []BenchmarkResult) {
	printHeader(color, "BENCHMARK: "+title)
	fmt.Println()

	fmt.Printf("Concurrency   Avg Time    Min Time    Max Time    Speedup    Failures\n")
	fmt.Printf("-----------   --------    --------    --------    -------    --------\n")

	baseline := results[0].AvgDuration
	fastestIdx := 0
	for i, r := range results {
		if r.AvgDuration < results[fastestIdx].AvgDuration {
			fastestIdx = i
		}
	}

	for i, r := range results {
		speedup := float64(baseline) / float64(r.AvgDuration)
		marker := ""
		if i == fastestIdx {
			marker = "    ◀"
		}
		fmt.Printf("%5d         %7.1fs    %7.1fs    %7.1fs    %5.2fx    %5d%s\n",
			r.Concurrency,
			r.AvgDuration.Seconds(),
			r.MinDuration.Seconds(),
			r.MaxDuration.Seconds(),
			speedup,
			r.FailedJobs,
			marker)
	}

	fmt.Println()
	best := results[fastestIdx]
	speedup := float64(baseline) / float64(best.AvgDuration)

	bar := "══════════════════════════════════════════════════════════════════════════════"
	fmt.Printf("%s%s%s\n", color, bar, reset)
	fmt.Printf("%s   Recommendation: -concurrency %d (%.1fs, %.2fx speedup)%s\n",
		color, best.Concurrency, best.AvgDuration.Seconds(), speedup, reset)
	fmt.Printf("%s%s%s\n", color, bar, reset)
}
