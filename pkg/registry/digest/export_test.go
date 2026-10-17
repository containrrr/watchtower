package digest

// BuildManifestURL exposes buildManifestURL to the external test package, so
// mirror and scheme handling can be checked without a request.
var BuildManifestURL = buildManifestURL

// DigestsMatch exposes digestsMatch to the external test package, so digest
// comparison can be checked without a registry.
var DigestsMatch = digestsMatch

// ExtractGetDigest exposes extractGetDigest to the external test package, so
// GET responses can be checked without a registry.
var ExtractGetDigest = extractGetDigest

// ExtractHeadDigest exposes extractHeadDigest to the external test package, so
// HEAD responses can be checked without a registry.
var ExtractHeadDigest = extractHeadDigest

// FetchDigest exposes fetchDigestWithGet to the external test package, so the
// GET digest request can be checked against a test registry.
var FetchDigest = fetchDigestWithGet

// HandleManifestResponse exposes handleManifestResponse to the external test
// package, so redirect and retry handling can be checked without a request.
var HandleManifestResponse = handleManifestResponse
