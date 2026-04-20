package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveColor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"green lowercase", "green", green},
		{"green uppercase", "GREEN", green},
		{"green mixed case", "Green", green},
		{"yellow", "yellow", yellow},
		{"magenta", "magenta", magenta},
		{"blue", "blue", blue},
		{"cyan", "cyan", cyan},
		{"white", "white", white},
		{"unknown color returns empty", "purple", ""},
		{"empty input returns empty", "", ""},
		{"raw ansi code with escape", "\033[31m", "\033[31m"},
		{"raw ansi code with bracket", "[31m", "[31m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveColor(tc.input)
			if got != tc.expected {
				t.Errorf("resolveColor(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestJobResult(t *testing.T) {
	t.Run("success result", func(t *testing.T) {
		result := JobResult{
			Dir:      "test-dir",
			ExitCode: 0,
			Duration: 2 * time.Second,
			Output:   []byte("test output"),
		}

		if result.Dir != "test-dir" {
			t.Errorf("Dir = %q, want %q", result.Dir, "test-dir")
		}
		if result.ExitCode != 0 {
			t.Errorf("ExitCode = %d, want 0", result.ExitCode)
		}
		if result.Duration != 2*time.Second {
			t.Errorf("Duration = %v, want 2s", result.Duration)
		}
	})

	t.Run("failure result", func(t *testing.T) {
		result := JobResult{
			Dir:      "failing-dir",
			ExitCode: 1,
			Duration: 500 * time.Millisecond,
			Output:   []byte("error: something failed"),
		}

		if result.ExitCode != 1 {
			t.Errorf("ExitCode = %d, want 1", result.ExitCode)
		}
		if !bytes.Contains(result.Output, []byte("error")) {
			t.Errorf("Output should contain error message")
		}
	})
}

func TestRunJobs_UnlimitedConcurrency(t *testing.T) {
	dirs := []string{"a", "b", "c", "d"}
	var maxConcurrent int32
	var currentConcurrent int32

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		current := atomic.AddInt32(&currentConcurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&currentConcurrent, -1)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", dirs, 0, false)
	for range results {
	}

	if maxConcurrent < 2 {
		t.Errorf("Expected multiple concurrent jobs with unlimited concurrency, got max=%d", maxConcurrent)
	}
}

func TestRunJobs_ConcurrencyOne(t *testing.T) {
	dirs := []string{"a", "b", "c", "d"}
	var maxConcurrent int32
	var currentConcurrent int32

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		current := atomic.AddInt32(&currentConcurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&currentConcurrent, -1)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", dirs, 1, false)
	for range results {
	}

	if maxConcurrent != 1 {
		t.Errorf("Expected max concurrent=1 with concurrency=1, got max=%d", maxConcurrent)
	}
}

func TestRunJobs_ConcurrencyTwo(t *testing.T) {
	dirs := []string{"a", "b", "c", "d"}
	var maxConcurrent int32
	var currentConcurrent int32

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		current := atomic.AddInt32(&currentConcurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&currentConcurrent, -1)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", dirs, 2, false)
	for range results {
	}

	if maxConcurrent > 2 {
		t.Errorf("Expected max concurrent<=2 with concurrency=2, got max=%d", maxConcurrent)
	}
	if maxConcurrent < 2 {
		t.Errorf("Expected to reach concurrency=2 with 4 jobs, got max=%d", maxConcurrent)
	}
}

func TestRunJobs_AllJobsComplete(t *testing.T) {
	dirs := []string{"a", "b", "c", "d", "e"}

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	testCases := []struct {
		name        string
		concurrency int
	}{
		{"unlimited", 0},
		{"one", 1},
		{"two", 2},
		{"three", 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := runJobs("test", dirs, tc.concurrency, false)
			count := 0
			for range results {
				count++
			}
			if count != len(dirs) {
				t.Errorf("Expected %d results, got %d", len(dirs), count)
			}
		})
	}
}

func TestRunJobs_PassesTargetCorrectly(t *testing.T) {
	var capturedTargets []string

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		capturedTargets = append(capturedTargets, target)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("custom-target", []string{"a", "b"}, 1, false)
	for range results {
	}

	for _, target := range capturedTargets {
		if target != "custom-target" {
			t.Errorf("Expected target 'custom-target', got %q", target)
		}
	}
}

func TestRunJobs_PassesDirCorrectly(t *testing.T) {
	capturedDirs := make(map[string]bool)

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		capturedDirs[dir] = true
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	expectedDirs := []string{"dir-a", "dir-b", "dir-c"}
	results := runJobs("test", expectedDirs, 0, false)
	for range results {
	}

	for _, dir := range expectedDirs {
		if !capturedDirs[dir] {
			t.Errorf("Expected dir %q to be processed", dir)
		}
	}
}

func TestRunJobs_ReturnsAllResults(t *testing.T) {
	dirs := []string{"a", "b", "c"}
	exitCodes := map[string]int{"a": 0, "b": 1, "c": 0}

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		return JobResult{Dir: dir, ExitCode: exitCodes[dir]}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", dirs, 1, false)
	resultMap := make(map[string]int)
	for result := range results {
		resultMap[result.Dir] = result.ExitCode
	}

	for dir, expectedCode := range exitCodes {
		if resultMap[dir] != expectedCode {
			t.Errorf("Dir %q: expected exit code %d, got %d", dir, expectedCode, resultMap[dir])
		}
	}
}

func TestRunJobs_EmptyDirs(t *testing.T) {
	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		t.Error("runMake should not be called with empty dirs")
		return JobResult{}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", []string{}, 0, false)
	count := 0
	for range results {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 results for empty dirs, got %d", count)
	}
}

func TestRunJobs_SingleDir(t *testing.T) {
	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		return JobResult{Dir: dir, ExitCode: 0, Duration: 100 * time.Millisecond}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", []string{"only-one"}, 1, false)
	count := 0
	for result := range results {
		count++
		if result.Dir != "only-one" {
			t.Errorf("Expected dir 'only-one', got %q", result.Dir)
		}
	}

	if count != 1 {
		t.Errorf("Expected 1 result, got %d", count)
	}
}

func TestRunJobs_ConcurrencyHigherThanDirs(t *testing.T) {
	dirs := []string{"a", "b"}
	var maxConcurrent int32
	var currentConcurrent int32

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		current := atomic.AddInt32(&currentConcurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&currentConcurrent, -1)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", dirs, 10, false)
	count := 0
	for range results {
		count++
	}

	if count != 2 {
		t.Errorf("Expected 2 results, got %d", count)
	}
	if maxConcurrent > 2 {
		t.Errorf("Max concurrent should not exceed number of dirs (2), got %d", maxConcurrent)
	}
}

func TestRunJobs_NegativeConcurrency(t *testing.T) {
	dirs := []string{"a", "b", "c", "d"}
	var maxConcurrent int32
	var currentConcurrent int32

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		current := atomic.AddInt32(&currentConcurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&currentConcurrent, -1)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", dirs, -1, false)
	for range results {
	}

	if maxConcurrent < 2 {
		t.Errorf("Negative concurrency should behave as unlimited, got max=%d", maxConcurrent)
	}
}

func captureOutput(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintHeader(t *testing.T) {
	output := captureOutput(func() {
		printHeader(green, "TEST TITLE")
	})

	if !strings.Contains(output, "TEST TITLE") {
		t.Errorf("Header should contain title, got: %s", output)
	}
	if !strings.Contains(output, "▶") {
		t.Errorf("Header should contain arrow symbol, got: %s", output)
	}
	if !strings.Contains(output, "═") {
		t.Errorf("Header should contain bar character, got: %s", output)
	}
	if !strings.Contains(output, green) {
		t.Errorf("Header should contain color code, got: %s", output)
	}
	if !strings.Contains(output, reset) {
		t.Errorf("Header should contain reset code, got: %s", output)
	}
}

func TestPrintHeader_NoColor(t *testing.T) {
	output := captureOutput(func() {
		printHeader("", "NO COLOR TITLE")
	})

	if !strings.Contains(output, "NO COLOR TITLE") {
		t.Errorf("Header should contain title even without color, got: %s", output)
	}
}

func TestPrintResult_Success(t *testing.T) {
	result := JobResult{
		Dir:      "my-service",
		ExitCode: 0,
		Duration: 1500 * time.Millisecond,
		Output:   []byte("some output"),
	}

	output := captureOutput(func() {
		printResult(blue, result, false)
	})

	if !strings.Contains(output, checkmark) {
		t.Errorf("Success result should contain checkmark, got: %s", output)
	}
	if !strings.Contains(output, "my-service") {
		t.Errorf("Result should contain dir name, got: %s", output)
	}
	if !strings.Contains(output, "1.5s") {
		t.Errorf("Result should contain duration, got: %s", output)
	}
	if strings.Contains(output, "some output") {
		t.Errorf("Success result should NOT print output, got: %s", output)
	}
}

func TestPrintResult_Failure(t *testing.T) {
	result := JobResult{
		Dir:      "failing-service",
		ExitCode: 1,
		Duration: 2 * time.Second,
		Output:   []byte("error: compilation failed\n"),
	}

	output := captureOutput(func() {
		printResult(yellow, result, false)
	})

	if !strings.Contains(output, xmark) {
		t.Errorf("Failure result should contain xmark, got: %s", output)
	}
	if !strings.Contains(output, "failing-service") {
		t.Errorf("Result should contain dir name, got: %s", output)
	}
	if !strings.Contains(output, "2.0s") {
		t.Errorf("Result should contain duration, got: %s", output)
	}
	if !strings.Contains(output, "error: compilation failed") {
		t.Errorf("Failure result should print output, got: %s", output)
	}
}

func TestPrintResult_FailureWithExitCode2(t *testing.T) {
	result := JobResult{
		Dir:      "service",
		ExitCode: 2,
		Duration: 100 * time.Millisecond,
		Output:   []byte("some error"),
	}

	output := captureOutput(func() {
		printResult(green, result, false)
	})

	if !strings.Contains(output, xmark) {
		t.Errorf("Non-zero exit code should show xmark, got: %s", output)
	}
}

func TestPrintFooter(t *testing.T) {
	output := captureOutput(func() {
		printFooter(magenta, 5*time.Second+500*time.Millisecond)
	})

	if !strings.Contains(output, "Total time:") {
		t.Errorf("Footer should contain 'Total time:', got: %s", output)
	}
	if !strings.Contains(output, "5.5s") {
		t.Errorf("Footer should contain duration, got: %s", output)
	}
	if !strings.Contains(output, "═") {
		t.Errorf("Footer should contain bar character, got: %s", output)
	}
	if !strings.Contains(output, magenta) {
		t.Errorf("Footer should contain color code, got: %s", output)
	}
}

func TestPrintFooter_SubSecond(t *testing.T) {
	output := captureOutput(func() {
		printFooter(green, 100*time.Millisecond)
	})

	if !strings.Contains(output, "0.1s") {
		t.Errorf("Footer should format sub-second duration, got: %s", output)
	}
}

func TestRunJobs_PreservesOutput(t *testing.T) {
	expectedOutput := []byte("detailed build output\nline 2\nline 3")

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		return JobResult{Dir: dir, ExitCode: 0, Output: expectedOutput}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("build", []string{"proj"}, 1, false)
	result := <-results

	if !bytes.Equal(result.Output, expectedOutput) {
		t.Errorf("Output not preserved. Got %q, want %q", result.Output, expectedOutput)
	}
}

func TestRunJobs_PreservesDuration(t *testing.T) {
	expectedDuration := 123 * time.Millisecond

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		return JobResult{Dir: dir, ExitCode: 0, Duration: expectedDuration}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("build", []string{"proj"}, 1, false)
	result := <-results

	if result.Duration != expectedDuration {
		t.Errorf("Duration not preserved. Got %v, want %v", result.Duration, expectedDuration)
	}
}

func TestColorConstants(t *testing.T) {
	colors := map[string]string{
		"reset":   reset,
		"cyan":    cyan,
		"green":   green,
		"yellow":  yellow,
		"magenta": magenta,
		"blue":    blue,
		"white":   white,
	}

	for name, code := range colors {
		if !strings.HasPrefix(code, "\033[") {
			t.Errorf("Color %s should start with ANSI escape, got %q", name, code)
		}
		if !strings.HasSuffix(code, "m") {
			t.Errorf("Color %s should end with 'm', got %q", name, code)
		}
	}
}

func TestColorMap(t *testing.T) {
	expectedColors := []string{"cyan", "green", "yellow", "magenta", "blue", "white"}

	for _, name := range expectedColors {
		if _, ok := colorMap[name]; !ok {
			t.Errorf("colorMap should contain %q", name)
		}
	}
}

func TestRunJobs_SequentialTiming(t *testing.T) {
	dirs := []string{"a", "b", "c"}
	jobDuration := 30 * time.Millisecond

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		time.Sleep(jobDuration)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	start := time.Now()
	results := runJobs("test", dirs, 1, false)
	for range results {
	}
	elapsed := time.Since(start)

	minExpected := jobDuration * time.Duration(len(dirs))
	if elapsed < minExpected {
		t.Errorf("Sequential execution too fast: %v < %v", elapsed, minExpected)
	}
}

func TestRunJobs_ParallelTiming(t *testing.T) {
	dirs := []string{"a", "b", "c", "d"}
	jobDuration := 50 * time.Millisecond

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		time.Sleep(jobDuration)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	start := time.Now()
	results := runJobs("test", dirs, 0, false)
	for range results {
	}
	elapsed := time.Since(start)

	sequentialTime := jobDuration * time.Duration(len(dirs))
	if elapsed >= sequentialTime {
		t.Errorf("Parallel execution too slow: %v >= %v (sequential)", elapsed, sequentialTime)
	}
}

func TestIntegration_FullWorkflow(t *testing.T) {
	dirs := []string{"service-a", "service-b", "service-c"}
	processedDirs := make(map[string]bool)

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		processedDirs[dir] = true
		exitCode := 0
		if dir == "service-b" {
			exitCode = 1
		}
		return JobResult{
			Dir:      dir,
			ExitCode: exitCode,
			Duration: 10 * time.Millisecond,
			Output:   []byte(fmt.Sprintf("output from %s", dir)),
		}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("build", dirs, 2, false)

	var successCount, failCount int
	for result := range results {
		if result.ExitCode == 0 {
			successCount++
		} else {
			failCount++
		}
	}

	if successCount != 2 {
		t.Errorf("Expected 2 successes, got %d", successCount)
	}
	if failCount != 1 {
		t.Errorf("Expected 1 failure, got %d", failCount)
	}

	for _, dir := range dirs {
		if !processedDirs[dir] {
			t.Errorf("Dir %q was not processed", dir)
		}
	}
}

func TestRunJobs_DurationReflectsExecutionTimeNotWaitTime(t *testing.T) {
	dirs := []string{"a", "b", "c", "d"}
	jobDuration := 50 * time.Millisecond

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		start := time.Now()
		time.Sleep(jobDuration)
		return JobResult{
			Dir:      dir,
			ExitCode: 0,
			Duration: time.Since(start),
		}
	}
	defer func() { runMake = originalRunMake }()

	results := runJobs("test", dirs, 1, false)

	for result := range results {
		if result.Duration < jobDuration {
			t.Errorf("Job %s duration %v is less than job execution time %v", result.Dir, result.Duration, jobDuration)
		}
		maxExpected := jobDuration + 20*time.Millisecond
		if result.Duration > maxExpected {
			t.Errorf("Job %s duration %v exceeds expected max %v (should not include queue wait time)", result.Dir, result.Duration, maxExpected)
		}
	}
}

func TestRunBenchmark_ResultCount(t *testing.T) {
	dirs := []string{"a", "b", "c"}
	callCount := 0

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		callCount++
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runBenchmark("test", dirs, false)

	// Should have len(dirs) results: concurrency len(dirs) down to 1
	expectedResults := len(dirs)
	if len(results) != expectedResults {
		t.Errorf("Expected %d results, got %d", expectedResults, len(results))
	}

	// Total calls: numDirs levels * numDirs jobs = 3 * 3 = 9
	expectedCalls := expectedResults * len(dirs)
	if callCount != expectedCalls {
		t.Errorf("Expected %d runMake calls, got %d", expectedCalls, callCount)
	}

	// First result should be max concurrency (all parallel)
	if results[0].Concurrency != len(dirs) {
		t.Errorf("First result should be %d (all parallel), got %d", len(dirs), results[0].Concurrency)
	}
}

func TestRunBenchmark_ConcurrencyOrder(t *testing.T) {
	dirs := []string{"a", "b", "c", "d"}

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runBenchmark("test", dirs, false)

	// Order should be: 4, 3, 2, 1
	expectedOrder := []int{4, 3, 2, 1}
	for i, expected := range expectedOrder {
		if results[i].Concurrency != expected {
			t.Errorf("Result %d: expected concurrency %d, got %d", i, expected, results[i].Concurrency)
		}
	}
}

func TestRunBenchmark_FailureTracking(t *testing.T) {
	dirs := []string{"pass", "fail"}

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		exitCode := 0
		if dir == "fail" {
			exitCode = 1
		}
		return JobResult{Dir: dir, ExitCode: exitCode}
	}
	defer func() { runMake = originalRunMake }()

	results := runBenchmark("test", dirs, false)

	for _, r := range results {
		if r.FailedJobs != 1 {
			t.Errorf("Concurrency %d: expected 1 failure, got %d",
				r.Concurrency, r.FailedJobs)
		}
	}
}

