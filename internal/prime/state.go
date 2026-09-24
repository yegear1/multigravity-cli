package prime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type BucketState struct {
	LastPrimedAt    string `json:"last_primed_at,omitempty"`
	LastResetTime   string `json:"last_reset_time,omitempty"`
	LastCascadeID   string `json:"last_cascade_id,omitempty"`
	LastModel       string `json:"last_model,omitempty"`
	LastPrompt      string `json:"last_prompt,omitempty"`
	TargetPrimeTime string `json:"target_prime_time,omitempty"`
	Status          string `json:"status,omitempty"`
}

type ProfileState map[string]BucketState

type PrimeState struct {
	Profiles map[string]ProfileState `json:"profiles"`
}

func GetStateFilePath() string {
	userHome, _ := os.UserHomeDir()
	if realHome := os.Getenv("REAL_HOME"); realHome != "" {
		userHome = realHome
	}

	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "multigravity", "prime_state.json")
		}
	}
	return filepath.Join(userHome, ".local", "share", "multigravity", "prime_state.json")
}

func LoadState() (*PrimeState, error) {
	path := GetStateFilePath()
	state := &PrimeState{
		Profiles: make(map[string]ProfileState),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return state, nil
	}

	rawProfiles, ok := rawMap["profiles"].(map[string]interface{})
	if !ok {
		return state, nil
	}

	for profKey, pVal := range rawProfiles {
		pMap, ok := pVal.(map[string]interface{})
		if !ok {
			continue
		}

		profState := make(ProfileState)

		// Check legacy flat state
		if _, hasLegacy := pMap["last_reset_time"]; hasLegacy {
			if _, hasGemini := pMap["gemini"]; !hasGemini {
				var legacyB BucketState
				legacyBytes, _ := json.Marshal(pMap)
				_ = json.Unmarshal(legacyBytes, &legacyB)
				if legacyB.LastModel == "" {
					legacyB.LastModel = "gemini-3.6-flash-low"
				}
				if legacyB.Status == "" {
					legacyB.Status = "success"
				}
				profState["gemini"] = legacyB
				state.Profiles[profKey] = profState
				continue
			}
		}

		// Normal dual-bucket / 4-bucket state
		for bKey, bVal := range pMap {
			bMap, ok := bVal.(map[string]interface{})
			if !ok {
				continue
			}
			var bs BucketState
			bBytes, _ := json.Marshal(bMap)
			if err := json.Unmarshal(bBytes, &bs); err == nil {
				profState[bKey] = bs
			}
		}

		state.Profiles[profKey] = profState
	}

	return state, nil
}

func SaveState(state *PrimeState) error {
	path := GetStateFilePath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
