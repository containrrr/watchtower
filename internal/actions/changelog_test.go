package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	internalGit "github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/internal/release"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// newDigest is the manifest digest of the new image in these tests.
const newDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

// panickingFetcher fails the test if it is ever called.
//
// The default configuration must never reach the registry, so the assertion is
// stronger than counting calls: any invocation fails outright.
type panickingFetcher struct {
	t *testing.T
}

func (f panickingFetcher) FetchDigestForTag(
	_ *zerolog.Logger,
	_ context.Context,
	_ types.Container,
	_ string,
	_ string,
	_ ...string,
) (string, error) {
	f.t.Fatal("the release tag lookup must not run without the changelog feature enabled")

	return "", nil
}

// stubFetcher returns a fixed tag for every candidate.
type stubFetcher struct {
	tag   string
	calls []string
}

func (f *stubFetcher) FetchDigestForTag(
	_ *zerolog.Logger,
	_ context.Context,
	_ types.Container,
	_ string,
	tag string,
	_ ...string,
) (string, error) {
	f.calls = append(f.calls, tag)

	return newDigest, nil
}

// newProgressWithContainer returns a progress map holding c as updated, built
// the way an update session builds it.
func newProgressWithContainer(t *testing.T, c *container.Container) *session.Progress {
	t.Helper()

	progress := session.Progress{}
	progress.AddScanned(newTestLogger(t), c, c.ImageID(), types.UpdateParams{})
	progress.MarkForUpdate(newTestLogger(t), c.ID())

	return &progress
}

// newReportContainer builds a real container so the changelog gate, which reads
// container labels, is exercised rather than bypassed.
func newReportContainer(t *testing.T, labels, imageLabels map[string]string) *container.Container {
	t.Helper()

	const name = "app"

	const image = "org/app:latest"

	imageConfig := &dockerspec.DockerOCIImageConfig{}
	imageConfig.Labels = imageLabels

	c := container.NewContainer(nil, &dockerContainer.InspectResponse{
		ID:   name,
		Name: "/" + name,
		Config: &dockerContainer.Config{
			Image:  image,
			Labels: labels,
		},
	}, &dockerImage.InspectResponse{
		ID:       "sha256:currentimage",
		Config:   imageConfig,
		RepoTags: []string{image},
	})

	require.Equal(t, types.ContainerID(name), c.ID())

	return c
}

func TestApplyLatestImageMetadataRecordsVersionWithoutEnabledFeature(t *testing.T) {
	t.Parallel()

	c := newReportContainer(t, map[string]string{
		git.RepoLabel: "https://github.com/org/app.git",
	}, map[string]string{
		oci.VersionLabel: "1.2.2",
	})

	progress := newProgressWithContainer(t, c)
	client := stubClient{annotations: oci.Annotations{Version: "1.2.3", Revision: "newrev"}}

	vars, anns := resolveLatestImageMeta(
		newTestLogger(t),
		context.Background(),
		release.NewResolver(panickingFetcher{t}),
		c,
		types.UpdateParams{},
		latestImageMetadata{annotations: client.annotations, digest: newDigest},
	)
	applyLatestImageMeta(newTestLogger(t), progress, c, types.UpdateParams{}, vars, anns)

	status := (*progress)[c.ID()]
	assert.Equal(t, "1.2.2", status.CurrentImageVersion())
	assert.Equal(t, "1.2.3", status.LatestImageVersion())
	assert.Equal(t, "newrev", status.LatestImageRevision())
	// Without the feature the changelog stays on the unversioned release index.
	assert.Equal(t, "https://github.com/org/app/releases", status.Changelog())
}

// A changelog-url label is a value, not a switch. It must not authorize a lookup.
func TestApplyLatestImageMetadataChangelogLabelDoesNotEnableProbe(t *testing.T) {
	t.Parallel()

	c := newReportContainer(t, map[string]string{
		git.RepoLabel:         "https://github.com/org/app.git",
		git.ChangelogURLLabel: "https://example.com/notes/{tag}",
	}, nil)

	progress := newProgressWithContainer(t, c)
	client := stubClient{annotations: oci.Annotations{Version: "1.2.3"}}

	vars, anns := resolveLatestImageMeta(
		newTestLogger(t),
		context.Background(),
		release.NewResolver(panickingFetcher{t}),
		c,
		types.UpdateParams{},
		latestImageMetadata{annotations: client.annotations, digest: newDigest},
	)
	applyLatestImageMeta(newTestLogger(t), progress, c, types.UpdateParams{}, vars, anns)

	status := (*progress)[c.ID()]
	// The label still populates the changelog. An unfilled {tag} placeholder is
	// left in place, and the redaction pass percent-encodes its braces.
	assert.Equal(t, "https://example.com/notes/%7Btag%7D", status.Changelog())
}