func TestRunBenchmark_SpeedupWithConcurrency(t *testing.T) {
	dirs := []string{"a", "b", "c", "d"}
	jobDuration := 30 * time.Millisecond

	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		time.Sleep(jobDuration)
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	results := runBenchmark("test", dirs, false)

	// Last result is concurrency=1 (sequential), should be slowest
	// First result is all parallel, should be fastest
	sequential := results[len(results)-1]
	allParallel := results[0]

	if allParallel.Duration >= sequential.Duration {
		t.Errorf("Expected all parallel (%v) to be faster than sequential (%v)",
			allParallel.Duration, sequential.Duration)
	}

	speedup := float64(sequential.Duration) / float64(allParallel.Duration)
	if speedup < 2.0 {
		t.Errorf("Expected >2x speedup, got %.2fx", speedup)
	}
}

func TestPrintBenchmarkResults(t *testing.T) {
	results := []BenchmarkResult{
		{Concurrency: 4, NumDirs: 4, Duration: 1 * time.Second, FailedJobs: 0},
		{Concurrency: 3, NumDirs: 4, Duration: 1200 * time.Millisecond, FailedJobs: 0},
		{Concurrency: 2, NumDirs: 4, Duration: 2 * time.Second, FailedJobs: 0},
		{Concurrency: 1, NumDirs: 4, Duration: 4 * time.Second, FailedJobs: 0},
	}

	output := captureOutput(func() {
		printBenchmarkResults("lint", results)
	})

	if !strings.Contains(output, "Benchmark results") {
		t.Error("Output should contain 'Benchmark results'")
	}
	if !strings.Contains(output, "Concurrency") {
		t.Error("Output should contain table header 'Concurrency'")
	}
	if !strings.Contains(output, "Lint Time") {
		t.Error("Output should contain 'Lint Time' column header")
	}
	if !strings.Contains(output, "← fastest") {
		t.Error("Output should mark fastest result")
	}
	if !strings.Contains(output, "4 (all parallel)") {
		t.Error("Output should show all parallel label")
	}
}

