package main

import (
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"regexp"
	"strings"
)

var goTemplateRegexp = regexp.MustCompile("{{.*?}}")

// readHelmfileCharts returns the charts listed in the provided helmfile.yaml
// reader.
func readHelmfileCharts(r io.Reader) ([]chart, error) {
	contents, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}
	// Strip all Go template actions. This may end up generating invalid YAML
	// if template actions are being used to generate YAML, but it's simpler
	// than trying to evaluate the template.
	contents = goTemplateRegexp.ReplaceAll(contents, nil)
	var v struct {
		Repositories []struct {
			Name string `yaml:"name"`
			URL  string `yaml:"url"`
		} `yaml:"repositories"`
		Releases []struct {
			Chart   string `yaml:"chart"`
			Version string `yaml:"version"`
		} `yaml:"releases"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	var repositories map[string]string
	var charts []chart
	for {
		if err := decoder.Decode(&v); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to unmarshal: %w", err)
		}

		// Helmfile only uses the last occurrence of a property.
		repositories = make(map[string]string)
		for _, repository := range v.Repositories {
			repositories[repository.Name] = repository.URL
		}

		charts = charts[:0]
		for _, release := range v.Releases {
			idx := strings.IndexByte(release.Chart, '/')
			if idx == -1 {
				continue
			}
			repositoryName := release.Chart[:idx]
			repositoryURL, ok := repositories[repositoryName]
			if !ok {
				continue
			}
			charts = append(charts, chart{
				repository: repositoryURL,
				name:       release.Chart[idx+1:],
				version:    release.Version,
			})
		}
	}
	return charts, nil
}
