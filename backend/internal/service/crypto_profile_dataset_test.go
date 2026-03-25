package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type cryptoProfileDatasetFile struct {
	MustHit    []cryptoProfileDatasetCase `json:"must_hit"`
	MustMiss   []cryptoProfileDatasetCase `json:"must_miss"`
	Borderline []cryptoProfileDatasetCase `json:"borderline"`
}

type cryptoProfileDatasetCase struct {
	ID              string  `json:"id"`
	Text            string  `json:"text"`
	ExpectedMatch   *bool   `json:"expected_match"`
	ExpectedProfile string  `json:"expected_profile"`
	Note            string  `json:"note"`
}

func TestCryptoProfileDataset_IsPresentAndLargeEnough(t *testing.T) {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "crypto_profile_dataset.json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var dataset cryptoProfileDatasetFile
	require.NoError(t, json.Unmarshal(raw, &dataset))

	require.GreaterOrEqual(t, len(dataset.MustHit), 15)
	require.GreaterOrEqual(t, len(dataset.MustMiss), 12)
	require.GreaterOrEqual(t, len(dataset.Borderline), 5)

	seen := make(map[string]struct{})
	checkCases := func(cases []cryptoProfileDatasetCase, requireExpectation bool) {
		for _, tc := range cases {
			require.NotEmpty(t, tc.ID)
			require.NotEmpty(t, tc.Text)
			if requireExpectation {
				require.NotNil(t, tc.ExpectedMatch)
			}
			if tc.ExpectedMatch != nil && *tc.ExpectedMatch {
				require.NotEmpty(t, tc.ExpectedProfile)
			}
			if _, exists := seen[tc.ID]; exists {
				t.Fatalf("duplicate dataset id: %s", tc.ID)
			}
			seen[tc.ID] = struct{}{}
		}
	}

	checkCases(dataset.MustHit, true)
	checkCases(dataset.MustMiss, true)
	checkCases(dataset.Borderline, false)
}