func TestPrintBenchmarkWarning(t *testing.T) {
	output := captureOutput(func() {
		printBenchmarkWarning("build", 5)
	})

	if !strings.Contains(output, "WARNING") {
		t.Error("Output should contain WARNING")
	}
	if !strings.Contains(output, "build") {
		t.Error("Output should contain target name")
	}
	if !strings.Contains(output, "5 directories") {
		t.Error("Output should mention number of directories")
	}
	// 5 levels (1-5) * 5 dirs = 25 total
	if !strings.Contains(output, "25 times total") {
		t.Error("Output should show total runs (5*5=25)")
	}
}

func TestFormatConcurrencyLabel(t *testing.T) {
	tests := []struct {
		result   BenchmarkResult
		expected string
	}{
		{BenchmarkResult{Concurrency: 5, NumDirs: 5}, "5 (all parallel)"},
		{BenchmarkResult{Concurrency: 1, NumDirs: 5}, "1"},
		{BenchmarkResult{Concurrency: 3, NumDirs: 5}, "3"},
	}

	for _, tc := range tests {
		got := formatConcurrencyLabel(tc.result)
		if got != tc.expected {
			t.Errorf("formatConcurrencyLabel(%+v) = %q, want %q", tc.result, got, tc.expected)
		}
	}
}

