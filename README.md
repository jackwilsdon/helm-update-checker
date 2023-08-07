# helm-update-checker
Check for updates to your Helm charts. Currently supports checking the 
following:
 * Helm subcharts (`dependencies` in `Chart.yaml`)
 * Helmfile charts (template actions are not evaluated)
 * Skaffold charts (both `manifest` and `deploy`)

## Usage
```shell
$ go install github.com/jackwilsdon/helm-update-checker/cmd/helm-update-checker@latest
$ helm-update-checker [path-to-directory]
```
