package source

import (
	"path"
	"regexp"
	"strings"
)

var (
	regVersionToken = regexp.MustCompile(`^v?\d+$`)
	// regQuotedVersion matches a dotted number in a regexp.QuoteMeta result.
	regQuotedVersion = regexp.MustCompile(`[0-9]+(\\\.[0-9]+)*`)

	tvosTokens = map[string]bool{"tvos": true, "appletv": true, "atv": true}
	iosTokens  = map[string]bool{"ios": true, "iphone": true, "iphoneos": true, "ipad": true, "ipados": true}
)

// IsIPAName reports whether name is an IPA file name (.ipa, or TrollStore's .tipa).
func IsIPAName(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return ext == ".ipa" || ext == ".tipa"
}

// Tokens returns the lowercase words of name without its IPA extension,
// split on every rune that is not a-z or 0-9.
func Tokens(name string) []string {
	if IsIPAName(name) {
		name = strings.TrimSuffix(name, path.Ext(name))
	}
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
}

// PlatformHint guesses the platform from words in texts: it is tvOS or iOS
// only when the words name that platform alone.
func PlatformHint(texts ...string) Platform {
	tvos, ios := false, false
	for _, text := range texts {
		for _, tok := range Tokens(text) {
			tvos = tvos || tvosTokens[tok]
			ios = ios || iosTokens[tok]
		}
	}
	switch {
	case tvos && !ios:
		return PlatformTVOS
	case ios && !tvos:
		return PlatformIOS
	default:
		return PlatformUnknown
	}
}

// DevicePlatform returns the platform of a device class.
func DevicePlatform(deviceClass string) Platform {
	switch deviceClass {
	case "AppleTV":
		return PlatformTVOS
	case "iPhone", "iPad":
		return PlatformIOS
	default:
		return PlatformUnknown
	}
}

// Opposite returns the other known platform, or PlatformUnknown.
func Opposite(p Platform) Platform {
	switch p {
	case PlatformTVOS:
		return PlatformIOS
	case PlatformIOS:
		return PlatformTVOS
	default:
		return PlatformUnknown
	}
}

// compatible reports whether a build hinted for p may run on want.
func compatible(p, want Platform) bool {
	other := Opposite(want)
	return other == PlatformUnknown || p != other
}

// DeriveFilter returns an asset-name filter that keeps selecting the variant
// picked among the IPA assets of a release (Obtainium's asset filter): the
// first word of picked that is neither a version number nor shared with the
// other assets, or else the whole name with its version numbers made generic
// so that it keeps matching in later releases. When the variants differ only
// in numbers (App_tvOS_15.ipa, App_tvOS_17.ipa), the generic name would match
// them all, so the filter is the exact name of picked.
func DeriveFilter(picked string, others []string) string {
	otherTokens := map[string]bool{}
	for _, n := range others {
		if n == picked {
			continue
		}
		for _, tok := range Tokens(n) {
			otherTokens[tok] = true
		}
	}
	for _, tok := range Tokens(picked) {
		if regVersionToken.MatchString(tok) || otherTokens[tok] {
			continue
		}
		return `(?i)(^|[^a-z0-9])` + regexp.QuoteMeta(tok) + `([^a-z0-9]|$)`
	}
	generic := `(?i)^` + regQuotedVersion.ReplaceAllLiteralString(regexp.QuoteMeta(picked), `[0-9]+(\.[0-9]+)*`) + `$`
	re := regexp.MustCompile(generic)
	for _, n := range others {
		if n != picked && re.MatchString(n) {
			return `(?i)^` + regexp.QuoteMeta(picked) + `$`
		}
	}
	return generic
}

// Suggest returns the ID of the build to preselect for deviceClass, or ""
// when the user must choose.
func Suggest(builds []Build, deviceClass string) string {
	want := DevicePlatform(deviceClass)
	var exact, compatibles []Build
	for _, b := range builds {
		if want != PlatformUnknown && b.Platform == want {
			exact = append(exact, b)
		}
		if compatible(b.Platform, want) {
			compatibles = append(compatibles, b)
		}
	}
	if len(exact) == 1 {
		return exact[0].ID
	}
	if len(exact) == 0 && len(compatibles) == 1 {
		return compatibles[0].ID
	}
	return ""
}
