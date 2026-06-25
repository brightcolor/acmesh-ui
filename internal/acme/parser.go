package acme

import (
	"bufio"
	"encoding/base64"
	"strings"
)

// ParseVersion extracts the acme.sh version from `--version` output, which
// looks like:
//
//	https://github.com/acmesh-official/acme.sh
//	v3.0.7
func ParseVersion(out string) string {
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "v") && len(line) > 1 && (line[1] >= '0' && line[1] <= '9') {
			return line
		}
	}
	return strings.TrimSpace(out)
}

// ListRow is one entry from `acme.sh --list`.
type ListRow struct {
	MainDomain string
	KeyLength  string
	SAN        []string
	CA         string
	Created    string
	Renew      string
}

// ParseList parses the tabular output of `acme.sh --list`. The columns are:
//
//	Main_Domain  KeyLength  SAN_Domains  CA  Created  Renew
//
// acme.sh separates columns with whitespace; SAN domains are comma separated.
func ParseList(out string) []ListRow {
	var rows []ListRow
	sc := bufio.NewScanner(strings.NewReader(out))
	first := true
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if first {
			first = false
			// Skip the header row if present.
			if len(fields) > 0 && strings.EqualFold(fields[0], "Main_Domain") {
				continue
			}
		}
		if len(fields) == 0 {
			continue
		}
		row := ListRow{MainDomain: fields[0]}
		if len(fields) > 1 {
			row.KeyLength = fields[1]
		}
		if len(fields) > 2 && fields[2] != "no" {
			row.SAN = splitSAN(fields[2])
		}
		if len(fields) > 3 {
			row.CA = fields[3]
		}
		rows = append(rows, row)
	}
	return rows
}

func splitSAN(s string) []string {
	if s == "" || s == "no" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseDomainConf parses an acme.sh per-domain ".conf" file into a flat map of
// KEY=value pairs (quotes stripped). These files use shell-style assignments.
func ParseDomainConf(content string) map[string]string {
	conf := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, "'\"")
		conf[key] = DecodeAcmeValue(val)
	}
	return conf
}

// DecodeAcmeValue decodes acme.sh's base64-wrapped config values. acme.sh stores
// values that may contain special characters (notably the install reload
// command) as `__ACME_BASE64__START_<base64>__ACME_BASE64__END_`. Plain values
// are returned unchanged.
func DecodeAcmeValue(v string) string {
	const start = "__ACME_BASE64__START_"
	const end = "__ACME_BASE64__END_"
	if strings.HasPrefix(v, start) && strings.HasSuffix(v, end) {
		b64 := v[len(start) : len(v)-len(end)]
		if dec, err := base64.StdEncoding.DecodeString(b64); err == nil {
			return strings.TrimRight(string(dec), "\n")
		}
	}
	return v
}
