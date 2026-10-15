package quota

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestTokenCalibrator_BaselineMatchesLegacy(t *testing.T) {
	cal := NewTokenCalibrator("")

	if cal.EstimateTokens("") != 0 {
		t.Fatalf("expected 0 for empty string, got %d", cal.EstimateTokens(""))
	}
	if cal.EstimateTokens("abcd") != 1 {
		t.Fatalf("expected 1 for 'abcd', got %d", cal.EstimateTokens("abcd"))
	}
	if cal.EstimateTokens("abcde") != 2 {
		t.Fatalf("expected 2 for 'abcde', got %d", cal.EstimateTokens("abcde"))
	}

	stats := cal.GetStats()
	if stats.Samples != 0 || stats.RunesPerToken != DefaultRunesPerToken {
		t.Fatalf("unexpected uncalibrated stats: %+v", stats)
	}
}

func TestTokenCalibrator_CalibrationAdaptsRatio(t *testing.T) {
	cal := NewTokenCalibrator("")

	// Calibration with denser token ratio: e.g. 2 runes per token
	// 200 runes, 100 tokens
	cal.CalibrateTokens("gemini-2.5-pro", "12345678901234567890", 10, "12345678901234567890", 10)

	stats := cal.GetStats()
	if stats.Samples != 1 {
		t.Fatalf("expected 1 sample, got %d", stats.Samples)
	}
	if stats.RunesPerToken >= DefaultRunesPerToken {
		t.Fatalf("expected runes per token to decrease below 4.0, got %f", stats.RunesPerToken)
	}

	// For a 20-rune string, uncalibrated (ratio 4.0) gives 5 tokens.
	// With denser ratio, it should estimate higher token count.
	est := cal.EstimateTokensForModel("gemini-2.5-pro", "12345678901234567890")
	if est <= 5 {
		t.Fatalf("expected calibrated estimate > 5, got %d", est)
	}
}

func TestTokenCalibrator_ModelIsolation(t *testing.T) {
	cal := NewTokenCalibrator("")

	// Model A: dense (2 runes/token)
	// Prompt 100 runes, 50 tokens
	cal.CalibrateTokens("model-dense", "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789", 50, "", 0)

	// Model B: sparse (6 runes/token)
	// Prompt 120 runes, 20 tokens
	cal.CalibrateTokens("model-sparse", "012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789", 20, "", 0)

	text := "0123456789012345678901234567890123456789" // 40 runes

	estDense := cal.EstimateTokensForModel("model-dense", text)
	estSparse := cal.EstimateTokensForModel("model-sparse", text)

	if estDense <= estSparse {
		t.Fatalf("expected dense model to estimate more tokens than sparse model (dense=%d, sparse=%d)", estDense, estSparse)
	}
}

func TestTokenCalibrator_SegmentAwareness(t *testing.T) {
	cal := NewTokenCalibrator("")

	// 100 prompt runes -> 50 prompt tokens (ratio ~2.0)
	// 100 completion runes -> 20 completion tokens (ratio ~5.0)
	promptText := "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"
	compText := promptText

	cal.CalibrateTokens("gemini-2.5-pro", promptText, 50, compText, 20)

	sample := "01234567890123456789" // 20 runes
	pTokens := cal.EstimateSegmentTokens("gemini-2.5-pro", "prompt", sample)
	cTokens := cal.EstimateSegmentTokens("gemini-2.5-pro", "completion", sample)

	if pTokens <= cTokens {
		t.Fatalf("expected prompt tokens (%d) > completion tokens (%d)", pTokens, cTokens)
	}
}

func TestTokenCalibrator_ConcurrencyAndReset(t *testing.T) {
	cal := NewTokenCalibrator("")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cal.CalibrateTokens("gemini", "sample prompt", 3, "sample completion", 4)
			_ = cal.EstimateTokens("some text to estimate tokens for")
			_ = cal.GetStats()
		}(i)
	}
	wg.Wait()

	stats := cal.GetStats()
	if stats.Samples != 20 {
		t.Fatalf("expected 20 samples, got %d", stats.Samples)
	}

	cal.Reset()
	after := cal.GetStats()
	if after.Samples != 0 || after.RunesPerToken != DefaultRunesPerToken {
		t.Fatalf("expected reset state, got %+v", after)
	}
}

func TestTokenCalibrator_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	persistPath := filepath.Join(tmpDir, "calibration.json")

	cal1 := NewTokenCalibrator(persistPath)
	cal1.CalibrateTokens("gemini-2.5-pro", "hello world prompt", 4, "hello response", 3)

	// Create new calibrator instance pointing to the same file
	cal2 := NewTokenCalibrator(persistPath)
	stats := cal2.GetStats()

	if stats.Samples != 1 {
		t.Fatalf("expected 1 restored sample, got %d", stats.Samples)
	}
	if _, ok := stats.Models["gemini-2.5-pro"]; !ok {
		t.Fatalf("expected model 'gemini-2.5-pro' in restored models: %+v", stats.Models)
	}
}
