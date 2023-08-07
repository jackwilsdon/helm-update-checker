package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type chart struct {
	path       string
	repository string
	name       string
	version    string
}

var chartReaders = map[string]func(r io.Reader) ([]chart, error){
	"Chart.yaml":    readHelmSubcharts,
	"helmfile.yaml": readHelmfileCharts,
	"skaffold.yaml": readSkaffoldCharts,
}

// getCharts returns all charts in the provided path.
func getCharts(path string) ([]chart, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", path, err)
	}

	var charts []chart
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		var entryCharts []chart
		if entry.IsDir() {
			if entryCharts, err = getCharts(entryPath); err != nil {
				return nil, err
			}
		} else if reader, ok := chartReaders[entry.Name()]; ok {
			err := func() error {
				f, err := os.Open(entryPath)
				if err != nil {
					return fmt.Errorf("failed to open %q: %w", entryPath, err)
				}
				defer func() {
					_ = f.Close()
				}()
				entryCharts, err = reader(f)
				if err != nil {
					return fmt.Errorf("failed to read %q: %w", entryPath, err)
				}
				for i := range entryCharts {
					entryCharts[i].path = entryPath
				}
				return nil
			}()
			if err != nil {
				return nil, err
			}
		}
		charts = append(charts, entryCharts...)
	}
	return charts, nil
}

// parseVersion parses a semantic version string into its major, minor and
// patch values.
func parseVersion(v string) (int, int, int, error) {
	pieces := strings.SplitN(v, ".", 3)
	if len(pieces) != 3 {
		return 0, 0, 0, fmt.Errorf("failed to parse version %q: not in the form X.Y.Z", v)
	}
	major, err := strconv.Atoi(pieces[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse version %q: bad major version %q: %w", v, pieces[0], err)
	}
	minor, err := strconv.Atoi(pieces[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse version %q: bad minor version %q: %w", v, pieces[1], err)
	}
	patch, err := strconv.Atoi(pieces[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse version %q: bad minor version %q: %w", v, pieces[2], err)
	}
	return major, minor, patch, nil
}

// compareVersions returns -1 if a < b, 0 if a == b and 1 if a > b, where a and
// b are semantic version numbers.
func compareVersions(a, b string) (int, error) {
	aMajor, aMinor, aPatch, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	bMajor, bMinor, bPatch, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	if aMajor > bMajor || (aMajor == bMajor && aMinor > bMinor) || (aMajor == bMajor && aMinor == bMinor && aPatch > bPatch) {
		return 1, nil
	} else if bMajor > aMajor || (bMajor == aMajor && bMinor > aMinor) || (bMajor == aMajor && bMinor == aMinor && bPatch > aPatch) {
		return -1, nil
	} else {
		return 0, nil
	}
}

// getRepositoryLatestChartVersions returns the latest version for each chart
// at repositoryURL.
func getRepositoryLatestChartVersions(repositoryURL string) (map[string]string, error) {
	indexURL, err := url.JoinPath(repositoryURL, "index.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to get index URL for %q: %w", repositoryURL, err)
	}
	res, err := http.Get(indexURL)
	if err != nil {
		return nil, fmt.Errorf("failed to GET %q: %w", indexURL, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status code to be %d, got %d for %q", http.StatusOK, res.StatusCode, indexURL)
	}
	var v struct {
		Entries map[string][]struct {
			Version string `yaml:"version"`
		} `yaml:"entries"`
	}
	if err := yaml.NewDecoder(res.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response from %q: %w", indexURL, err)
	}
	versions := make(map[string]string)
	for name, entry := range v.Entries {
		var entryVersions []string
		for _, release := range entry {
			// Ignore beta, alpha, etc.
			if !strings.Contains(release.Version, "-") {
				entryVersions = append(entryVersions, release.Version)
			}
		}
		if len(entryVersions) == 0 {
			continue
		}

		var err error
		sort.Slice(entryVersions, func(i, j int) bool {
			var c int
			c, err = compareVersions(entryVersions[i], entryVersions[j])
			if err != nil {
				return false
			}
			return c < 0
		})
		if err != nil {
			return nil, fmt.Errorf("failed to sort versions: %w", err)
		}
		versions[name] = entryVersions[len(entryVersions)-1]
	}
	return versions, nil
}

// getLatestChartVersions returns the latest version for each of the chart in
// charts.
func getLatestChartVersions(charts []chart) (map[chart]string, error) {
	repositoryChartVersions := make(map[string]map[string]string)
	for _, chart := range charts {
		if _, ok := repositoryChartVersions[chart.repository]; ok {
			// We've already fetched this repository.
			continue
		}

		var err error
		repositoryChartVersions[chart.repository], err = getRepositoryLatestChartVersions(chart.repository)
		if err != nil {
			return nil, err
		}
	}

	chartVersions := make(map[chart]string)
	for _, chart := range charts {
		version, ok := repositoryChartVersions[chart.repository][chart.name]
		if !ok {
			return nil, fmt.Errorf("no version for chart %q in repository %q", chart.name, chart.repository)
		}
		chartVersions[chart] = version
	}
	return chartVersions, nil
}

func main() {
	if len(os.Args) != 1 && len(os.Args) != 2 {
		_, _ = fmt.Fprintf(os.Stderr, "usage: %s [path-to-directory]\n", os.Args[0])
		os.Exit(1)
	}

	var path string
	var err error
	if len(os.Args) == 2 {
		path, err = filepath.Abs(os.Args[1])
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: failed to get absolute path of %q: %s\n", os.Args[1], err)
			os.Exit(1)
		}
	} else {
		path, err = os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: failed to get working directory: %s\n", err)
			os.Exit(1)
		}
	}

	charts, err := getCharts(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: failed to get charts: %s\n", err)
		os.Exit(1)
	}

	latestChartVersions, err := getLatestChartVersions(charts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: failed to get latest chart versions: %s\n", err)
		os.Exit(1)
	}

	for chart, latestVersion := range latestChartVersions {
		if chart.version != latestVersion {
			fmt.Printf("%s: %s %s %s -> %s\n", chart.path, chart.repository, chart.name, chart.version, latestVersion)
		}
	}
}
