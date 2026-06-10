// Package sensitive is the deterministic counterpart of the sensitive-area
// list in CLAUDE.md: instead of asking the model whether a diff touches a
// sensitive area, the binary detects it from paths and diff content.
package sensitive

import (
	"regexp"
	"sort"
	"strings"
)

type Area string

const (
	AreaAuth       Area = "auth/sessions"
	AreaCrypto     Area = "crypto"
	AreaValidation Area = "input-validation"
	AreaDatabase   Area = "database"
	AreaEndpoints  Area = "public-endpoints"
	AreaAuthz      Area = "authorization"
	AreaSecrets    Area = "secrets"
	AreaMigrations Area = "schema-migrations"
	AreaDeps       Area = "dependencies"
	AreaPathSSRF   Area = "path/ssrf"
	AreaTemplates  Area = "templates"
)

type rule struct {
	area Area
	re   *regexp.Regexp
}

var pathRules = []rule{
	{AreaAuth, regexp.MustCompile(`(?i)(^|/)(auth|authn|login|logout|session|sessions|oauth|oidc|sso|jwt|token|password|credential)s?[^/]*\.\w+$`)},
	{AreaAuth, regexp.MustCompile(`(?i)(^|/)(auth|sessions?|identity)(/|$)`)},
	{AreaCrypto, regexp.MustCompile(`(?i)(^|/)(crypto|cipher|encrypt|decrypt|hash|hmac|signing|signature)[^/]*\.\w+$`)},
	{AreaSecrets, regexp.MustCompile(`(?i)(^|/)\.env(\.|$)|(^|/)(secrets?|credentials?)[^/]*\.(json|ya?ml|toml|env|pem|key)$|\.(pem|key|p12|pfx)$`)},
	{AreaMigrations, regexp.MustCompile(`(?i)(^|/)migrations?(/|$)|(^|/)\d{4,}[-_][^/]*\.sql$`)},
	{AreaDeps, regexp.MustCompile(`(^|/)(go\.mod|go\.sum|package\.json|package-lock\.json|yarn\.lock|pnpm-lock\.yaml|requirements[^/]*\.txt|pyproject\.toml|poetry\.lock|Cargo\.toml|Cargo\.lock|Gemfile(\.lock)?|composer\.json|composer\.lock)$`)},
	{AreaDatabase, regexp.MustCompile(`(?i)(^|/)(repositor(y|ies)|queries|dao|models?)(/|[^/]*\.\w+$)|\.sql$`)},
	{AreaEndpoints, regexp.MustCompile(`(?i)(^|/)(handlers?|routes?|controllers?|endpoints?|api)(/|[^/]*\.\w+$)`)},
	{AreaAuthz, regexp.MustCompile(`(?i)(^|/)(authz|permission|rbac|abac|acl|polic(y|ies)|tenant)[^/]*(\.\w+)?$`)},
}

var diffRules = []rule{
	{AreaAuth, regexp.MustCompile(`(?i)\b(jwt|bearer|set-?cookie|session[_-]?id|csrf|xsrf|refresh[_-]?token)\b`)},
	{AreaCrypto, regexp.MustCompile(`(?i)\b(aes|rsa|ecdsa|ed25519|hmac|sha(1|256|512)|md5|bcrypt|argon2|scrypt|pbkdf2|rand\.(Read|Int)|getRandomValues)\b`)},
	{AreaSecrets, regexp.MustCompile(`(?i)\b(api[_-]?key|secret[_-]?key|private[_-]?key|client[_-]?secret|access[_-]?token)\b`)},
	{AreaDatabase, regexp.MustCompile(`(?i)\b(select|insert|update|delete)\b.*\b(from|into|set|where)\b|\$\d\b|\bexec(ute)?(Query|Context)?\s*\(`)},
	{AreaValidation, regexp.MustCompile(`(?i)\b(sanitiz|validat|unmarshal|json\.parse|parseint|strconv\.|deserializ)`)},
	{AreaPathSSRF, regexp.MustCompile(`(?i)\b(filepath\.Join|path\.Join|os\.Open|readfile|http\.Get|fetch\(|urllib|requests\.(get|post))\b`)},
	{AreaTemplates, regexp.MustCompile(`(?i)(dangerouslySetInnerHTML|innerHTML\s*=|template\.HTML|v-html|\{\{\{|render_template|mark_safe)`)},
	{AreaEndpoints, regexp.MustCompile(`(?i)\b(http\.HandleFunc|router\.(Get|Post|Put|Delete|Handle)|app\.(get|post|put|delete|use)\s*\(|@(Get|Post|Put|Delete)Mapping|websocket)\b`)},
}

// MatchPath returns the sensitive areas a single file path belongs to.
func MatchPath(path string) []Area {
	p := strings.ReplaceAll(path, "\\", "/")
	seen := map[Area]bool{}
	for _, r := range pathRules {
		if r.re.MatchString(p) {
			seen[r.area] = true
		}
	}
	return sorted(seen)
}

// MatchDiff classifies a unified diff: file paths via path rules and
// added lines via content rules. Only "+" lines count — removing
// sensitive code should not trigger enforcement by itself.
func MatchDiff(diff string) []Area {
	seen := map[Area]bool{}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			for _, a := range MatchPath(strings.TrimPrefix(line, "+++ b/")) {
				seen[a] = true
			}
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			for _, r := range diffRules {
				if !seen[r.area] && r.re.MatchString(line) {
					seen[r.area] = true
				}
			}
		}
	}
	return sorted(seen)
}

func sorted(set map[Area]bool) []Area {
	var out []Area
	for a := range set {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Join renders areas for human-facing hook messages.
func Join(areas []Area) string {
	parts := make([]string, len(areas))
	for i, a := range areas {
		parts[i] = string(a)
	}
	return strings.Join(parts, ", ")
}
