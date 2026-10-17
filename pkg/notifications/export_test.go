package notifications

// AppendLegacyURLs exposes appendLegacyURLs to the external test package, so
// legacy URL and delay conversion can be checked without a notifier.
var AppendLegacyURLs = appendLegacyURLs

// TemplateData exposes templateData to the external test package, so the
// notification title can be checked without a notifier.
var TemplateData = templateData

// ColorHex exposes colorHex to the external test package, so the legacy
// notifier colors can be checked.
const ColorHex = colorHex

// ColorInt exposes colorInt to the external test package, so the legacy
// notifier colors can be checked.
const ColorInt = colorInt

// GetDelay exposes getDelay to the external test package, so the legacy
// notification delay can be checked without a notifier.
var GetDelay = getDelay

// GetTitle exposes getTitle to the external test package, so the notification
// title can be checked without a notifier.
var GetTitle = getTitle
