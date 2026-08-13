package tools

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/vaadin/agent-tools/internal/tool"
)

// CreateProject bootstraps a new Vaadin project by downloading a skeleton from
// start.vaadin.com and extracting it. It mirrors the core of create-vaadin
// (`npm init vaadin`): GET https://start.vaadin.com/skeleton?<params> and unzip
// with the top-level directory stripped. The interactive prompts and IDE launch
// of create-vaadin are deliberately omitted — an agent passes flags instead.
var CreateProject = tool.Descriptor{
	Name:    "create-project",
	Summary: "Bootstrap a new Vaadin project from start.vaadin.com.",
	Usage: `vaadin-agent-tools create-project <targetDir> [flags]

Downloads a fresh Vaadin project skeleton from https://start.vaadin.com and
extracts it into <targetDir>. Mirrors ` + "`npm init vaadin`" + ` (create-vaadin).

Arguments:
  targetDir      Directory to create the project in.

Flags:
  --name=<id>    Maven artifactId (default: sanitized targetDir basename)
  --example=<v>  Example view to include: "flow" (Task List) or "none"
                 (default: flow)
  --pre          Use the pre-release Vaadin platform version
  --overwrite    If targetDir exists and is non-empty, replace its contents

Exit codes:
  0  project created
  1  download or extraction failed
  2  usage error (missing targetDir, or it exists without --overwrite)`,
	Run: runCreateProject,
}

// createReport is the typed result. The json-tagged fields form the JSON
// payload; OK/UsageError are CLI control fields.
type createReport struct {
	OK         bool   `json:"-"`
	UsageError string `json:"-"`

	Project     string `json:"project"`
	Directory   string `json:"directory"`
	Example     string `json:"example"`
	PreRelease  bool   `json:"preRelease"`
	Files       int    `json:"files"`
	SkeletonURL string `json:"skeletonUrl"`
	Error       string `json:"error,omitempty"`
}

func runCreateProject(args tool.Args) tool.Result {
	r := createProject(args)
	if r.UsageError != "" {
		return tool.Result{UsageError: r.UsageError}
	}
	return tool.Result{OK: r.OK, Payload: r, Human: renderCreateHuman(r)}
}

func createProject(args tool.Args) createReport {
	positionals, flags := splitFlags(args.Positionals)
	if len(positionals) == 0 {
		return createReport{UsageError: "Missing target directory. Usage: create-project <targetDir> [flags]"}
	}

	target := positionals[0]
	if !filepath.IsAbs(target) {
		target = filepath.Join(args.Cwd, target)
	}

	name := flags["name"]
	if name == "" {
		name = filepath.Base(target)
	}
	name = sanitizeName(name)
	if name == "" {
		return createReport{UsageError: "Could not derive a valid project name; pass --name=<id>."}
	}

	example := flags["example"]
	if example == "" {
		example = "flow"
	}
	if example != "flow" && example != "none" {
		return createReport{UsageError: `--example must be "flow" or "none"`}
	}
	pre := isSet(flags, "pre")
	overwrite := isSet(flags, "overwrite")

	// Target directory precondition (mirrors create-vaadin's overwrite handling).
	if info, err := os.Stat(target); err == nil {
		if !info.IsDir() {
			return createReport{UsageError: "Target exists and is not a directory: " + target}
		}
		entries, _ := os.ReadDir(target)
		if len(entries) > 0 {
			if !overwrite {
				return createReport{UsageError: "Target directory already exists and is not empty: " + target + " (pass --overwrite to replace its contents)"}
			}
			for _, e := range entries {
				if err := os.RemoveAll(filepath.Join(target, e.Name())); err != nil {
					return createReport{UsageError: "Could not clear target directory: " + err.Error()}
				}
			}
		}
	}

	skeletonURL := buildSkeletonURL(name, example, pre)
	base := createReport{Project: name, Directory: target, Example: example, PreRelease: pre, SkeletonURL: skeletonURL}

	zipBytes, err := download(skeletonURL)
	if err != nil {
		base.Error = err.Error()
		return base
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		base.Error = "could not create target directory: " + err.Error()
		return base
	}
	count, err := extractZip(zipBytes, target)
	if err != nil {
		base.Error = "extraction failed: " + err.Error()
		return base
	}

	base.OK = true
	base.Files = count
	return base
}

