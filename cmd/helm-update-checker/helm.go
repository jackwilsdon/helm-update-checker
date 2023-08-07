package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"strings"
)

// readHelmSubcharts returns the subcharts listed in the provided Chart.yaml
// reader.
func readHelmSubcharts(r io.Reader) ([]chart, error) {
	var v struct {
		Dependencies []struct {
			Name       string `yaml:"name"`
			Repository string `yaml:"repository"`
			Version    string `yaml:"version"`
		} `yaml:"dependencies"`
	}
	if err := yaml.NewDecoder(r).Decode(&v); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	var charts []chart
	for _, dependency := range v.Dependencies {
		if strings.HasPrefix(dependency.Repository, "http") {
			charts = append(charts, chart{
				repository: dependency.Repository,
				name:       dependency.Name,
				version:    dependency.Version,
			})
		}
	}
	return charts, nil
}
