// Package dnsproviders manages acme.sh DNS-01 provider configurations. Secret
// env values are encrypted at rest and never returned in clear text.
package dnsproviders

import "time"

// Provider is a stored DNS-01 configuration.
type Provider struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"` // acme.sh provider code, e.g. dns_cf
	Description string    `json:"description,omitempty"`
	Env         []EnvVar  `json:"env"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EnvVar is a single environment variable for the provider. Secret values are
// stored encrypted; non-secret values are stored in clear.
type EnvVar struct {
	Name   string `json:"name"`
	Value  string `json:"value"` // encrypted if Secret, plain otherwise
	Secret bool   `json:"secret"`
}

// KnownProvider is a built-in provider definition used to populate the UI.
type KnownProvider struct {
	Code        string   `json:"code"`
	Label       string   `json:"label"`
	SecretVars  []string `json:"secret_vars"`
	PlainVars   []string `json:"plain_vars"`
	Description string   `json:"description"`
}

// KnownProviders is a curated static list complementing whatever is discovered
// in the acme.sh dnsapi directory.
var KnownProviders = []KnownProvider{
	{Code: "dns_cf", Label: "Cloudflare", SecretVars: []string{"CF_Token"}, PlainVars: []string{"CF_Account_ID", "CF_Zone_ID"}, Description: "Cloudflare API token (scoped Zone.DNS edit)"},
	{Code: "dns_hetzner", Label: "Hetzner DNS", SecretVars: []string{"HETZNER_Token"}, Description: "Hetzner DNS Console API token"},
	{Code: "dns_inwx", Label: "INWX", SecretVars: []string{"INWX_Password"}, PlainVars: []string{"INWX_User"}, Description: "INWX account credentials"},
	{Code: "dns_aws", Label: "AWS Route53", SecretVars: []string{"AWS_SECRET_ACCESS_KEY"}, PlainVars: []string{"AWS_ACCESS_KEY_ID"}, Description: "AWS Route53 IAM credentials"},
	{Code: "dns_gandi_livedns", Label: "Gandi LiveDNS", SecretVars: []string{"GANDI_LIVEDNS_KEY"}, Description: "Gandi LiveDNS API key"},
	{Code: "dns_namecheap", Label: "Namecheap", SecretVars: []string{"NAMECHEAP_API_KEY"}, PlainVars: []string{"NAMECHEAP_USERNAME", "NAMECHEAP_SOURCEIP"}, Description: "Namecheap API access"},
	{Code: "dns_ovh", Label: "OVH", SecretVars: []string{"OVH_AK", "OVH_AS", "OVH_CK"}, PlainVars: []string{"OVH_END_POINT"}, Description: "OVH API application keys"},
	{Code: "dns_duckdns", Label: "DuckDNS", SecretVars: []string{"DuckDNS_Token"}, Description: "DuckDNS token"},
	{Code: "dns_desec", Label: "deSEC", SecretVars: []string{"DEDYN_TOKEN"}, PlainVars: []string{"DEDYN_NAME"}, Description: "deSEC.io token"},
	{Code: "dns_netcup", Label: "netcup", SecretVars: []string{"NC_Apipw", "NC_Apikey"}, PlainVars: []string{"NC_CID"}, Description: "netcup CCP API"},
}

// FindKnown returns the built-in definition for a provider code, if any.
func FindKnown(code string) (KnownProvider, bool) {
	for _, p := range KnownProviders {
		if p.Code == code {
			return p, true
		}
	}
	return KnownProvider{}, false
}
