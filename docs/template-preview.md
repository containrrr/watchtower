
<style>
    body {font-size: .75rem;}

    textarea {
        box-decoration-break: slice;
        overflow: auto;
        padding: 0.77em 1.18em;
        scrollbar-color: var(--md-default-fg-color--lighter) transparent;
        scrollbar-width: thin;
        touch-action: auto;
        word-break: normal;
        height: 420px;
    }

    textarea, input {
        background-color: var(--md-code-bg-color);
        border-width: 0;
        border-radius: 0.1rem;
        color: var(--md-code-fg-color);
        font-feature-settings: "kern";
        font-family: var(--md-code-font-family);
    }

    .numfield {
        font-size: .7rem;
        display: flex;
        flex-direction: column;
        justify-content: space-between;
    }
    label:not(:last-child) {
        /* margin-bottom: 0.5rem; */
    }

    button {
        border-radius: 0.1rem;
        color: var(--md-typeset-color);
        background-color: var(--md-primary-fg-color);
        
    }
    button:hover {
            background-color: var(--md-accent-fg-color);
        }

    input[type="number"] { width: 5ch; flex: 1; font-size: 1rem; }

    fieldset {
        margin-top: -0.5rem;
        display: flex;
        flex: 1;
        column-gap: 0.5rem;
    }

    #result {
        font-size: 0.7rem;
        background-color: var(--md-code-bg-color);
        scrollbar-color: var(--md-default-fg-color--lighter) transparent;
        scrollbar-width: thin;
        touch-action: auto;
        overflow: auto;
        padding: 0.77em 1.18em;
        margin:0;
        height: 540px;
    }

</style>
<script src="../assets/wasm_exec.js"></script>
<script>
    const updatePreview = () => {
        const form = document.querySelector('#tplprev');
        const input = form.template.value;
        console.log('Input: %o', input);
        const arrFromCount = (key) => Array.from(Array(form[key]?.valueAsNumber ?? 0), () => key);
        const states = form.enablereport.checked ? [
            ...arrFromCount("skipped"),
            ...arrFromCount("scanned"),
            ...arrFromCount("updated"),
            ...arrFromCount("failed" ),
            ...arrFromCount("fresh"  ),
            ...arrFromCount("stale"  ),
        ] : [];
        console.log("States: %o", states);
        const levels = form.enablelog.checked ? [
            ...arrFromCount("error"),
            ...arrFromCount("warning"),
            ...arrFromCount("info"),
            ...arrFromCount("debug"),
        ] : [];
        console.log("Levels: %o", levels);
        const output = WATCHTOWER.tplprev(input, states, levels);
        console.log('Output: \n%o', output);
        document.querySelector('#result').innerText = output;
    }
    const formSubmitted = (e) => {
        console.log("Event: %o", e);
        e.preventDefault();
        updatePreview();
    }
    let debounce;
    const inputUpdated = () => {
        if(debounce) clearTimeout(debounce);
        debounce = setTimeout(() => updatePreview(), 400);
    }
    const formChanged = (e) =>  {
        console.log('form changed: %o', e);
    }
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("../assets/tplprev.wasm"), go.importObject).then((result) => {
        document.querySelector('#loading').style.display = "none";
        go.run(result.instance);
        updatePreview();
    });
</script>




<form id="tplprev" onchange="inputUpdated()" onsubmit="formSubmitted(event)" style="margin: 0;display: flex; flex-direction: column; row-gap: 1rem; box-sizing: border-box; position: relative; margin-right: -13.3rem">
<pre id="loading" style="position: absolute; inset: 0; display: flex; padding: 1rem; box-sizing: border-box; background: var(--md-code-bg-color); margin-top: 0">loading wasm...</pre>


<div style="display: flex; flex:1; column-gap: 1rem;">
<textarea name="template" type="text" style="flex: 1" onkeyup="inputUpdated()">{{- with .Report -}}
  {{- if ( or .Updated .Failed ) -}}
{{len .Scanned}} Scanned, {{len .Updated}} Updated, {{len .Failed}} Failed
    {{- range .Updated}}
- {{.Name}} ({{.ImageName}}): {{.CurrentImageID.ShortID}} updated to {{.LatestImageID.ShortID}}
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
{{- if (and .Entries .Report) }}

Logs:
{{ end -}}
{{range .Entries -}}{{.Time.Format "2006-01-02T15:04:05Z07:00"}} [{{.Level}}] {{.Message}}{{"\n"}}{{- end -}}</textarea>
</div>
<div style="display: flex; flex-direction: row; column-gap: 0.5rem">
<fieldset>
    <legend><label><input type="checkbox" name="enablereport" checked /> Container report</label></legend>
    <label class="numfield">
        Skipped:
        <input type="number" name="skipped" value="3" />
    </label>
    <label class="numfield">
        Scanned:
        <input type="number" name="scanned" value="3" />
    </label>
    <label class="numfield">
        Updated:
        <input type="number" name="updated" value="3" />
    </label>
    <label class="numfield">
        Failed:
        <input type="number" name="failed" value="3" />
    </label>
    <label class="numfield">
        Fresh:
        <input type="number" name="fresh" value="3" />
    </label>
    <label class="numfield">
        Stale:
        <input type="number" name="stale" value="3" />
    </label>
</fieldset>
<fieldset>
    <legend><label><input type="checkbox" name="enablelog" checked /> Log entries</label></legend>
    <label class="numfield">
        Error: 
        <input type="number" name="error" value="1" />
    </label>
    <label class="numfield">
        Warning:
        <input type="number" name="warning" value="2" />
    </label>
    <label class="numfield">
        Info:
        <input type="number" name="info" value="3" />
    </label>
    <label class="numfield">
        Debug:
        <input type="number" name="debug" value="4" />
    </label>
</fieldset>
<button type="submit" style="flex:1; min-width: 12ch; padding: 0.5rem">Update preview</button>
</div>
<div style="flex: 1; display: flex">
    <pre style="flex:1; width:100%" id="result"></pre>
</div>