// --- helpers ----------------------------------------------------------------

var nonNameChars = regexp.MustCompile(`[^a-zA-Z0-9-_]`)

// sanitizeName strips everything but [A-Za-z0-9-_], mirroring create-vaadin's
// sanitizePath.
func sanitizeName(s string) string {
	return nonNameChars.ReplaceAllString(strings.TrimSpace(s), "")
}

// buildSkeletonURL assembles the start.vaadin.com/skeleton URL. Query parameters
// mirror create-vaadin: artifactId, ref=agent-tools, optional frameworks=flow, optional
// platformVersion=pre.
func buildSkeletonURL(artifactID, example string, pre bool) string {
	q := url.Values{}
	q.Set("artifactId", artifactID)
	q.Set("ref", "agent-tools")
	if example == "flow" {
		q.Set("frameworks", "flow")
	}
	if pre {
		q.Set("platformVersion", "pre")
	}
	return "https://start.vaadin.com/skeleton?" + q.Encode()
}

func download(u string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Vaadin CLI")
	// Let net/http negotiate and transparently decompress transfer encoding.

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return io.ReadAll(resp.Body)
	case http.StatusNotFound:
		return nil, fmt.Errorf("preset not found (HTTP 404) for %s", u)
	default:
		return nil, fmt.Errorf("unable to download project: HTTP %d", resp.StatusCode)
	}
}

// stripFirst removes the leading path component (the archive's top-level
// directory), matching create-vaadin's decompress `strip: 1`. Returns "" when
// nothing remains (e.g. the top-level directory entry itself).
func stripFirst(name string) string {
	name = strings.TrimPrefix(name, "./")
	i := strings.IndexByte(name, '/')
	if i < 0 {
		return ""
	}
	return name[i+1:]
}

// extractZip writes the archive into target with the top-level directory
// stripped, and returns the number of files written. It rejects entries that
// would escape target (zip-slip) and preserves file modes (so mvnw stays
// executable) and empty files (so an empty theme styles.css survives).
func extractZip(data []byte, target string) (int, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, f := range zr.File {
		rel := stripFirst(f.Name)
		if rel == "" {
			continue
		}
		dest := filepath.Join(target, filepath.FromSlash(rel))

		absDest, err := filepath.Abs(dest)
		if err != nil {
			return count, err
		}
		if absDest != absTarget && !strings.HasPrefix(absDest, absTarget+string(os.PathSeparator)) {
			return count, fmt.Errorf("unsafe path in archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return count, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return count, err
		}
		if err := writeZipFile(f, dest); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func writeZipFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := f.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// splitFlags separates "--key=value" / "--flag" tokens from positional
// arguments. Bare flags map to "true".
func splitFlags(args []string) (positionals []string, flags map[string]string) {
	flags = map[string]string{}
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			kv := strings.TrimPrefix(a, "--")
			if i := strings.IndexByte(kv, '='); i >= 0 {
				flags[kv[:i]] = kv[i+1:]
			} else {
				flags[kv] = "true"
			}
			continue
		}
		positionals = append(positionals, a)
	}
	return positionals, flags
}

func isSet(flags map[string]string, key string) bool {
	v, ok := flags[key]
	return ok && v != "false"
}

func renderCreateHuman(r createReport) string {
	if r.Error != "" {
		return "# create-project\n\n✗ Failed to create project: " + r.Error +
			"\n  Skeleton URL: " + r.SkeletonURL
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# create-project\n\n")
	fmt.Fprintf(&b, "✓ Project '%s' created in %s\n", r.Project, r.Directory)
	fmt.Fprintf(&b, "  example view: %s\n", r.Example)
	if r.PreRelease {
		fmt.Fprintf(&b, "  pre-release:  yes\n")
	}
	fmt.Fprintf(&b, "  files written: %d\n\n", r.Files)
	fmt.Fprintf(&b, "Next: cd %s && mvn", r.Directory)
	return b.String()
}
