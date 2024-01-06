package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
)

// readDevSpaceCharts returns the charts listed in the provided devspace.yaml
// reader.
func readDevSpaceCharts(r io.Reader) ([]chart, error) {
	var v struct {
		Deployments map[string]struct {
			Helm struct {
				Chart struct {
					Name    string `yaml:"name"`
					Repo    string `yaml:"repo"`
					Version string `yaml:"version"`
				}
			} `yaml:"helm"`
		} `yaml:"deployments"`
	}
	if err := yaml.NewDecoder(r).Decode(&v); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	var charts []chart
	for _, deployment := range v.Deployments {
		if len(deployment.Helm.Chart.Repo) > 0 && len(deployment.Helm.Chart.Version) > 0 {
			charts = append(charts, chart{
				repository: deployment.Helm.Chart.Repo,
				name:       deployment.Helm.Chart.Name,
				version:    deployment.Helm.Chart.Version,
			})
		}
	}
	return charts, nil
}
