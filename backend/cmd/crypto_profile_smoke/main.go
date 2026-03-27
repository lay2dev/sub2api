package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type dataset struct {
	MustHit    []datasetCase `json:"must_hit"`
	MustMiss   []datasetCase `json:"must_miss"`
	Borderline []datasetCase `json:"borderline"`
}

type datasetCase struct {
	ID              string `json:"id"`
	Text            string `json:"text"`
	ExpectedMatch   *bool  `json:"expected_match"`
	ExpectedProfile string `json:"expected_profile"`
	Note            string `json:"note"`
}

func main() {
	datasetPath := flag.String("dataset", "testdata/crypto_profile_dataset.json", "dataset path relative to backend module root")
	includeBorderline := flag.Bool("include-borderline", false, "also print borderline cases")
	flag.Parse()

	provider := firstNonEmptyEnv("CRYPTO_PROFILE_DETECTOR_PROVIDER", "openrouter")
	apiKey := firstNonEmptyEnv("CRYPTO_PROFILE_DETECTOR_API_KEY", "OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "CRYPTO_PROFILE_DETECTOR_API_KEY or OPENROUTER_API_KEY is required")
		os.Exit(1)
	}

	endpoint := firstNonEmptyEnv("CRYPTO_PROFILE_DETECTOR_ENDPOINT")
	model := firstNonEmptyEnv("CRYPTO_PROFILE_DETECTOR_MODEL", "OPENROUTER_MODEL")
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			CryptoProfileDetection: config.CryptoProfileDetectionConfig{
				Provider:         provider,
				Enabled:          true,
				Endpoint:         endpoint,
				APIKey:           apiKey,
				OpenRouterAPIKey: apiKey,
				Model:            model,
				TimeoutSeconds:   15,
			},
		},
	}

	detector := service.NewOpenRouterCryptoProfileDetector(cfg)
	if !detector.Enabled() {
		fmt.Fprintln(os.Stderr, "crypto profile detector is disabled")
		os.Exit(1)
	}

	ds, err := loadDataset(*datasetPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	failures := 0
	ctx := context.Background()

	failures += runCases(ctx, detector, "must_hit", ds.MustHit)
	failures += runCases(ctx, detector, "must_miss", ds.MustMiss)

	if *includeBorderline {
		_ = runCases(ctx, detector, "borderline", ds.Borderline)
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\ncrypto profile smoke failed: %d case(s)\n", failures)
		os.Exit(1)
	}

	fmt.Println("\nPASS: crypto profile smoke")
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func loadDataset(path string) (*dataset, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve dataset path: %w", err)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read dataset %s: %w", abs, err)
	}

	var ds dataset
	if err := json.Unmarshal(raw, &ds); err != nil {
		return nil, fmt.Errorf("parse dataset %s: %w", abs, err)
	}
	return &ds, nil
}

func runCases(ctx context.Context, detector service.CryptoProfileDetector, group string, cases []datasetCase) int {
	failures := 0
	fmt.Printf("\n[%s]\n", group)
	for _, tc := range cases {
		result, err := detector.Detect(ctx, tc.Text)
		if err != nil {
			failures++
			fmt.Printf("FAIL %s: detector error: %v\n", tc.ID, err)
			continue
		}

		actualMatch := result != nil && result.Matched
		actualProfile := ""
		if result != nil {
			actualProfile = result.Profile
		}

		if tc.ExpectedMatch != nil {
			if actualMatch != *tc.ExpectedMatch {
				failures++
				fmt.Printf("FAIL %s: expected match=%v got match=%v profile=%q\n", tc.ID, *tc.ExpectedMatch, actualMatch, actualProfile)
				continue
			}
		}
		if tc.ExpectedProfile != "" && actualProfile != tc.ExpectedProfile {
			failures++
			fmt.Printf("FAIL %s: expected profile=%q got profile=%q\n", tc.ID, tc.ExpectedProfile, actualProfile)
			continue
		}

		fmt.Printf("PASS %s: match=%v profile=%q\n", tc.ID, actualMatch, actualProfile)
	}
	return failures
}
