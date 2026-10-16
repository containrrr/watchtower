package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/distribution/reference"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
)

// BenchmarkNewAuthClient benchmarks the NewAuthClient function which creates
// a new HTTP client for each auth request. This measures the overhead of
// client creation including TLS configuration and transport setup.
func BenchmarkNewAuthClient(b *testing.B) {
	for b.Loop() {
		_ = NewAuthClient(nil, registryConfig.Registry{})
	}
}

// BenchmarkNewAuthClientWithAlloc measures memory allocations during client creation.
func BenchmarkNewAuthClientWithAlloc(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = NewAuthClient(nil, registryConfig.Registry{})
	}
}

// BenchmarkNewAuthClientParallel benchmarks concurrent client creation
// to simulate multiple containers requiring auth simultaneously.
func BenchmarkNewAuthClientParallel(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = NewAuthClient(nil, registryConfig.Registry{})
		}
	})
}

// BenchmarkTransformAuth benchmarks the TransformAuth function which decodes
// and re-encodes authentication credentials. This measures the overhead of
// base64 decoding/encoding and JSON unmarshaling.
func BenchmarkTransformAuth(b *testing.B) {
	// Pre-encoded registry auth with credentials
	authWithCredentials := "eyJ1c2VybmFtZSI6InRlc3QiLCJwYXNzd29yZCI6InRlc3RwYXNzIn0=" // {"username":"test","password":"testpass"}

	for b.Loop() {
		_ = TransformAuth(testLog(), authWithCredentials)
	}
}

// BenchmarkTransformAuthNoCredentials benchmarks TransformAuth with empty credentials.
// This represents the common case where no credentials are configured.
func BenchmarkTransformAuthNoCredentials(b *testing.B) {
	emptyAuth := ""

	for b.Loop() {
		_ = TransformAuth(testLog(), emptyAuth)
	}
}

// BenchmarkTransformAuthInvalid benchmarks TransformAuth with invalid base64 input.
// This measures error handling overhead for malformed inputs.
func BenchmarkTransformAuthInvalid(b *testing.B) {
	invalidAuth := "not-valid-base64!!!"

	for b.Loop() {
		_ = TransformAuth(testLog(), invalidAuth)
	}
}

// BenchmarkGetRegistryAddress benchmarks GetRegistryAddress which extracts
// the registry domain from an image reference.
func BenchmarkGetRegistryAddress(b *testing.B) {
	testImages := []string{
		"ghcr.io/example/app:latest",
		"docker.io/library/nginx:latest",
		"quay.io/redhat/ubi8:latest",
		"gcr.io/project/image:v1.0",
		"registry.example.com/namespace/image:tag",
	}

	for b.Loop() {
		for _, img := range testImages {
			_, _ = GetRegistryAddress(testLog(), img)
		}
	}
}

// BenchmarkGetChallengeURL benchmarks GetChallengeURL which constructs
// the challenge URL for registry authentication.
func BenchmarkGetChallengeURL(b *testing.B) {
	refs := make([]reference.Named, 0, 2)

	for _, img := range []string{
		"ghcr.io/example/app:latest",
		"docker.io/library/nginx:latest",
	} {
		ref, _ := reference.ParseNormalizedNamed(img)
		refs = append(refs, ref)
	}

	for b.Loop() {
		for _, ref := range refs {
			_ = GetChallengeURL(testLog(), ref, registryConfig.Registry{}, "")
		}
	}
}

// BenchmarkGetAuthURL benchmarks GetAuthURL which constructs the authentication URL.
func BenchmarkGetAuthURL(b *testing.B) {
	challenge := "bearer realm=\"https://ghcr.io/token\",service=\"ghcr.io\",scope=\"repository:user/repo:pull\""
	ref, _ := reference.ParseNormalizedNamed("ghcr.io/user/repo:latest")

	for b.Loop() {
		_, _ = GetAuthURL(testLog(), challenge, ref, "", registryConfig.Registry{})
	}
}

// BenchmarkStringOperations benchmarks common string operations used in auth processing.
func BenchmarkStringOperations(b *testing.B) {
	testInputs := []string{
		"Bearer realm=\"https://ghcr.io/token\"",
		"https://ghcr.io/token",
		"ghcr.io/example/app:latest",
	}

	for b.Loop() {
		for _, input := range testInputs {
			_ = strings.ToLower(input)
			_ = strings.TrimSpace(input)
			_ = strings.TrimPrefix(input, "bearer ")
			_ = strings.Split(input, ",")
			// Simulate CutPrefix
			_, _ = strings.CutPrefix(input, "https://")
			// Simulate Cut
			_, _, _ = strings.Cut(input, "/")
		}
	}
}

// BenchmarkHTTPClientCreation benchmarks the raw HTTP client creation
// without the registry-specific wrapper.
func BenchmarkHTTPClientCreation(b *testing.B) {
	for b.Loop() {
		_ = &http.Client{}
	}
}

// BenchmarkHTTPTransportCreation benchmarks HTTP transport creation with custom settings.
func BenchmarkHTTPTransportCreation(b *testing.B) {
	for b.Loop() {
		_ = &http.Transport{}
	}
}

// BenchmarkBase64Encoding benchmarks base64 encoding operations used in auth transformation.
func BenchmarkBase64Encoding(b *testing.B) {
	input := "testuser:testpassword"

	for b.Loop() {
		_ = base64.StdEncoding.EncodeToString([]byte(input))
	}
}

// BenchmarkTokenFetchSimulation simulates the token fetch pattern.
// This measures the CPU overhead of the auth flow without actual network calls.
func BenchmarkTokenFetchSimulation(b *testing.B) {
	// Simulate the token fetch pattern: create client, process challenge, get auth URL
	challenge := `Bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/repo:pull"`

	for b.Loop() {
		// Simulate NewAuthClient
		_ = &http.Client{}

		// Simulate string operations for URL construction
		_ = strings.ToLower(challenge)
		_ = strings.TrimPrefix(challenge, "bearer ")
	}
}

// BenchmarkConcurrentAuthRequests benchmarks concurrent auth request setup
// to simulate multiple containers being checked simultaneously.
func BenchmarkConcurrentAuthRequests(b *testing.B) {
	numGoroutines := 10

	testImages := make([]string, numGoroutines)
	for i := range numGoroutines {
		testImages[i] = "ghcr.io/example/app-" + string(rune('a'+i)) + ":latest"
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := range numGoroutines {
				// Simulate auth client creation per container
				_ = NewAuthClient(nil, registryConfig.Registry{})

				// Simulate registry address lookup
				_, _ = GetRegistryAddress(testLog(), testImages[i])
			}
		}
	})
}

// BenchmarkAuthWithMultipleRegistries benchmarks auth setup for multiple registries.
// This simulates a watchtower instance monitoring containers from different registries.
func BenchmarkAuthWithMultipleRegistries(b *testing.B) {
	registries := []struct {
		name  string
		image string
	}{
		{"ghcr", "ghcr.io/example/app:latest"},
		{"dockerhub", "docker.io/library/nginx:latest"},
		{"quay", "quay.io/redhat/ubi8:latest"},
		{"gcr", "gcr.io/project/image:v1.0"},
		{"ecr", "123456789012.dkr.ecr.us-east-1.amazonaws.com/image:tag"},
	}

	for b.Loop() {
		for _, reg := range registries {
			// Create client for each registry
			_ = NewAuthClient(nil, registryConfig.Registry{})

			// Get registry address
			_, _ = GetRegistryAddress(testLog(), reg.image)

			// Process challenge (simulated)
			_ = strings.ToLower(reg.image)
		}
	}
}
