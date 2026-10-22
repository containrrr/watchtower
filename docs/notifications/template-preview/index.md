---
hide:
  - toc
---
<!-- markdownlint-disable -->

<!-- Page title for the template preview tool -->
# Template Preview

<!-- Link to the custom CSS stylesheet for styling the form and preview -->
<link rel="stylesheet" href="styles.css">

<!-- Script for WebAssembly runtime support -->
<script src="../../assets/wasm_exec.js"></script>

<!-- Script containing JavaScript logic for form interactions, WASM loading, and preview updates -->
<script src="script.js"></script>

<!-- Form element for user input and preview generation, with ID for JS targeting and role for accessibility -->
<form id="tplprev" role="form">

<!-- Loading indicator shown while WebAssembly module is loading -->
<pre class="loading">loading wasm...</pre>

<!-- Wrapper div for the template textarea and its header, used for styling and layout -->
<div class="template-wrapper">

  <!-- Container div for the template input header bar, ensuring isolated styling -->
  <div class="template-header-container">

    <!-- Header bar for the template input textarea, styled to match tab-header -->
    <div class="template-header">Template Input</div>

  <!-- Closing div for template-header-container -->
  </div>

  <!-- Textarea for user to edit the notification template, with default Go template content for report and logs -->
  <textarea name="template" rows="20">{{- if .Report -}}
 {{- with .Report -}}
   {{len .Scanned}} Scanned, {{len .Updated}} Updated, {{len .Failed}} Failed, {{len .Restarted}} Restarted
   {{- if ( or .Updated .Failed .Restarted ) -}}
     {{- range .Updated}}
- {{.Name}} ({{.ImageName}}): {{.CurrentImageID.ShortID}} updated to {{.LatestImageID.ShortID}}
     {{- end -}}
     {{- range .Restarted}}
- {{.Name}} ({{.ImageName}}): {{.State}}
     {{- end -}}
     {{- range .Fresh}}
- {{.Name}} ({{.ImageName}}): {{.State}}
     {{- end -}}
     {{- range .Skipped}}
- {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
     {{- end -}}
     {{- range .Failed}}
- {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
     {{- end -}}
   {{- end -}}
 {{- end -}}
{{- else -}}
 {{range .Entries -}}{{.Message}}{{"\n"}}{{- end -}}
{{- end -}}</textarea>

<!-- Closing div for template-wrapper -->
</div>

<!-- Wrapper div for control fieldsets, used for styling and layout -->
<div class="controls">

  <!-- Fieldset for container report controls, with ID for potential JS targeting -->
  <fieldset id="report-fieldset">

    <!-- Hidden input to toggle report generation, default to "yes" -->
    <input type="hidden" name="report" value="yes" />

    <!-- Legend with checkbox to toggle report visibility, checked by default, with data-toggle attribute for JS handling -->
    <legend><label><input type="checkbox" data-toggle="report" checked /> Container report</label></legend>

    <!-- Label and number input for skipped containers count, default value 3, minimum value 0 -->
    <label class="numfield">
        Skipped:
        <input type="number" name="skipped" value="3" min="0" />
    </label>

    <!-- Label and number input for scanned containers count, default value 3, minimum value 0 -->
    <label class="numfield">
        Scanned:
        <input type="number" name="scanned" value="3" min="0" />
    </label>

    <!-- Label and number input for updated containers count, default value 3, minimum value 0 -->
    <label class="numfield">
        Updated:
        <input type="number" name="updated" value="3" min="0" />
    </label>

    <!-- Label and number input for failed containers count, default value 3, minimum value 0 -->
    <label class="numfield">
        Failed:
        <input type="number" name="failed" value="3" min="0" />
    </label>

    <!-- Label and number input for restarted containers count, default value 3, minimum value 0 -->
    <label class="numfield">
        Restarted:
        <input type="number" name="restarted" value="3" min="0" />
    </label>

    <!-- Label and number input for fresh containers count, default value 3, minimum value 0 -->
    <label class="numfield">
        Fresh:
        <input type="number" name="fresh" value="3" min="0" />
    </label>

    <!-- Label and number input for stale containers count, default value 3, minimum value 0 -->
    <label class="numfield">
        Stale:
        <input type="number" name="stale" value="3" min="0" />
    </label>

  <!-- Closing fieldset for report controls -->
  </fieldset>

  <!-- Fieldset for log entries controls, with ID for potential JS targeting -->
  <fieldset id="log-fieldset">

    <!-- Hidden input to toggle log generation, default to "yes" -->
    <input type="hidden" name="log" value="yes" />

    <!-- Legend with checkbox to toggle log visibility, checked by default, with data-toggle attribute for JS handling -->
    <legend><label><input type="checkbox" data-toggle="log" checked /> Log entries</label></legend>

    <!-- Label and number input for error log count, default value 1, minimum value 0 -->
    <label class="numfield">
        Error:
        <input type="number" name="error" value="1" min="0" />
    </label>

    <!-- Label and number input for warning log count, default value 2, minimum value 0 -->
    <label class="numfield">
        Warning:
        <input type="number" name="warning" value="2" min="0" />
    </label>

    <!-- Label and number input for info log count, default value 3, minimum value 0 -->
    <label class="numfield">
        Info:
        <input type="number" name="info" value="3" min="0" />
    </label>

    <!-- Label and number input for debug log count, default value 4, minimum value 0 -->
    <label class="numfield">
        Debug:
        <input type="number" name="debug" value="4" min="0" />
    </label>

  <!-- Closing fieldset for log controls -->
  </fieldset>

<!-- Closing div for controls -->
</div>

<!-- Wrapper div for the preview result area, used for styling and layout -->
<div class="result-wrapper">

  <!-- Div for rendering the template preview output, with ID for JS targeting -->
  <div id="result"></div>

<!-- Closing div for result-wrapper -->
</div>

<!-- Closing form element -->
</form>
