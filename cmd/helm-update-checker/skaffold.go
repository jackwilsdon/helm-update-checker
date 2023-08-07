package main

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
)

// readSkaffoldCharts returns the charts listed in the provided skaffold.yaml
// reader.
func readSkaffoldCharts(r io.Reader) ([]chart, error) {
	type helm struct {
		Releases []struct {
			Repo        string `yaml:"repo"`
			RemoteChart string `yaml:"remoteChart"`
			Version     string `yaml:"version"`
		} `yaml:"releases"`
	}
	var v struct {
		Manifests struct {
			Helm helm `yaml:"helm"`
		} `yaml:"manifests"`
		Deploy struct {
			Helm helm `yaml:"helm"`
		} `yaml:"deploy"`
	}
	decoder := yaml.NewDecoder(r)
	var charts []chart
	for {
		if err := decoder.Decode(&v); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to unmarshal: %w", err)
		}
		for _, release := range v.Deploy.Helm.Releases {
			if len(release.RemoteChart) > 0 {
				charts = append(charts, chart{
					repository: release.Repo,
					name:       release.RemoteChart,
					version:    release.Version,
				})
			}
		}
		for _, release := range v.Manifests.Helm.Releases {
			if len(release.RemoteChart) > 0 {
				charts = append(charts, chart{
					repository: release.Repo,
					name:       release.RemoteChart,
					version:    release.Version,
				})
			}
		}
	}
	return charts, nil
}
