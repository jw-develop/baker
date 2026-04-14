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

	results := runJobs("test", dirs, 0)
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

	results := runJobs("test", dirs, 1)
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

	results := runJobs("test", dirs, 2)
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
			results := runJobs("test", dirs, tc.concurrency)
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

	results := runJobs("custom-target", []string{"a", "b"}, 1)
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
	results := runJobs("test", expectedDirs, 0)
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

	results := runJobs("test", dirs, 1)
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

	results := runJobs("test", []string{}, 0)
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

	results := runJobs("test", []string{"only-one"}, 1)
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

	results := runJobs("test", dirs, 10)
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

	results := runJobs("test", dirs, -1)
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
		printResult(blue, result)
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
		printResult(yellow, result)
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
		printResult(green, result)
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

	results := runJobs("build", []string{"proj"}, 1)
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

	results := runJobs("build", []string{"proj"}, 1)
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
	results := runJobs("test", dirs, 1)
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
	results := runJobs("test", dirs, 0)
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

	results := runJobs("build", dirs, 2)

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
