// Command update-foreign-resources bumps npm-sourced entries in a MediaWiki
// foreign-resources.yaml.
//
// For every component whose src points at registry.npmjs.org, it queries the
// npm registry for the latest published version. When a newer version exists,
// it rewrites the component's version, src and integrity lines in place. All
// other lines — including comments — are left byte-for-byte untouched, which
// a YAML parse/dump round-trip would not guarantee.
//
// Components with non-npm sources are skipped with a notice. Regenerating the
// bundled files themselves is left to MediaWiki's manageForeignResources
// maintenance script, which validates downloads against the integrity hashes
// written here. Known limitation: purl fields (used only by core's make-cdx
// SBOM action, not by update/verify) are not rewritten.
//
// Outputs changed=true|false to $GITHUB_OUTPUT when set — plus, when
// something changed, a Dependabot-style title fragment naming the bumped
// packages and versions — and an optional markdown summary (--summary-file)
// suitable for a pull request body.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const registry = "https://registry.npmjs.org"

var (
	componentRE = regexp.MustCompile(`^([A-Za-z0-9_.-]+):\s*(?:#.*)?$`)
	fieldRE     = regexp.MustCompile(`^(\s+)(version|src|integrity):\s*(.*?)\s*$`)
	npmSrcRE    = regexp.MustCompile(`^https://registry\.npmjs\.org/(.+?)/-/[^/]+\.tgz$`)
	semverRE    = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
)

type field struct {
	line   int
	indent string
	value  string
}

type component struct {
	name   string
	fields map[string]field
}

// packument is the subset of the npm registry's abbreviated package
// metadata that this tool needs.
type packument struct {
	DistTags map[string]string `json:"dist-tags"`
	Versions map[string]struct {
		Dist struct {
			Tarball   string `json:"tarball"`
			Integrity string `json:"integrity"`
		} `json:"dist"`
	} `json:"versions"`
}

type update struct {
	name, pkg, old, new string
}

// parseComponents maps each top-level component to its
// version/src/integrity lines.
func parseComponents(lines []string) []component {
	var components []component
	current := -1
	for i, line := range lines {
		if m := componentRE.FindStringSubmatch(line); m != nil {
			components = append(components, component{name: m[1], fields: map[string]field{}})
			current = len(components) - 1
			continue
		}
		if current < 0 {
			continue
		}
		if m := fieldRE.FindStringSubmatch(line); m != nil {
			fields := components[current].fields
			if _, seen := fields[m[2]]; !seen {
				fields[m[2]] = field{line: i, indent: m[1], value: m[3]}
			}
		}
	}
	return components
}

func fetchPackument(pkg string) (*packument, error) {
	req, err := http.NewRequest(http.MethodGet, registry+"/"+url.PathEscape(pkg), nil)
	if err != nil {
		return nil, err
	}
	// Abbreviated packument: full metadata for big packages runs to
	// tens of MB; this variant still includes per-version dist info.
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json")
	req.Header.Set("User-Agent", "mediawiki-ci-workflows/update-foreign-resources")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	var p packument
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// semver parses a strict X.Y.Z version; ok is false for anything else
// (prereleases, build metadata, partial versions).
func semver(value string) (v [3]int, ok bool) {
	m := semverRE.FindStringSubmatch(value)
	if m == nil {
		return v, false
	}
	for i := range 3 {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	return v, true
}

func newer(a, b [3]int) bool {
	for i := range 3 {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// titleFragment builds a Dependabot-style PR title fragment (without the
// conventional-commit prefix) from the applied updates.
func titleFragment(updates []update) string {
	switch len(updates) {
	case 1:
		u := updates[0]
		return fmt.Sprintf("bump %s from %s to %s", u.pkg, u.old, u.new)
	case 2:
		return fmt.Sprintf("bump %s to %s and %s to %s",
			updates[0].pkg, updates[0].new, updates[1].pkg, updates[1].new)
	default:
		return fmt.Sprintf("bump %d foreign resources", len(updates))
	}
}

func writeSummary(path string, updates []update) {
	var b strings.Builder
	b.WriteString("Updates the following foreign resources to their latest npm release:\n\n")
	b.WriteString("| Component | Package | From | To |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, u := range updates {
		fmt.Fprintf(&b, "| %s | [%s](https://www.npmjs.com/package/%s) | %s | %s |\n",
			u.name, u.pkg, u.pkg, u.old, u.new)
	}
	b.WriteString("\nBundled files were regenerated with MediaWiki's " +
		"`manageForeignResources` maintenance script and verified " +
		"against the updated integrity hashes.\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		fatalf("cannot write %s: %v", path, err)
	}
}

func main() {
	summaryFile := flag.String("summary-file", "",
		"Write a markdown summary of the changes to this path")
	flag.Parse()
	if flag.NArg() != 1 {
		fatalf("usage: update-foreign-resources [--summary-file PATH] <foreign-resources.yaml>")
	}
	yamlPath := flag.Arg(0)

	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		fatalf("%v", err)
	}
	lines := strings.Split(string(raw), "\n")

	var updates []update
	for _, c := range parseComponents(lines) {
		version, hasVersion := c.fields["version"]
		src, hasSrc := c.fields["src"]
		_, hasIntegrity := c.fields["integrity"]
		if !hasVersion || !hasSrc || !hasIntegrity {
			fmt.Printf("%s: no version/src/integrity fields, skipping\n", c.name)
			continue
		}
		m := npmSrcRE.FindStringSubmatch(src.value)
		if m == nil {
			fmt.Printf("%s: src is not an npm registry tarball, skipping\n", c.name)
			continue
		}
		pkg := m[1]
		current, ok := semver(version.value)
		if !ok {
			fmt.Printf("%s: cannot parse pinned version %q, skipping\n", c.name, version.value)
			continue
		}

		p, err := fetchPackument(pkg)
		if err != nil {
			fatalf("%s: npm registry request for %s failed: %v", c.name, pkg, err)
		}
		latestStr := p.DistTags["latest"]
		latest, ok := semver(latestStr)
		if !ok {
			fmt.Printf("%s: cannot parse latest version %q of %s, skipping\n", c.name, latestStr, pkg)
			continue
		}
		if !newer(latest, current) {
			fmt.Printf("%s: %s %s is up to date\n", c.name, pkg, version.value)
			continue
		}

		dist := p.Versions[latestStr].Dist
		if dist.Tarball == "" || dist.Integrity == "" {
			fatalf("%s: registry metadata for %s@%s is missing dist tarball/integrity",
				c.name, pkg, latestStr)
		}
		for _, r := range []struct{ key, value string }{
			{"version", latestStr},
			{"src", dist.Tarball},
			{"integrity", dist.Integrity},
		} {
			f := c.fields[r.key]
			lines[f.line] = f.indent + r.key + ": " + r.value
		}
		updates = append(updates, update{name: c.name, pkg: pkg, old: version.value, new: latestStr})
		fmt.Printf("%s: %s %s -> %s\n", c.name, pkg, version.value, latestStr)
	}

	if len(updates) > 0 {
		if err := os.WriteFile(yamlPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			fatalf("cannot write %s: %v", yamlPath, err)
		}
		if *summaryFile != "" {
			writeSummary(*summaryFile, updates)
		}
	}

	if out := os.Getenv("GITHUB_OUTPUT"); out != "" {
		f, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fatalf("cannot open GITHUB_OUTPUT: %v", err)
		}
		defer f.Close()
		fmt.Fprintf(f, "changed=%t\n", len(updates) > 0)
		if len(updates) > 0 {
			fmt.Fprintf(f, "title=%s\n", titleFragment(updates))
		}
	}
}