func TestApplyLatestImageMetadataResolvesVersionedChangelogWhenEnabled(t *testing.T) {
	t.Parallel()

	c := newReportContainer(t, map[string]string{
		git.RepoLabel: "https://github.com/org/app.git",
	}, nil)

	progress := newProgressWithContainer(t, c)
	fetcher := &stubFetcher{tag: "v1.2.3"}
	client := stubClient{annotations: oci.Annotations{Version: "1.2.3"}}

	vars, anns := resolveLatestImageMeta(
		newTestLogger(t),
		context.Background(),
		release.NewResolver(fetcher),
		c,
		types.UpdateParams{EnableChangelog: true},
		latestImageMetadata{annotations: client.annotations, digest: newDigest},
	)
	applyLatestImageMeta(newTestLogger(t), progress, c, types.UpdateParams{EnableChangelog: true}, vars, anns)

	status := (*progress)[c.ID()]
	assert.Equal(t, "https://github.com/org/app/releases/tag/v1.2.3", status.Changelog())
	assert.Equal(t, "1.2.3", status.LatestImageVersion())
}

// The per-container label enables the feature without the global flag.
func TestApplyLatestImageMetadataEnabledByLabel(t *testing.T) {
	t.Parallel()

	c := newReportContainer(t, map[string]string{
		git.RepoLabel:            "https://github.com/org/app.git",
		git.EnableChangelogLabel: "true",
	}, nil)

	progress := newProgressWithContainer(t, c)
	fetcher := &stubFetcher{tag: "v1.2.3"}
	client := stubClient{annotations: oci.Annotations{Version: "1.2.3"}}

	vars, anns := resolveLatestImageMeta(
		newTestLogger(t),
		context.Background(),
		release.NewResolver(fetcher),
		c,
		types.UpdateParams{},
		latestImageMetadata{annotations: client.annotations, digest: newDigest},
	)
	applyLatestImageMeta(newTestLogger(t), progress, c, types.UpdateParams{}, vars, anns)

	assert.Equal(
		t,
		"https://github.com/org/app/releases/tag/v1.2.3",
		(*progress)[c.ID()].Changelog(),
	)
}

// A probe that finds nothing must leave the unversioned link in place.
func TestApplyLatestImageMetadataFallsBackWhenProbeMisses(t *testing.T) {
	t.Parallel()

	c := newReportContainer(t, map[string]string{
		git.RepoLabel: "https://github.com/org/app.git",
	}, nil)

	progress := newProgressWithContainer(t, c)
	client := stubClient{annotations: oci.Annotations{Version: "1.2.3"}}

	vars, anns := resolveLatestImageMeta(
		newTestLogger(t),
		context.Background(),
		release.NewResolver(&missFetcher{}),
		c,
		types.UpdateParams{EnableChangelog: true},
		latestImageMetadata{annotations: client.annotations, digest: newDigest},
	)
	applyLatestImageMeta(newTestLogger(t), progress, c, types.UpdateParams{EnableChangelog: true}, vars, anns)

	assert.Equal(t, "https://github.com/org/app/releases", (*progress)[c.ID()].Changelog())
}

// The changelog log entry is what makes the legacy notification line appear, and
// it must only be emitted when the feature is enabled.
func TestLogChangelogOnlyWhenEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		labels     map[string]string
		params     types.UpdateParams
		wantEvents int
	}{
		{
			name:       "disabled",
			labels:     map[string]string{git.RepoLabel: "https://github.com/org/app.git"},
			wantEvents: 0,
		},
		{
			name:       "changelog url alone is not enough",
			labels:     map[string]string{git.ChangelogURLLabel: "https://example.com/notes"},
			wantEvents: 0,
		},
		{
			name:       "global flag",
			labels:     map[string]string{git.RepoLabel: "https://github.com/org/app.git"},
			params:     types.UpdateParams{EnableChangelog: true},
			wantEvents: 1,
		},
		{
			name:       "container label",
			labels:     map[string]string{git.EnableChangelogLabel: "true"},
			wantEvents: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := newReportContainer(t, tt.labels, nil)
			meta := container.ReportMeta{Changelog: "https://example.com/notes"}

			var buf bytes.Buffer

			log := zerolog.New(&buf)

			logChangelog(&log, c, tt.params, meta)

			events := 0

			for line := range bytes.Lines(buf.Bytes()) {
				if len(line) == 0 {
					continue
				}

				var entry map[string]any
				require.NoError(t, json.Unmarshal(line, &entry))

				events++
			}

			assert.Equal(t, tt.wantEvents, events, "log output: %s", buf.String())
		})
	}
}