func TestFormatTimeLabel(t *testing.T) {
	tests := []struct {
		duration  time.Duration
		isFastest bool
		expected  string
	}{
		{1500 * time.Millisecond, false, "1.5s"},
		{1500 * time.Millisecond, true, "1.5s ← fastest"},
		{10 * time.Second, false, "10.0s"},
	}

	for _, tc := range tests {
		got := formatTimeLabel(tc.duration, tc.isFastest)
		if got != tc.expected {
			t.Errorf("formatTimeLabel(%v, %v) = %q, want %q", tc.duration, tc.isFastest, got, tc.expected)
		}
	}
}

func TestRunJobs_VerbosePrintsRunningMessage(t *testing.T) {
	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	output := captureOutput(func() {
		results := runJobs("build", []string{"mydir"}, 1, true)
		for range results {
		}
	})

	if !strings.Contains(output, "→ Running:") {
		t.Errorf("Verbose mode should print '→ Running:', got: %s", output)
	}
	if !strings.Contains(output, "make -C mydir build") {
		t.Errorf("Verbose mode should print the make command, got: %s", output)
	}
}

func TestRunJobs_NonVerboseNoRunningMessage(t *testing.T) {
	originalRunMake := runMake
	runMake = func(target, dir string) JobResult {
		return JobResult{Dir: dir, ExitCode: 0}
	}
	defer func() { runMake = originalRunMake }()

	output := captureOutput(func() {
		results := runJobs("build", []string{"mydir"}, 1, false)
		for range results {
		}
	})

	if strings.Contains(output, "→ Running:") {
		t.Errorf("Non-verbose mode should NOT print '→ Running:', got: %s", output)
	}
}

