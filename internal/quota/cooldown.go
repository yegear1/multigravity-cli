package quota

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

const (
	cooldownPoolFileName = "cooldown-pool.json"

	CooldownScopeProfile = "profile"
	CooldownScopeModel   = "model"
	CooldownScopeBucket  = "bucket"
	CooldownScopeGlobal  = "global"
)

// CooldownEntry represents an active rate-limit or throttling state for a specific target.
type CooldownEntry struct {
	Target    string    `json:"target"` // "profile:<name>", "model:<model>", "bucket:<bucket>", "global"
	Until     time.Time `json:"until"`
	Reason    string    `json:"reason,omitempty"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CooldownPool represents the serialized cross-profile shared cooldown state.
type CooldownPool struct {
	Entries map[string]CooldownEntry `json:"entries"`
}

var (
	cooldownPoolMu         sync.RWMutex
	customCooldownPoolPath string

	sharedCooldownMu sync.RWMutex
	sharedCooldowns  = make(map[string]CooldownEntry)
)

// SetTestCooldownPoolPath overrides the pool file path for isolated testing.
func SetTestCooldownPoolPath(path string) func() {
	cooldownPoolMu.Lock()
	prev := customCooldownPoolPath
	customCooldownPoolPath = path
	cooldownPoolMu.Unlock()
	return func() {
		cooldownPoolMu.Lock()
		customCooldownPoolPath = prev
		cooldownPoolMu.Unlock()
	}
}

func getCooldownPoolPath() string {
	cooldownPoolMu.RLock()
	defer cooldownPoolMu.RUnlock()
	if customCooldownPoolPath != "" {
		return customCooldownPoolPath
	}
	home := config.GetMultigravityHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".multigravity", cooldownPoolFileName)
}

// FormatCooldownTarget normalizes scope and identifier into a canonical target string.
func FormatCooldownTarget(scope, identifier string) string {
	scope = strings.TrimSpace(strings.ToLower(scope))
	identifier = strings.TrimSpace(identifier)
	switch scope {
	case CooldownScopeGlobal, "":
		if scope == "" && identifier == "" {
			return CooldownScopeGlobal
		}
		if scope == "" {
			return "profile:" + identifier
		}
		return CooldownScopeGlobal
	case CooldownScopeModel:
		return "model:" + strings.ToLower(identifier)
	case CooldownScopeBucket:
		return "bucket:" + strings.ToLower(identifier)
	case CooldownScopeProfile:
		return "profile:" + identifier
	default:
		return scope + ":" + identifier
	}
}

// RecordCooldown records a target cooldown in memory and persists to the shared cooldown pool file.
// If duration <= 0, the target is cleared from active cooldown.
func RecordCooldown(target string, duration time.Duration, reason, source string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}

	sharedCooldownMu.Lock()
	if duration <= 0 {
		delete(sharedCooldowns, target)
		sharedCooldownMu.Unlock()
		return removeCooldownFromPoolFile(target)
	}

	now := time.Now().UTC()
	until := now.Add(duration)
	entry := CooldownEntry{
		Target:    target,
		Until:     until,
		Reason:    strings.TrimSpace(reason),
		Source:    strings.TrimSpace(source),
		CreatedAt: now,
	}
	sharedCooldowns[target] = entry
	sharedCooldownMu.Unlock()

	return updateCooldownInPoolFile(entry)
}

// RecordModelCooldown places a specific model (and its normalized name) under shared cooldown.
func RecordModelCooldown(model string, duration time.Duration, reason string) {
	norm := strings.ToLower(strings.TrimSpace(model))
	if norm == "" {
		return
	}
	_ = RecordCooldown("model:"+norm, duration, reason, SourceGateway)
}

// RecordBucketCooldown places an entire model bucket group (argon, 3p, gemini) under shared cooldown.
func RecordBucketCooldown(bucketGroup string, duration time.Duration, reason string) {
	norm := strings.ToLower(strings.TrimSpace(bucketGroup))
	if norm == "" {
		return
	}
	_ = RecordCooldown("bucket:"+norm, duration, reason, SourceGateway)
}

// RecordGlobalCooldown places all models and profiles under global shared cooldown.
func RecordGlobalCooldown(duration time.Duration, reason string) {
	_ = RecordCooldown(CooldownScopeGlobal, duration, reason, SourceGateway)
}

// IsTargetOnCooldown checks whether a canonical target is currently cooling down.
func IsTargetOnCooldown(target string) (bool, time.Time, string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return false, time.Time{}, ""
	}

	now := time.Now().UTC()

	// Check in-memory fast path
	sharedCooldownMu.RLock()
	entry, ok := sharedCooldowns[target]
	sharedCooldownMu.RUnlock()

	if ok {
		if now.Before(entry.Until) {
			return true, entry.Until, entry.Reason
		}
		// Expired in memory: cleanup
		sharedCooldownMu.Lock()
		delete(sharedCooldowns, target)
		sharedCooldownMu.Unlock()
	}

	// Fallback: check shared pool file
	poolPath := getCooldownPoolPath()
	if poolPath == "" {
		return false, time.Time{}, ""
	}

	pool, err := readCooldownPoolFile(poolPath)
	if err == nil && pool != nil {
		if fileEntry, found := pool.Entries[target]; found {
			if now.Before(fileEntry.Until) {
				sharedCooldownMu.Lock()
				sharedCooldowns[target] = fileEntry
				sharedCooldownMu.Unlock()
				return true, fileEntry.Until, fileEntry.Reason
			}
		}
	}

	return false, time.Time{}, ""
}

// IsModelOnCooldown checks whether a model, its base family, its bucket group, or global scope is on cooldown.
func IsModelOnCooldown(model string) (bool, time.Time, string) {
	normModel := strings.ToLower(strings.TrimSpace(model))
	if normModel == "" {
		return false, time.Time{}, ""
	}

	// 1. Check global cooldown
	if onCd, until, reason := IsTargetOnCooldown(CooldownScopeGlobal); onCd {
		return true, until, reason
	}

	// 2. Check exact model name
	if onCd, until, reason := IsTargetOnCooldown("model:" + normModel); onCd {
		return true, until, reason
	}

	// 3. Check base model name without effort suffix (-low, -medium, -high)
	baseModel := normModel
	for _, suffix := range []string{"-low", "-medium", "-high"} {
		if strings.HasSuffix(baseModel, suffix) {
			baseModel = strings.TrimSuffix(baseModel, suffix)
			break
		}
	}
	if baseModel != normModel {
		if onCd, until, reason := IsTargetOnCooldown("model:" + baseModel); onCd {
			return true, until, reason
		}
	}

	// 4. Check bucket group
	bucketGroup := BucketGroupForModel(normModel)
	if bucketGroup != "" {
		if onCd, until, reason := IsTargetOnCooldown("bucket:" + bucketGroup); onCd {
			return true, until, reason
		}
	}

	return false, time.Time{}, ""
}

// ResetSharedCooldowns clears all in-memory and disk-persisted shared cooldown entries.
func ResetSharedCooldowns() {
	profileCooldownMu.Lock()
	profileCooldowns = make(map[string]time.Time)
	profileCooldownMu.Unlock()

	sharedCooldownMu.Lock()
	sharedCooldowns = make(map[string]CooldownEntry)
	sharedCooldownMu.Unlock()

	path := getCooldownPoolPath()
	if path != "" {
		_ = os.Remove(path)
	}
}

// ParseRetryAfterHeader parses standard HTTP Retry-After headers supporting integer seconds, duration strings, or HTTP dates.
func ParseRetryAfterHeader(headerVal string) time.Duration {
	val := strings.TrimSpace(headerVal)
	if val == "" {
		return 0
	}
	// 1. Integer seconds (e.g. "120")
	if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	// 2. Duration string (e.g. "120s", "5m")
	if d, err := time.ParseDuration(val); err == nil && d > 0 {
		return d
	}
	// 3. HTTP date (RFC1123, RFC850, ANSI C)
	if t, err := http.ParseTime(val); err == nil {
		diff := time.Until(t)
		if diff > 0 {
			return diff
		}
	}
	return 0
}

func readCooldownPoolFile(poolPath string) (*CooldownPool, error) {
	if poolPath == "" {
		return &CooldownPool{Entries: make(map[string]CooldownEntry)}, nil
	}
	f, err := os.Open(poolPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CooldownPool{Entries: make(map[string]CooldownEntry)}, nil
		}
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return &CooldownPool{Entries: make(map[string]CooldownEntry)}, nil
	}

	var pool CooldownPool
	if err := json.Unmarshal(data, &pool); err != nil {
		return &CooldownPool{Entries: make(map[string]CooldownEntry)}, nil
	}
	if pool.Entries == nil {
		pool.Entries = make(map[string]CooldownEntry)
	}
	return &pool, nil
}

func updateCooldownInPoolFile(entry CooldownEntry) error {
	poolPath := getCooldownPoolPath()
	if poolPath == "" {
		return nil
	}
	dir := filepath.Dir(poolPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(poolPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := lockFile(f); err != nil {
		return err
	}
	defer unlockFile(f)

	data, err := io.ReadAll(f)
	pool := &CooldownPool{Entries: make(map[string]CooldownEntry)}
	if err == nil && len(bytes.TrimSpace(data)) > 0 {
		_ = json.Unmarshal(data, pool)
	}
	if pool.Entries == nil {
		pool.Entries = make(map[string]CooldownEntry)
	}

	// Prune expired entries while modifying
	now := time.Now().UTC()
	for k, v := range pool.Entries {
		if now.After(v.Until) {
			delete(pool.Entries, k)
		}
	}

	pool.Entries[entry.Target] = entry

	serialized, err := json.MarshalIndent(pool, "", "  ")
	if err != nil {
		return err
	}

	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err = f.Write(append(serialized, '\n'))
	return err
}

func removeCooldownFromPoolFile(target string) error {
	poolPath := getCooldownPoolPath()
	if poolPath == "" {
		return nil
	}

	f, err := os.OpenFile(poolPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	if err := lockFile(f); err != nil {
		return err
	}
	defer unlockFile(f)

	data, err := io.ReadAll(f)
	pool := &CooldownPool{Entries: make(map[string]CooldownEntry)}
	if err == nil && len(bytes.TrimSpace(data)) > 0 {
		_ = json.Unmarshal(data, pool)
	}
	if pool.Entries == nil {
		return nil
	}

	delete(pool.Entries, target)

	// Prune expired entries
	now := time.Now().UTC()
	for k, v := range pool.Entries {
		if now.After(v.Until) {
			delete(pool.Entries, k)
		}
	}

	serialized, err := json.MarshalIndent(pool, "", "  ")
	if err != nil {
		return err
	}

	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err = f.Write(append(serialized, '\n'))
	return err
}
