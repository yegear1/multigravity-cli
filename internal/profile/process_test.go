package profile

import (
	"reflect"
	"testing"
)

func TestLineMatchesPath(t *testing.T) {
	target := "/home/user/AntigravityProfiles/yegear"

	tests := []struct {
		name     string
		line     string
		target   string
		expected bool
	}{
		{
			name:     "exact match surrounded by spaces",
			line:     "1234 1 /app --user-data-dir /home/user/AntigravityProfiles/yegear/.config/Antigravity",
			target:   target,
			expected: true,
		},
		{
			name:     "exact match with trailing slash",
			line:     "1234 1 /app /home/user/AntigravityProfiles/yegear/ something",
			target:   target,
			expected: true,
		},
		{
			name:     "prefix collision yegear vs yegear2",
			line:     "1234 1 /app --user-data-dir /home/user/AntigravityProfiles/yegear2/.config/Antigravity",
			target:   target,
			expected: false,
		},
		{
			name:     "prefix collision with equals",
			line:     "1234 1 --user-data-dir=/home/user/AntigravityProfiles/yegear2",
			target:   target,
			expected: false,
		},
		{
			name:     "match with equals",
			line:     "1234 1 --user-data-dir=/home/user/AntigravityProfiles/yegear/.config",
			target:   target,
			expected: true,
		},
		{
			name:     "match with quotes",
			line:     `1234 1 --dir="/home/user/AntigravityProfiles/yegear"`,
			target:   target,
			expected: true,
		},
		{
			name:     "empty line",
			line:     "",
			target:   target,
			expected: false,
		},
		{
			name:     "empty target",
			line:     "1234 1 /app",
			target:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lineMatchesPath(tt.line, tt.target)
			if got != tt.expected {
				t.Errorf("lineMatchesPath(%q, %q) = %v, want %v", tt.line, tt.target, got, tt.expected)
			}
		})
	}
}

func TestParsePIDsFromPSOutput_PrefixCollision(t *testing.T) {
	mockPS := `  961     942 /home/yegear/apps/antigravity/antigravity --user-data-dir /home/yegear/AntigravityProfiles/yegear2/.config/Antigravity --extensions-dir /home/yegear/AntigravityProfiles/yegear2/.antigravity/extensions
 1013     961 /proc/self/exe --user-data-dir=/home/yegear/AntigravityProfiles/yegear2/.config/Antigravity
 2001       1 /home/yegear/apps/antigravity/antigravity --user-data-dir /home/yegear/AntigravityProfiles/yegear/.config/Antigravity
`
	yegearProfile := "/home/yegear/AntigravityProfiles/yegear"
	yegearData := "/home/yegear/AntigravityProfiles/yegear/.config/Antigravity"

	pidsYegear := parsePIDsFromPSOutput(mockPS, yegearProfile, yegearData, 99999, 99998)
	expectedYegear := []int{2001}
	if !reflect.DeepEqual(pidsYegear, expectedYegear) {
		t.Errorf("expected pids for yegear to be %v, got %v", expectedYegear, pidsYegear)
	}

	yegear2Profile := "/home/yegear/AntigravityProfiles/yegear2"
	yegear2Data := "/home/yegear/AntigravityProfiles/yegear2/.config/Antigravity"

	pidsYegear2 := parsePIDsFromPSOutput(mockPS, yegear2Profile, yegear2Data, 99999, 99998)
	expectedYegear2 := []int{961, 1013}
	if !reflect.DeepEqual(pidsYegear2, expectedYegear2) {
		t.Errorf("expected pids for yegear2 to be %v, got %v", expectedYegear2, pidsYegear2)
	}
}
