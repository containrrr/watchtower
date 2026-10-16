package notifications

// AppendLegacyURLs exposes appendLegacyURLs to the external test package, so
// legacy URL and delay conversion can be checked without a notifier.
var AppendLegacyURLs = appendLegacyURLs

// TemplateData exposes templateData to the external test package, so the
// notification title can be checked without a notifier.
var TemplateData = templateData
