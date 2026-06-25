package dnsproviders

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/secrets"
)

// savedPrefix is how acme.sh stores reusable DNS credentials in account.conf.
const savedPrefix = "SAVED_"

// DetectedVar is one credential variable found in account.conf.
type DetectedVar struct {
	Name        string `json:"name"`
	Secret      bool   `json:"secret"`
	MaskedValue string `json:"masked_value"`
}

// Detected is a DNS provider recognised from acme.sh's account.conf.
type Detected struct {
	Code        string        `json:"code"`
	Label       string        `json:"label"`
	Description string        `json:"description,omitempty"`
	Vars        []DetectedVar `json:"vars"`
}

// readSavedVars reads <home>/account.conf and returns the SAVED_* variables with
// the prefix stripped (real env name -> value).
func readSavedVars(home string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(home, "account.conf"))
	if err != nil {
		return nil, err
	}
	conf := acme.ParseDomainConf(string(data))
	out := make(map[string]string)
	for k, v := range conf {
		if strings.HasPrefix(k, savedPrefix) {
			out[strings.TrimPrefix(k, savedPrefix)] = v
		}
	}
	return out, nil
}

// DetectDNS returns DNS providers detectable from acme.sh's account.conf, with
// secret values masked. A missing account.conf yields an empty list, not an error.
func DetectDNS(home string) ([]Detected, error) {
	saved, err := readSavedVars(home)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Detected
	for _, kp := range KnownProviders {
		d := Detected{Code: kp.Code, Label: kp.Label, Description: kp.Description}
		present := false
		add := func(name string, secret bool) {
			val, ok := saved[name]
			if !ok {
				return
			}
			present = true
			masked := val
			if secret {
				masked = secrets.MaskValue(val)
			}
			d.Vars = append(d.Vars, DetectedVar{Name: name, Secret: secret, MaskedValue: masked})
		}
		for _, n := range kp.SecretVars {
			add(n, true)
		}
		for _, n := range kp.PlainVars {
			add(n, false)
		}
		if present {
			out = append(out, d)
		}
	}
	return out, nil
}

// ImportFromAccountConf reads the SAVED_ values for the given provider code and
// stores them as a managed (encrypted) provider.
func (s *Store) ImportFromAccountConf(home, code string) (Provider, error) {
	kp, ok := FindKnown(code)
	if !ok {
		return Provider{}, fmt.Errorf("unknown provider code %q", code)
	}
	saved, err := readSavedVars(home)
	if err != nil {
		return Provider{}, fmt.Errorf("read account.conf: %w", err)
	}
	in := Input{
		Name:        kp.Label,
		Code:        code,
		Description: "Imported from acme.sh account.conf",
		Source:      SourceManaged,
		Env:         make(map[string]string),
	}
	for _, n := range kp.SecretVars {
		if v, ok := saved[n]; ok {
			in.Env[n] = v
			in.SecretNames = append(in.SecretNames, n)
		}
	}
	for _, n := range kp.PlainVars {
		if v, ok := saved[n]; ok {
			in.Env[n] = v
		}
	}
	if len(in.Env) == 0 {
		return Provider{}, fmt.Errorf("no saved variables found for %q in account.conf", code)
	}
	return s.Create(in)
}
