package notifications

// commonTemplates defines a map of predefined notification templates for different output formats.
// Each template is a Go template string that formats container update information.
// Templates can access either .Report (summary data with container lists) or .Entries (individual event logs).
// Container states include: Updated (successfully updated), Failed (update failed), Fresh (no update needed), Skipped (skipped for various reasons).
var commonTemplates = map[string]string{
	// "default-legacy" template formats individual event entries in a legacy log style.
	// It iterates over .Entries, checking each entry's Message to format specific container lifecycle events.
	// Handles messages: "Found new image" (new image available), "Stopping container" (stopping old container),
	// "Started new container" (new container started), "Stopping linked container" (stopping linked container),
	// "Started linked container" (linked container started), "Removing image" (image cleanup completed),
	// "Failed to list containers for image usage check, skipping removal" (image cleanup skipped),
	// "Container updated" (update completed),
	// "Image is within cooldown period - deferring update" (image too new for update), "Image age exceeds cooldown - eligible for update" (image old enough),
	// "Image creation time unavailable - update check unavailable" (image age could not be determined from registry),
	// "Detected multiple Watchtower instances - initiating cleanup" (multiple instances detected),
	// "Docker image usage exceeds configured maximum" (session blocked by image-usage budget),
	// "Docker image usage exceeds configured warning threshold" (image-usage budget warning),
	// "Failed to query Docker image disk usage" (image-usage query failed),
	// "Docker image usage budget enabled" (startup budget configuration),
	// "Registry rate limit retries exhausted. Container failed for this cycle" (update failed on a registry rate limit),
	// "Ignoring depends-on entry that names no container" (depends-on label entry matched nothing),
	// "Skipping container update (self-dependency)" (container depends on itself),
	// "Skipping container update (circular dependency)" (container is in a dependency cycle).
	// For unrecognized messages, displays the message with key=value data pairs if Data exists, otherwise just the message.
	// Expects .Entries []Entry where each Entry has Message string and Data map[string]interface{}.
	"default-legacy": `
{{- /* Iterate over entries, adding newline between them */ -}}
{{- range $i, $e := . -}}
{{- /* Add newline if not the first entry */ -}}
{{- if $i}}{{- println -}}{{- end -}}
{{- /* Extract message for conditional formatting */ -}}
{{- $msg := $e.Message -}}
{{- /* Format based on specific message types */ -}}
{{- if eq $msg "Found new image" -}}
    Found new image: {{index $e.Data "image"}} ({{with (index $e.Data "new_id")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Stopping container" -}}
    Stopped stale container: {{index $e.Data "container"}} ({{with (index $e.Data "id")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Started new container" -}}
    Started new container: {{index $e.Data "container"}} ({{with (index $e.Data "new_id")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Stopping linked container" -}}
    Stopped linked container: {{index $e.Data "container"}} ({{with (index $e.Data "id")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Started linked container" -}}
    Started linked container: {{index $e.Data "container"}} ({{with (index $e.Data "new_id")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Removing image" -}}
    Cleaned up old image: {{with (index $e.Data "image_name")}}{{.}}{{else}}unknown{{end}} ({{with (index $e.Data "image_id")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Failed to list containers for image usage check, skipping removal" -}}
    Skipped image cleanup: {{with (index $e.Data "image_name")}}{{.}}{{else}}unknown{{end}} ({{with (index $e.Data "image_id")}}{{.}}{{else}}unknown{{end}}){{with (index $e.Data "error")}}: {{.}}{{end}}
{{- else if eq $msg "Container updated" -}}
    Updated container: {{with (index $e.Data "container")}}{{.}}{{else}}unknown{{end}} ({{with (index $e.Data "image")}}{{.}}{{else}}unknown{{end}}): {{with (index $e.Data "old_id")}}{{.}}{{else}}unknown{{end}} updated to {{with (index $e.Data "new_id")}}{{.}}{{else}}unknown{{end}}
{{- else if eq $msg "Changelog" -}}
    Changelog: {{with (index $e.Data "changelog")}}{{.}}{{else}}unknown{{end}}
{{- else if eq $msg "Skipping Watchtower self-update in run-once mode" -}}
    Run once mode: Watchtower self-update skipped
{{- else if eq $msg "Detected multiple Watchtower instances - initiating cleanup" -}}
    Detected {{index $e.Data "count"}} Watchtower instances - initiating cleanup
	{{- else if eq $msg "Successfully removed all excess Watchtower containers" -}}
    Successfully removed {{index $e.Data "removed_instances"}} old Watchtower container{{if ne (index $e.Data "removed_instances") 1}}s{{end}}
{{- else if eq $msg "Image is within cooldown period - not eligible for update" -}}
    {{with (index $e.Data "image")}}{{.}}{{else}}unknown{{end}} created less than {{with (index $e.Data "cooldown")}}{{.}}{{else}}unknown{{end}} ago - eligible in {{with (index $e.Data "eligible_in")}}{{.}}{{else}}unknown{{end}} ({{with (index $e.Data "eligible_at")}}{{RFC1123 .}}{{else}}unknown{{end}})
{{- else if eq $msg "Image age exceeds cooldown - eligible for update" -}}
    {{with (index $e.Data "image")}}{{.}}{{else}}unknown{{end}} created more than {{with (index $e.Data "cooldown")}}{{.}}{{else}}unknown{{end}} ago - eligible for update
{{- else if eq $msg "Image creation time unavailable - update check unavailable" -}}
    {{with (index $e.Data "image")}}{{.}}{{else}}unknown{{end}} creation time unavailable (cooldown: {{with (index $e.Data "cooldown")}}{{.}}{{else}}unknown{{end}}) - update check unavailable
{{- else if eq $msg "Starting HTTP API server" -}}
    Starting HTTP API server at {{if (index $e.Data "tls")}}https{{else}}http{{end}}://{{index $e.Data "host"}}:{{index $e.Data "port"}}
{{- else if eq $msg "HTTP API server is enabled" -}}
    HTTP API server is available at {{if (index $e.Data "tls")}}https{{else}}http{{end}}://{{index $e.Data "host"}}:{{index $e.Data "port"}}
{{- else if eq $msg "Only checking containers in scope" -}}
    Only checking containers in scope: {{index $e.Data "scope"}}
{{- else if eq $msg "Docker image usage exceeds configured maximum" -}}
    Docker image usage exceeds configured maximum: {{if HasKey $e.Data "usage"}}{{FormatDiskSpace (index $e.Data "usage")}}{{else}}unknown{{end}} of {{if HasKey $e.Data "max"}}{{FormatDiskSpace (index $e.Data "max")}}{{else}}unknown{{end}} used ({{if HasKey $e.Data "reclaimable"}}{{FormatDiskSpace (index $e.Data "reclaimable")}}{{else}}unknown{{end}} reclaimable, {{if HasKey $e.Data "image_count"}}{{index $e.Data "image_count"}}{{else}}unknown{{end}} images)
{{- else if eq $msg "Docker image usage exceeds configured warning threshold" -}}
    Docker image usage exceeds configured warning threshold: {{if HasKey $e.Data "usage"}}{{FormatDiskSpace (index $e.Data "usage")}}{{else}}unknown{{end}} of {{if HasKey $e.Data "warn"}}{{FormatDiskSpace (index $e.Data "warn")}}{{else}}unknown{{end}} used ({{if HasKey $e.Data "reclaimable"}}{{FormatDiskSpace (index $e.Data "reclaimable")}}{{else}}unknown{{end}} reclaimable, {{if HasKey $e.Data "image_count"}}{{index $e.Data "image_count"}}{{else}}unknown{{end}} images)
{{- else if eq $msg "Failed to query Docker image disk usage" -}}
    Failed to query Docker image disk usage{{with (index $e.Data "error")}}: {{.}}{{end}}
{{- else if eq $msg "Docker image usage budget enabled" -}}
    Docker image usage budget enabled: maximum {{if HasKey $e.Data "disk_space_max"}}{{FormatDiskSpace (index $e.Data "disk_space_max")}}{{else}}0 B{{end}}, warning at {{if HasKey $e.Data "disk_space_warn"}}{{FormatDiskSpace (index $e.Data "disk_space_warn")}}{{else}}0 B{{end}}
{{- else if eq $msg "Found new Git revision" -}}
    Found new Git revision: {{with (index $e.Data "revision")}}{{.}}{{else}}unknown{{end}} ({{with (index $e.Data "short_commit")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Built image from Git URL context" -}}
    Built image: {{with (index $e.Data "image")}}{{.}}{{else}}unknown{{end}} ({{with (index $e.Data "image_id")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Built Compose project" -}}
    Built Compose project: {{with (index $e.Data "project")}}{{.}}{{else}}unknown{{end}} ({{with (index $e.Data "service")}}{{.}}{{else}}unknown{{end}})
{{- else if eq $msg "Compose apply finished with unconfirmed service results" -}}
    Compose update finished with unconfirmed results for {{with (index $e.Data "service")}}{{.}}{{else}}unknown{{end}}
{{- else if eq $msg "Git build failed. Leaving running container untouched" -}}
    Git update failed for {{with (index $e.Data "container")}}{{.}}{{else}}unknown{{end}}. The running container was left in place{{with (index $e.Data "error")}}: {{.}}{{end}}
{{- else if eq $msg "Compose apply failed. Docker Compose may have partially recreated services" -}}
    Compose update failed for {{with (index $e.Data "container")}}{{.}}{{else}}unknown{{end}}. Docker Compose may have partially recreated services{{with (index $e.Data "error")}}: {{.}}{{end}}
{{- else if eq $msg "Compose update failed before container replacement" -}}
    Compose update failed for {{with (index $e.Data "container")}}{{.}}{{else}}unknown{{end}}. The running container was left in place{{with (index $e.Data "error")}}: {{.}}{{end}}
{{- else if eq $msg "Compose project directory is not readable. Leaving the running container untouched" -}}
    Skipped {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: compose directory is not readable
{{- else if eq $msg "Compose apply skipped. Container has no compose service label" -}}
    Skipped {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: container has no compose service label
{{- else if eq $msg "Compose apply result omitted this service. Runtime state is unknown" -}}
    Skipped {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: compose result did not include {{with (index $e.Data "service")}}{{.}}{{else}}the service{{end}}
{{- else if eq $msg "Compose apply did not return an identifiable instance for this replica. Runtime state is unknown" -}}
    Skipped {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: compose result did not identify this replica
{{- else if eq $msg "Could not dependency-sort Compose batches. Using the existing container order" -}}
    Compose dependency order is unavailable. Using the current container order
{{- else if eq $msg "Could not order Compose batches by dependencies. Using first-seen order" -}}
    Compose dependency order is unavailable. Using the current container order
{{- else if eq $msg "Compose batch dependencies are cyclic. Appending unresolved batches in first-seen order" -}}
    Compose dependency order is unavailable. Using the current container order
{{- else if eq $msg "Skipped container with an invalid git semver policy" -}}
    Skipped {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: invalid git semver policy
{{- else if eq $msg "Skipped container with an invalid git-host" -}}
    Skipped {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: invalid git-host
{{- else if eq $msg "Registry rate limit retries exhausted. Container failed for this cycle" -}}
    Update failed for {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}} ({{with (index $e.Data "image")}}{{.}}{{else}}unknown{{end}}){{with (index $e.Data "error")}}: {{.}}{{end}}
{{- else if eq $msg "Ignoring depends-on entry that names no container" -}}
    Ignored depends-on {{with (index $e.Data "depends_on")}}{{.}}{{else}}entry{{end}} for {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: no container has that name
{{- else if eq $msg "Skipping container update (self-dependency)" -}}
    Skipped {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: depends on itself
{{- else if eq $msg "Skipping container update (circular dependency)" -}}
    Skipped {{with (index $e.Data "container")}}{{.}}{{else}}container{{end}}: circular dependency
{{- else if $e.Data -}}
    {{- /* For messages with data, show message and key=value pairs */ -}}
    {{$msg}} | {{range $k, $v := $e.Data}}{{$k}}={{$v}} {{end}}
{{- else -}}
    {{- /* For messages without data, show just the message */ -}}
    {{$msg}}
{{- end -}}
{{- end -}}`,

	// "default" template provides a human-readable summary report of container update operations.
	// If .Report exists, displays counts of scanned/updated/failed containers, then lists details for each category.
	// Updated containers show name, image, and old/new image IDs.
	// Restarted containers show name, image, and state.
	// Fresh containers (no update needed) show name, image, and state.
	// Skipped containers show name, image, state, and error reason.
	// Failed containers show name, image, state, and error details.
	// If no .Report, falls back to listing all .Entries messages (one per line).
	// Expects .Report with Scanned, Updated, Restarted, Failed, Fresh, Skipped slices of containers.
	// Each container has Name, ImageName, State, Error, CurrentImageID, LatestImageID fields.
	`default`: `
{{- if .Report -}}
  {{- /* Use report summary data */ -}}
  {{- with .Report -}}
    {{len .Scanned}} Scanned, {{len .Updated}} Updated, {{len .Restarted}} Restarted, {{len .Failed}} Failed, {{len .Fresh}} Fresh, {{len .Skipped}} Skipped
      {{- /* List successfully updated containers */ -}}
      {{- range .Updated}}
- {{.Name}} ({{.ImageName}}): {{.CurrentImageID.ShortID}} updated to {{.LatestImageID.ShortID}}{{with .GitRef}} ref {{.}}{{end}}{{with .Changelog}} {{.}}{{end}}
      {{- end -}}
      {{- /* List restarted containers */ -}}
      {{- range .Restarted}}
- {{.Name}} ({{.ImageName}}): {{.State}}
      {{- end -}}
      {{- /* List fresh containers (no update needed) */ -}}
      {{- range .Fresh}}
- {{.Name}} ({{.ImageName}}): {{.State}}
	  {{- end -}}
	  {{- /* List skipped containers with reason */ -}}
	  {{- range .Skipped}}
- {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
	  {{- end -}}
	  {{- /* List failed containers with error */ -}}
	  {{- range .Failed}}
- {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
	  {{- end -}}
  {{- end -}}
{{- else -}}
  {{- /* Fallback to simple entry messages */ -}}
  {{range .Entries -}}{{.Message}}{{"\n"}}{{- end -}}
{{- end -}}`,

	// "porcelain.v1.summary-no-log" template provides machine-readable output for scripting/automation.
	// Iterates over all containers in .Report.All, showing name, image, state, and error (if any) per line.
	// If no containers matched the filter, outputs "no containers matched filter".
	// Expects .Report.All []Container slice with Name, ImageName, State, Error fields.
	// States include: Updated, Restarted, Fresh, Skipped, Failed.
	`porcelain.v1.summary-no-log`: `
{{- if .Report -}}
  {{- /* Iterate over all containers */ -}}
  {{- range .Report.All }}
    {{- .Name}} ({{.ImageName}}): {{.State -}}
    {{- with .Error}} Error: {{.}}{{end}}{{ println }}
  {{- else -}}
    no containers matched filter
  {{- end -}}
{{- else -}}
  no containers matched filter
{{- end -}}`,

	// "json.v1" template outputs the entire data structure as JSON for programmatic consumption.
	// Useful for integrations that need structured data.
	// Expects any data structure that can be JSON marshaled (typically the full report or entries).
	`json.v1`: `{{ . | ToJSON }}`,

	// "porcelain.json" template outputs the update report as a stable JSON document with per-container details.
	// It omits log entries and focuses on the report: name, image, image_id, latest_image_id, state, update_available, and error.
	// Expects .Report with an All() method returning []ContainerReport.
	`porcelain.json`: `{{ .Report | ToPorcelainJSON }}`,
}