func TestLogChangelogSkipsEmptyURL(t *testing.T) {
	t.Parallel()

	c := newReportContainer(t, nil, nil)

	var buf bytes.Buffer

	log := zerolog.New(&buf)

	logChangelog(&log, c, types.UpdateParams{EnableChangelog: true}, container.ReportMeta{})
	logChangelog(&log, nil, types.UpdateParams{EnableChangelog: true}, container.ReportMeta{
		Changelog: "https://example.com/notes",
	})

	assert.Empty(t, buf.String())
}

// A Git rebuild must produce exactly one Changelog entry, resolved from the Git
// tag. The registry path is skipped for a Git-watched container, and the second
// half of this test shows what that skip prevents: the same container driven
// through the registry helpers emits a second, unversioned entry.
func TestGitRebuildEmitsExactlyOneChangelogEntry(t *testing.T) {
	t.Parallel()

	c := newReportContainer(t, map[string]string{
		git.RepoLabel: "https://github.com/org/app.git",
		git.RefLabel:  "main",
	}, nil)

	params := types.UpdateParams{EnableGitMonitoring: true, EnableChangelog: true}
	progress := newProgressWithContainer(t, c)

	var buf bytes.Buffer

	log := zerolog.New(&buf)

	sess := newGitSession(internalGit.New(newTestLogger(t), internalGit.Options{Timeout: time.Second}))
	docker := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	err := sess.buildOne(
		&log,
		t.Context(),
		docker,
		c,
		internalGit.CheckResult{
			Stale:  true,
			Commit: "0123456789abcdef0123456789abcdef01234567",
			Tag:    "v1.2.3",
		},
		params,
		progress,
	)
	require.NoError(t, err)

	assert.Equal(t, 1, countChangelogEntries(buf.Bytes()), "build path should emit one: %s", buf.String())
	assert.Equal(t, "https://github.com/org/app/releases/tag/v1.2.3", (*progress)[c.ID()].Changelog())

	// The registry path, if it ran for this container, would add a second entry
	// pointing at the unversioned release index. That is why executeUpdate
	// resolves latest image metadata only when the container is not Git-watched.
	buf.Reset()

	vars, anns := resolveLatestImageMeta(
		&log,
		t.Context(),
		release.NewResolver(panickingFetcher{t}),
		c,
		params,
		latestImageMetadata{annotations: oci.Annotations{Version: "10.11.6"}},
	)
	applyLatestImageMeta(&log, progress, c, params, vars, anns)

	assert.Equal(t, 1, countChangelogEntries(buf.Bytes()), "registry path would duplicate: %s", buf.String())
	assert.Equal(t, "https://github.com/org/app/releases", (*progress)[c.ID()].Changelog())
}

// countChangelogEntries returns how many Changelog notification entries appear
// in the captured log output.
func countChangelogEntries(output []byte) int {
	entries := 0

	for line := range bytes.Lines(output) {
		if len(line) == 0 {
			continue
		}

		var entry map[string]any
		if json.Unmarshal(line, &entry) != nil {
			continue
		}

		if entry["message"] == "Changelog" {
			entries++
		}
	}

	return entries
}

// stubClient returns fixed annotations for any image reference.
type stubClient struct {
	annotations oci.Annotations
}

func (c stubClient) GetImageAnnotations(_ context.Context, _ string) oci.Annotations {
	return c.annotations
}

// missFetcher reports no tag for any candidate.
type missFetcher struct{}

func (f *missFetcher) FetchDigestForTag(
	_ *zerolog.Logger,
	_ context.Context,
	_ types.Container,
	_ string,
	_ string,
	_ ...string,
) (string, error) {
	return "", nil
}