func TestPrintResult_VerboseShowsSuccessOutput(t *testing.T) {
	result := JobResult{
		Dir:      "my-service",
		ExitCode: 0,
		Duration: 1 * time.Second,
		Output:   []byte("build successful\n"),
	}

	output := captureOutput(func() {
		printResult(green, result, true)
	})

	if !strings.Contains(output, "build successful") {
		t.Errorf("Verbose mode should print success output, got: %s", output)
	}
}

func TestPrintResult_NonVerboseHidesSuccessOutput(t *testing.T) {
	result := JobResult{
		Dir:      "my-service",
		ExitCode: 0,
		Duration: 1 * time.Second,
		Output:   []byte("build successful\n"),
	}

	output := captureOutput(func() {
		printResult(green, result, false)
	})

	if strings.Contains(output, "build successful") {
		t.Errorf("Non-verbose mode should NOT print success output, got: %s", output)
	}
}

func TestPrintResult_FailureAlwaysShowsOutput(t *testing.T) {
	result := JobResult{
		Dir:      "my-service",
		ExitCode: 1,
		Duration: 1 * time.Second,
		Output:   []byte("error: build failed\n"),
	}

	// Test with verbose=false
	output := captureOutput(func() {
		printResult(green, result, false)
	})
	if !strings.Contains(output, "error: build failed") {
		t.Errorf("Failure should always print output even without verbose, got: %s", output)
	}

	// Test with verbose=true
	output = captureOutput(func() {
		printResult(green, result, true)
	})
	if !strings.Contains(output, "error: build failed") {
		t.Errorf("Failure should always print output with verbose, got: %s", output)
	}
}
