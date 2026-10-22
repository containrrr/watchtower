// Immediately Invoked Function Expression (IIFE) to encapsulate the entire script, preventing global scope pollution
// This ensures all variables and functions are scoped locally, avoiding conflicts with other scripts
(function () {
  // Global Variables Section
  // Define variables accessible throughout the script's lifecycle
  let wasmLoaded = false; // Boolean flag to track if the WebAssembly module has been successfully loaded; initialized as false
  // This flag prevents preview updates until the WASM module is ready
  let currentTab = "text"; // String to track the currently selected tab ("text" or "json"), defaulting to "text"
  // This variable determines which tab's content is displayed in the preview area
  let debounce; // Variable to store the debounce timeout ID, used to throttle frequent updates
  // Debouncing prevents excessive calls to updatePreview during rapid input changes

  // Helper Functions Section
  // Collection of utility functions to simplify DOM queries and improve code reusability
  const getForm = () => document.querySelector("#tplprev"); // Function to retrieve the main form element by ID; returns null if not found
  // This function provides a reusable way to access the form, handling potential DOM absence
  const getResult = () => document.querySelector("#result"); // Function to retrieve the result div element by ID; returns null if not found
  // Used to target the preview output container, essential for rendering
  const getTextOutput = () => getResult()?.querySelector(".text-output"); // Function to retrieve the text preview output pre element; uses optional chaining due to potential null result
  // Safely accesses the text output element, avoiding errors if the result div is missing
  const getJsonOutput = () => getResult()?.querySelector(".json-output"); // Function to retrieve the JSON preview output pre element; uses optional chaining
  // Similar to getTextOutput, but for JSON output, ensuring robustness
  const getTabText = () => document.querySelector("#tab-text"); // Function to retrieve the text tab radio input element
  // Provides access to the text tab radio for event handling
  const getTabJson = () => document.querySelector("#tab-json"); // Function to retrieve the JSON tab radio input element
  // Provides access to the JSON tab radio for event handling

  // Format Output Function
  // Function to process and format the output for display in the preview area
  // Takes a string output and returns an object with formatting instructions
  const formatOutput = (output) => {
    // Check if the output starts with an error message
    if (output.startsWith("Error: ")) {
      // If true, return an object with HTML content highlighting the error
      // The substring(7) removes the "Error: " prefix for clean display
      return { isHtml: true, content: `<b>Error</b>: ${output.substring(7)}` };
    } else if (output.length === 0) {
      // If the output is empty, return an object with an italicized placeholder message
      // Indicates that no notification would be sent, enhancing user feedback
      return {
        isHtml: true,
        content: "<i>empty (would not be sent as a notification)</i>",
      };
    } else {
      // Otherwise, return the output as plain text
      // Preserves the original content without HTML formatting
      return { isHtml: false, content: output };
    }
  };

  // Main Preview Update Function
  // Core function to update the preview based on form input and WASM processing
  // Triggered by input changes, tab switches, or initial load
  const updatePreview = () => {
    // Check if WebAssembly is loaded; exit early if not to prevent errors
    // This prevents attempts to call WASM functions before initialization
    if (!wasmLoaded) return;

    // Retrieve the form element and validate its existence
    // The form is the root element for all input data
    const form = getForm();
    if (!form) {
      // Log an error if the form is not found in the DOM
      // This helps diagnose DOM loading issues
      console.error("Form #tplprev not found");
      return;
    }

    // Extract the template text from the textarea
    // This is the user-provided Go template for rendering
    const input = form.template.value;

    // Helper function to generate arrays based on numeric input values
    // Takes a key (e.g., "skipped") and returns an array of that length
    const arrFromCount = (key) =>
      Array.from(Array(form[key]?.valueAsNumber ?? 0), () => key);

    // Generate state arrays based on report toggle
    // Populates arrays for WASM processing based on form inputs
    const states =
      form.report.value === "yes"
        ? [
            ...arrFromCount("skipped"), // Array of "skipped" states based on input
            ...arrFromCount("scanned"), // Array of "scanned" states
            ...arrFromCount("updated"), // Array of "updated" states
            ...arrFromCount("failed"), // Array of "failed" states
            ...arrFromCount("restarted"), // Array of "restarted" states based on input
            ...arrFromCount("fresh"), // Array of "fresh" states
            ...arrFromCount("stale"), // Array of "stale" states
          ]
        : []; // Empty array if report is disabled

    // Generate level arrays based on log toggle
    // Similar to states, but for log entry levels
    const levels =
      form.log.value === "yes"
        ? [
            ...arrFromCount("error"), // Array of "error" log levels
            ...arrFromCount("warning"), // Array of "warning" log levels
            ...arrFromCount("info"), // Array of "info" log levels
            ...arrFromCount("debug"), // Array of "debug" log levels
          ]
        : []; // Empty array if log is disabled

    // Render text output using the WASM function with the user-provided template
    // Try-catch block to handle potential WASM errors
    let textOutput;
    try {
      textOutput = WATCHTOWER.tplprev(input, states, levels);
    } catch (err) {
      // Handle any errors during text output rendering
      // Logs the error and sets a fallback message
      console.error("Text output WASM error:", err);
      textOutput = `Error: ${err.message}`;
    }

    // Render JSON output using a fixed JSON template
    // Separate try-catch for JSON rendering
    let jsonOutput;
    const jsonTemplate = "{{ . | ToJSON }}"; // Fixed template for JSON output
    try {
      jsonOutput = WATCHTOWER.tplprev(jsonTemplate, states, levels);
    } catch (err) {
      // Handle any errors during JSON output rendering
      console.error("JSON output WASM error:", err);
      jsonOutput = `Error: ${err.message}`;
    }

    // Format the outputs for display
    // Apply formatting rules to both text and JSON outputs
    const textFormatted = formatOutput(textOutput);
    const jsonFormatted = formatOutput(jsonOutput);

    // Retrieve output elements
    // Use helper functions to safely access DOM elements
    const textPre = getTextOutput(); // Get text output pre element
    const jsonPre = getJsonOutput(); // Get JSON output pre element

    // Update text preview if element exists
    // Use innerHTML for formatted content, innerText for plain text
    if (textPre) {
      textPre[textFormatted.isHtml ? "innerHTML" : "innerText"] =
        textFormatted.content;
    }

    // Update JSON preview if element exists
    if (jsonPre) {
      jsonPre[jsonFormatted.isHtml ? "innerHTML" : "innerText"] =
        jsonFormatted.content;
    }

    // Force tab visibility based on current tab selection
    // Ensure the correct tab is displayed
    const textTab = getTabText(); // Get text tab radio
    const jsonTab = getTabJson(); // Get JSON tab radio
    if (currentTab === "json" && jsonTab) {
      jsonTab.checked = true; // Select JSON tab
    } else if (textTab) {
      textTab.checked = true; // Select text tab
    }
  };

  // Input Change Handler
  // Function to handle changes in form inputs, updating the hidden toggle fields
  // Triggered by input events on form fields
  const handleInputChange = (e) => {
    const form = e.target.form || e.target.closest("form"); // Get the form from the event target
    // Fallback to closest form if target is not directly within it
    const targetToggle = e.target.dataset["toggle"]; // Get the data-toggle attribute
    // Retrieves the attribute used to map checkboxes to hidden inputs
    if (targetToggle) {
      form[targetToggle].value = e.target.checked ? "yes" : "no"; // Update hidden input
      // Sets the hidden input value based on checkbox state
    }
    if (debounce) clearTimeout(debounce); // Clear any existing debounce timeout
    // Prevents multiple simultaneous updates by clearing previous timeout
    debounce = setTimeout(() => requestAnimationFrame(updatePreview), 400); // Debounce update with 400ms delay
    // Uses requestAnimationFrame for smooth, performance-optimized updates
  };

  // Tab Change Handler
  // Function to handle tab selection changes
  // Triggered by radio input changes
  const handleTabChange = (e) => {
    currentTab = e.target.id === "tab-text" ? "text" : "json"; // Update current tab based on radio ID
    // Sets the currentTab based on which radio button is selected
    requestAnimationFrame(updatePreview); // Request animation frame for smooth update
    // Ensures the UI update is synchronized with the browser's rendering cycle
  };

  // Load Query Values
  // Function to initialize form fields from URL query parameters
  // Runs on page load to populate form with URL data
  const loadQueryVals = () => {
    const form = getForm(); // Get form element
    if (!form) return; // Exit if form not found
    const params = new URLSearchParams(location.search); // Parse URL query parameters
    for (const [key, value] of params) {
      form[key].value = value; // Set form field value
      // Populates input fields with query parameter values
      const toggleInput = form.querySelector(`[data-toggle="${key}"]`); // Find toggle checkbox
      if (toggleInput) {
        toggleInput.checked = value === "yes"; // Set checkbox state
        // Updates checkbox state if the value indicates "yes"
      }
    }
  };

  // Setup Event Listeners
  // Function to initialize event listeners and tab structure
  // Executes on DOM content loaded or page reload
  const setupEventListeners = () => {
    loadQueryVals(); // Load initial values from query params
    const form = getForm(); // Get form element
    if (!form) return; // Exit if form not found

    form.removeEventListener("input", handleInputChange); // Remove existing listener to prevent duplicates
    form.addEventListener("input", handleInputChange); // Add input change listener
    // Ensures only one input listener is active at a time

    const resultElement = getResult(); // Get result div
    if (!resultElement) return; // Exit if result not found
    if (!resultElement.querySelector(".tabs")) {
      // Initialize tab structure if not present
      resultElement.innerHTML = `
        <div class="tabs">
          <div class="tab-header">
            <label for="tab-text">Text</label>
            <label for="tab-json">JSON</label>
          </div>
          <input type="radio" name="result-tabs" id="tab-text" ${
            currentTab === "text" ? 'checked="checked"' : ""
          }>
          <div class="tab text-tab">
            <pre class="preview-output text-output"></pre>
          </div>
          <input type="radio" name="result-tabs" id="tab-json" ${
            currentTab === "json" ? 'checked="checked"' : ""
          }>
          <div class="tab json-tab">
            <pre class="preview-output json-output"></pre>
          </div>
        </div>
      `;
      // Dynamically creates the tabbed interface with radio inputs and preview areas
      const tabText = getTabText();
      const tabJson = getTabJson();
      if (tabText) {
        tabText.removeEventListener("change", handleTabChange); // Remove existing listener
        tabText.addEventListener("change", handleTabChange); // Add new listener
      }
      if (tabJson) {
        tabJson.removeEventListener("change", handleTabChange); // Remove existing listener
        tabJson.addEventListener("change", handleTabChange); // Add new listener
      }
    }

    if (currentTab === "json") {
      const jsonTab = getTabJson(); // Get JSON tab
      if (jsonTab) jsonTab.checked = true; // Set initial tab to JSON if selected
    }

    setTimeout(() => requestAnimationFrame(updatePreview), 500); // Delay initial update
    // Uses a 500ms delay to ensure DOM is fully rendered before first update
  };

  // WASM Loading Section
  // Function to load and initialize the WebAssembly module
  // Executes when the script loads to set up the WASM environment
  const go = new Go(); // Create Go runtime instance
  WebAssembly.instantiateStreaming(
    fetch("../../assets/tplprev.wasm"),
    go.importObject
  )
    .then((result) => {
      go.run(result.instance); // Run the WebAssembly module
      const loadingElement = document.querySelector("#tplprev .loading"); // Get loading indicator
      if (loadingElement) loadingElement.style.display = "none"; // Hide loading when done
      wasmLoaded = true; // Mark WASM as loaded
      window.wasmLoaded = wasmLoaded; // Expose to global scope
      setTimeout(() => requestAnimationFrame(updatePreview), 500); // Initial update after load
    })
    .catch((err) => {
      console.error("WASM loading failed:", err); // Log any loading errors
      const resultElement = getResult(); // Get result div
      if (resultElement) {
        resultElement.innerHTML =
          '<div class="error">Failed to load WebAssembly module. Please refresh the page.</div>';
      } else {
        console.error("#result element not found during WASM error handling");
      }
    });

  // Guard Clause Section
  // Check if WASM is already loaded to prevent duplicate initialization
  // Runs if the page is reloaded or WASM is pre-loaded
  if (typeof window.wasmLoaded !== "undefined") {
    setTimeout(() => setupEventListeners(), 500); // Re-run setup if already loaded
    return;
  }

  window.wasmLoaded = wasmLoaded; // Store WASM status in global scope

  // DOM Content Loaded Handler
  // Initialize setup when DOM is ready
  // Ensures the script waits for the DOM to be fully parsed
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () =>
      setTimeout(setupEventListeners, 500)
    );
  } else {
    setTimeout(() => setupEventListeners(), 500);
  }

  // Input Validation Section
  // Function to validate and correct negative number inputs
  const validateNumberInputs = () => {
    const form = getForm(); // Get form element
    if (!form) return; // Exit if form not found
    const numberInputs = form.querySelectorAll('input[type="number"]'); // Select all number inputs
    numberInputs.forEach((input) => {
      const value = parseInt(input.value, 10); // Parse input value as integer
      if (value < 0) {
        input.value = "0"; // Reset to 0 if negative
        // Update the corresponding hidden input if it exists
        const toggleKey = input.name;
        const hiddenInput = form.querySelector(`input[name="${toggleKey}"]`);
        if (hiddenInput) {
          hiddenInput.value = input.value; // Sync hidden input
        }
      }
    });
  };

  // Attach input validation on change
  const form = getForm(); // Get form element
  if (form) {
    form.addEventListener("change", validateNumberInputs); // Validate on change event
  }
})();
