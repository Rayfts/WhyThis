package sensitive

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Finding struct {
	Category    string `json:"category"`
	Source      string `json:"source"`
	Match       string `json:"match"`
	Explanation string `json:"explanation"`
}

type Report struct {
	Sensitive bool      `json:"sensitive"`
	Generated bool      `json:"generated"`
	Vendor    bool      `json:"vendor"`
	Findings  []Finding `json:"findings,omitempty"`
}

var pathRules = []struct {
	category string
	terms    []string
	why      string
}{
	{"access-control", []string{"auth", "authentication", "authorization", "permission", "permissions", "rbac", "acl", "policy"}, "path indicates authentication, authorization, or permission enforcement"},
	{"secrets", []string{"secret", "secrets", "credential", "credentials", "token", "tokens", "apikey", "api_key", "privatekey", "private_key", "kms", "vault"}, "path indicates secret or credential handling"},
	{"cryptography", []string{"crypto", "cryptography", "encrypt", "encryption", "decrypt", "decryption", "cipher", "tls", "ssl", "x509", "signature", "signing"}, "path indicates cryptographic or transport-security code"},
	{"financial", []string{"payment", "payments", "billing", "checkout", "invoice", "invoices", "wallet", "ledger"}, "path indicates financial or billing behavior"},
	{"data-integrity", []string{"migration", "migrations", "schema", "schemas", "database", "datastore", "transaction"}, "path indicates schema, migration, or transaction-sensitive code"},
	{"deployment-security", []string{"deploy", "deployment", "workflow", "workflows", "terraform", "pulumi", "kubernetes", "k8s", "helm"}, "path indicates deployment or infrastructure control code"},
}

var contentRules = []struct {
	category string
	terms    []string
	why      string
}{
	{"access-control", []string{"authorize(", "authorise(", "permission", "rbac", "access token", "bearer token"}, "file content contains access-control terminology"},
	{"secrets", []string{"password", "client_secret", "private key", "api key", "credential"}, "file content contains secret/credential-handling terminology"},
	{"cryptography", []string{"encrypt(", "decrypt(", "x509", "tls.config", "rsa.", "ecdsa.", "ed25519.", "aes."}, "file content contains cryptographic API terminology"},
	{"financial", []string{"payment", "checkout", "credit card", "invoice", "ledger"}, "file content contains financial-processing terminology"},
}

func Analyze(repoRoot, path string) Report {
	rel := filepath.ToSlash(filepath.Clean(path))
	lowerPath := strings.ToLower(rel)
	r := Report{Generated: isGenerated(lowerPath), Vendor: isVendor(lowerPath)}
	seen := map[string]bool{}
	add := func(f Finding) {
		key := f.Category + "|" + f.Source + "|" + f.Match
		if seen[key] {
			return
		}
		seen[key] = true
		r.Findings = append(r.Findings, f)
	}
	for _, rule := range pathRules {
		for _, term := range rule.terms {
			if pathHasTerm(lowerPath, term) {
				add(Finding{Category: rule.category, Source: "path", Match: term, Explanation: rule.why})
				break
			}
		}
	}
	full := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if data, err := os.ReadFile(full); err == nil && len(data) <= 2<<20 {
		lower := strings.ToLower(string(data))
		for _, rule := range contentRules {
			for _, term := range rule.terms {
				if strings.Contains(lower, term) {
					add(Finding{Category: rule.category, Source: "content-keyword", Match: term, Explanation: rule.why})
					break
				}
			}
		}
	}
	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Category != r.Findings[j].Category {
			return r.Findings[i].Category < r.Findings[j].Category
		}
		if r.Findings[i].Source != r.Findings[j].Source {
			return r.Findings[i].Source < r.Findings[j].Source
		}
		return r.Findings[i].Match < r.Findings[j].Match
	})
	r.Sensitive = len(r.Findings) > 0
	return r
}

func isGenerated(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(path, "/generated/") || strings.HasPrefix(path, "generated/") ||
		strings.HasSuffix(base, ".pb.go") || strings.HasSuffix(base, ".gen.go") ||
		strings.HasSuffix(base, "_generated.go") || strings.Contains(base, ".generated.") ||
		strings.HasSuffix(base, ".g.cs") || strings.HasSuffix(base, ".designer.cs")
}

func isVendor(path string) bool {
	return strings.HasPrefix(path, "vendor/") || strings.Contains(path, "/vendor/") ||
		strings.HasPrefix(path, "node_modules/") || strings.Contains(path, "/node_modules/") ||
		strings.HasPrefix(path, "third_party/") || strings.Contains(path, "/third_party/")
}

func pathHasTerm(path, term string) bool {
	parts := strings.FieldsFunc(path, func(r rune) bool {
		switch r {
		case '/', '\\', '.', '-', '_':
			return true
		default:
			return false
		}
	})
	for _, part := range parts {
		if part == term {
			return true
		}
		// Prefix/suffix matching is useful for compound path segments such as
		// paymentservice, but short terms like auth/acl otherwise create noisy
		// false positives (for example, author).
		if len(term) >= 6 && (strings.HasPrefix(part, term) || strings.HasSuffix(part, term)) {
			return true
		}
	}
	return false
}
