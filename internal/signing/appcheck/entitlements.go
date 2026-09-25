package appcheck

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	entitlementApplicationID  = "application-identifier"
	entitlementTeamID         = "com.apple.developer.team-identifier"
	entitlementKeychainGroups = "keychain-access-groups"
	entitlementGetTaskAllow   = "get-task-allow"
)

// teamPrefix is the engine's TEAM_ID_REGEX.
var teamPrefix = regexp.MustCompile(`^[A-Z0-9]{10}\.`)

// unrequestableEntitlements are the keys the engine derives from the profile
// or rewrites itself; they are not compared with what the executable requests.
var unrequestableEntitlements = map[string]bool{
	entitlementApplicationID:  true,
	entitlementTeamID:         true,
	entitlementKeychainGroups: true,
	entitlementGetTaskAllow:   true,
}

// simulateEntitlements returns the entitlements the engine signs a bundle
// with. The engine starts from the profile entitlements; when the executable
// embeds entitlements it applies merge_entitlements with the (rewritten)
// bundle identifier, otherwise merge_entitlements fails early and the profile
// entitlements are used verbatim.
func simulateEntitlements(profile, binary map[string]any, bundleID string) map[string]any {
	entitlements := cloneDict(profile)
	if binary == nil {
		return entitlements
	}
	teamID, hasTeamID := entitlements[entitlementTeamID].(string)

	for key, value := range entitlements {
		entitlements[key] = replaceWildcard(value, bundleID)
	}
	if groups, ok := binary[entitlementKeychainGroups].([]any); ok {
		entitlements[entitlementKeychainGroups] = cloneValue(groups)
	}
	if groups, ok := entitlements[entitlementKeychainGroups].([]any); ok {
		kept := make([]any, 0, len(groups))
		for _, group := range groups {
			value, ok := group.(string)
			if !ok || !teamPrefix.MatchString(value) {
				continue
			}
			if hasTeamID {
				value = teamID + "." + value[11:]
			}
			kept = append(kept, value)
		}
		entitlements[entitlementKeychainGroups] = kept
	}
	return entitlements
}

// replaceWildcard replaces every '*' of every string with bundleID, like the
// engine does on the values of the profile entitlements.
func replaceWildcard(value any, bundleID string) any {
	switch v := value.(type) {
	case string:
		return strings.ReplaceAll(v, "*", bundleID)
	case []any:
		for i := range v {
			v[i] = replaceWildcard(v[i], bundleID)
		}
		return v
	case map[string]any:
		for key := range v {
			v[key] = replaceWildcard(v[key], bundleID)
		}
		return v
	}
	return value
}

func cloneDict(dict map[string]any) map[string]any {
	clone := make(map[string]any, len(dict))
	for key, value := range dict {
		clone[key] = cloneValue(value)
	}
	return clone
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneDict(v)
	case []any:
		clone := make([]any, len(v))
		for i, item := range v {
			clone[i] = cloneValue(item)
		}
		return clone
	case []byte:
		return bytes.Clone(v)
	}
	return value
}

// unsatisfiedEntitlements compares what the executable requests with what the
// bundle will be signed with. missing lists requested keys that are absent;
// changed lists keys whose requested values are not all granted.
func unsatisfiedEntitlements(requested, granted map[string]any) (missing, changed []string) {
	keys := make([]string, 0, len(requested))
	for key := range requested {
		if !unrequestableEntitlements[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, ok := granted[key]
		switch {
		case !ok:
			missing = append(missing, key)
		case !satisfies(value, requested[key]):
			changed = append(changed, key)
		}
	}
	return missing, changed
}

// satisfies reports whether granted provides requested: every requested array
// item is granted, every requested dictionary key is satisfied and scalars are
// equal.
func satisfies(granted, requested any) bool {
	switch r := requested.(type) {
	case []any:
		g, ok := granted.([]any)
		if !ok {
			return false
		}
		for _, item := range r {
			if !containsValue(g, item) {
				return false
			}
		}
		return true
	case map[string]any:
		g, ok := granted.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range r {
			grantedValue, ok := g[key]
			if !ok || !satisfies(grantedValue, value) {
				return false
			}
		}
		return true
	}
	return plistEqual(granted, requested)
}

func containsValue(values []any, value any) bool {
	for _, candidate := range values {
		if plistEqual(candidate, value) {
			return true
		}
	}
	return false
}

// plistEqual compares two decoded property list values; integers compare by
// value whatever their signedness.
func plistEqual(a, b any) bool {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for key, value := range x {
			other, ok := y[key]
			if !ok || !plistEqual(value, other) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !plistEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	case []byte:
		y, ok := b.([]byte)
		return ok && bytes.Equal(x, y)
	case time.Time:
		y, ok := b.(time.Time)
		return ok && x.Equal(y)
	case string, bool:
		return a == b
	}
	return numberEqual(a, b)
}

func numberEqual(a, b any) bool {
	// Binary property lists may decode 32-bit reals as float32.
	if f, ok := a.(float32); ok {
		a = float64(f)
	}
	if f, ok := b.(float32); ok {
		b = float64(f)
	}
	switch x := a.(type) {
	case uint64:
		switch y := b.(type) {
		case uint64:
			return x == y
		case int64:
			return y >= 0 && uint64(y) == x
		case float64:
			return float64(x) == y
		}
	case int64:
		switch y := b.(type) {
		case int64:
			return x == y
		case uint64:
			return x >= 0 && uint64(x) == y
		case float64:
			return float64(x) == y
		}
	case float64:
		switch y := b.(type) {
		case float64:
			return x == y
		case uint64:
			return x == float64(y)
		case int64:
			return x == float64(y)
		}
	}
	return false
}
