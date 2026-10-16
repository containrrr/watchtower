package container

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("Git wrappers", func() {
	ginkgo.Describe("SetLabel", func() {
		ginkgo.It("writes a config label", func() {
			c := MockContainer()
			c.SetLabel(git.RepoLabel, "https://github.com/org/app.git")

			value, ok := c.GetLabel(git.RepoLabel)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(value).To(gomega.Equal("https://github.com/org/app.git"))
		})

		ginkgo.It("overwrites an existing label", func() {
			c := MockContainer(WithLabels(map[string]string{git.RepoLabel: "old"}))
			c.SetLabel(git.RepoLabel, "new")

			value, ok := c.GetLabel(git.RepoLabel)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(value).To(gomega.Equal("new"))
		})

		ginkgo.It("creates the labels map when it is nil", func() {
			c := MockContainer()
			c.containerInfo.Config.Labels = nil
			c.SetLabel(git.RefLabel, "main")

			gomega.Expect(c.containerInfo.Config.Labels).To(gomega.HaveKeyWithValue(git.RefLabel, "main"))
		})

		ginkgo.It("is a no-op for a nil receiver or empty key", func() {
			var c *Container
			c.SetLabel(git.RepoLabel, "https://github.com/org/app.git")

			c = MockContainer()
			c.SetLabel("", "value")
			gomega.Expect(c.containerInfo.Config.Labels).NotTo(gomega.HaveKey(""))
		})

		ginkgo.It("removes an existing label", func() {
			c := MockContainer()
			c.SetLabel(git.LastTagLabel, "v1")
			c.DeleteLabel(git.LastTagLabel)
			_, ok := c.GetLabel(git.LastTagLabel)
			gomega.Expect(ok).To(gomega.BeFalse())
		})

		ginkgo.It("is a no-op when container info or config is missing", func() {
			c := MockContainer()
			c.containerInfo.Config = nil
			c.SetLabel(git.RepoLabel, "https://github.com/org/app.git")

			c = MockContainer()
			c.containerInfo = nil
			c.SetLabel(git.RepoLabel, "https://github.com/org/app.git")
		})
	})

	ginkgo.Describe("ResolveAssociation", func() {
		ginkgo.It("returns false for a nil container", func() {
			assoc, ok := git.ResolveAssociation(nil, types.UpdateParams{})
			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(assoc).To(gomega.Equal(git.Association{}))
		})

		ginkgo.It("prefers a git-repo label over an image mapping", func() {
			c := MockContainer(
				WithImageName("myapp:latest"),
				WithLabels(map[string]string{
					git.RepoLabel: "https://github.com/org/from-label.git",
					git.RefLabel:  "release",
				}),
			)

			assoc, ok := git.ResolveAssociation(c, types.UpdateParams{
				GitImages: map[string]types.GitImage{
					"myapp:latest": {Repo: "https://github.com/org/from-map.git", Ref: "mapped"},
				},
			})
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(assoc.Repo).To(gomega.Equal("https://github.com/org/from-label.git"))
			gomega.Expect(assoc.Ref).To(gomega.Equal("release"))
		})

		ginkgo.It("uses an exact image mapping when labels are absent", func() {
			c := MockContainer(WithImageName("myapp:latest"))

			assoc, ok := git.ResolveAssociation(c, types.UpdateParams{
				GitImages: map[string]types.GitImage{
					"myapp:latest": {Repo: "https://github.com/org/mapped.git", Ref: "v1"},
				},
			})
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(assoc.Repo).To(gomega.Equal("https://github.com/org/mapped.git"))
			gomega.Expect(assoc.Ref).To(gomega.Equal("v1"))
		})

		ginkgo.It("returns false when the container is not associated", func() {
			c := MockContainer(WithImageName("nginx:latest"))
			assoc, ok := git.ResolveAssociation(c, types.UpdateParams{})
			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(assoc).To(gomega.Equal(git.Association{}))
		})
	})

	ginkgo.Describe("ShouldMonitor", func() {
		ginkgo.It("returns false for a nil container", func() {
			gomega.Expect(git.ShouldMonitor(nil, nil, types.UpdateParams{EnableGitMonitoring: true})).To(gomega.BeFalse())
		})

		ginkgo.It("returns false for Watchtower even when associated", func() {
			c := MockContainer(WithLabels(map[string]string{
				watchtowerLabel: "true",
				git.RepoLabel:   "https://github.com/nicholas-fedor/watchtower.git",
			}))

			gomega.Expect(git.ShouldMonitor(nil, c, types.UpdateParams{EnableGitMonitoring: true})).To(gomega.BeFalse())
		})

		ginkgo.It("returns true when associated and monitoring is enabled", func() {
			c := MockContainer(WithLabels(map[string]string{
				git.RepoLabel: "https://github.com/org/app.git",
			}))

			gomega.Expect(git.ShouldMonitor(nil, c, types.UpdateParams{EnableGitMonitoring: true})).To(gomega.BeTrue())
		})

		ginkgo.It("returns false when the container turns watch off", func() {
			c := MockContainer(WithLabels(map[string]string{
				git.RepoLabel:  "https://github.com/org/app.git",
				git.WatchLabel: "false",
			}))

			gomega.Expect(git.ShouldMonitor(nil, c, types.UpdateParams{EnableGitMonitoring: true})).To(gomega.BeFalse())
		})

		ginkgo.It("returns false when watch is on but the container is not associated", func() {
			c := MockContainer(
				WithImageName("nginx:latest"),
				WithLabels(map[string]string{git.WatchLabel: "true"}),
			)
			nop := zerolog.Nop()

			gomega.Expect(git.ShouldMonitor(&nop, c, types.UpdateParams{})).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("LastGitStamp", func() {
		ginkgo.It("returns empty values for a nil container", func() {
			commit, tag := git.LastStamp(nil)
			gomega.Expect(commit).To(gomega.BeEmpty())
			gomega.Expect(tag).To(gomega.BeEmpty())
		})

		ginkgo.It("reads last-commit and last-tag labels", func() {
			c := MockContainer(WithLabels(map[string]string{
				git.LastCommitLabel: "abc123",
				git.LastTagLabel:    "v1.2.3",
			}))

			commit, tag := git.LastStamp(c)
			gomega.Expect(commit).To(gomega.Equal("abc123"))
			gomega.Expect(tag).To(gomega.Equal("v1.2.3"))
		})
	})

	ginkgo.Describe("ApplyGitStamp", func() {
		ginkgo.It("is a no-op for a nil container", func() {
			ApplyGitStamp(nil, "abc", "v1.0.0")
		})

		ginkgo.It("writes the commit and omits an empty tag", func() {
			c := MockContainer()
			c.SetLabel(git.LastTagLabel, "v0.1.0")
			ApplyGitStamp(c, "abc123", "")

			commit, ok := c.GetLabel(git.LastCommitLabel)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(commit).To(gomega.Equal("abc123"))

			_, ok = c.GetLabel(git.LastTagLabel)
			gomega.Expect(ok).To(gomega.BeFalse())
		})

		ginkgo.It("writes commit and tag", func() {
			c := MockContainer()
			ApplyGitStamp(c, "abc123", "v1.2.3")

			commit, tag := git.LastStamp(c)
			gomega.Expect(commit).To(gomega.Equal("abc123"))
			gomega.Expect(tag).To(gomega.Equal("v1.2.3"))
		})
	})

	ginkgo.Describe("ApplyGitAssociation", func() {
		ginkgo.It("is a no-op for a nil container or empty repo", func() {
			ApplyGitAssociation(nil, gitAssociation{Repo: "https://github.com/org/app.git"}, true)

			c := MockContainer()
			ApplyGitAssociation(c, gitAssociation{}, true)
			gomega.Expect(c.containerInfo.Config.Labels).NotTo(gomega.HaveKey(git.RepoLabel))
		})

		ginkgo.It("persists association labels and watch", func() {
			c := MockContainer()
			ApplyGitAssociation(c, gitAssociation{
				Repo:       "https://github.com/org/app.git",
				Ref:        "main",
				Policy:     types.GitPolicyPatch,
				Host:       "https://git.example.com:3000",
				Dockerfile: "build/docker/Dockerfile",
				Context:    ".",
			}, true)

			gomega.Expect(c.containerInfo.Config.Labels).To(gomega.Equal(map[string]string{
				git.RepoLabel:         "https://github.com/org/app.git",
				git.RefLabel:          "main",
				git.SemverPolicyLabel: types.GitPolicyPatch,
				git.HostLabel:         "https://git.example.com:3000",
				git.DockerfileLabel:   "build/docker/Dockerfile",
				git.ContextLabel:      ".",
				git.WatchLabel:        "true",
			}))
		})

		ginkgo.It("preserves bare local repository paths in labels", func() {
			c := MockContainer()
			ApplyGitAssociation(c, gitAssociation{Repo: "../repos/app", Ref: "main"}, true)

			repo, ok := c.GetLabel(git.RepoLabel)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(repo).To(gomega.Equal("../repos/app"))
		})

		ginkgo.It("preserves SSH identities and benign local path queries", func() {
			sshContainer := MockContainer()
			ApplyGitAssociation(sshContainer, gitAssociation{Repo: "ssh://git@git.example.com/org/app.git"}, true)
			sshRepo, ok := sshContainer.GetLabel(git.RepoLabel)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(sshRepo).To(gomega.Equal("ssh://git@git.example.com/org/app.git"))

			pathContainer := MockContainer()
			ApplyGitAssociation(pathContainer, gitAssociation{Repo: "/srv/repos/app?release=1"}, true)
			pathRepo, ok := pathContainer.GetLabel(git.RepoLabel)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(pathRepo).To(gomega.Equal("/srv/repos/app?release=1"))
		})

		ginkgo.It("redacts credentials and query tokens before persisting labels", func() {
			c := MockContainer()
			ApplyGitAssociation(c, gitAssociation{
				Repo: "https://user:secret@github.com/org/app.git?access_token=query#main",
				Host: "https://git.example.com:3000?token=host-secret",
				Ref:  "main",
			}, true)

			repo, ok := c.GetLabel(git.RepoLabel)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(repo).To(gomega.Equal("https://github.com/org/app.git#main"))
			gomega.Expect(repo).NotTo(gomega.ContainSubstring("secret"))

			host, ok := c.GetLabel(git.HostLabel)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(host).To(gomega.Equal("https://git.example.com:3000"))
			gomega.Expect(host).NotTo(gomega.ContainSubstring("secret"))
		})

		ginkgo.It("removes association labels when a repository cannot be sanitized", func() {
			c := MockContainer(WithLabels(map[string]string{
				git.RepoLabel: "https://user:secret@github.com/org/app.git",
			}))
			ApplyGitAssociation(c, gitAssociation{Repo: "://invalid"}, true)

			_, ok := c.GetLabel(git.RepoLabel)
			gomega.Expect(ok).To(gomega.BeFalse())
		})

		ginkgo.It("does not persist process defaults", func() {
			c := MockContainer()
			ApplyGitAssociation(c, gitAssociation{
				Repo:                  "https://github.com/org/app.git",
				Ref:                   "main",
				Policy:                types.GitPolicyNone,
				Dockerfile:            "Dockerfile",
				Context:               ".",
				RefFromDefault:        true,
				PolicyFromDefault:     true,
				DockerfileFromDefault: true,
				ContextFromDefault:    true,
			}, false)

			gomega.Expect(c.containerInfo.Config.Labels).To(gomega.Equal(map[string]string{
				git.RepoLabel: "https://github.com/org/app.git",
			}))
		})

		ginkgo.It("omits empty optional fields and does not force watch", func() {
			c := MockContainer()
			c.SetLabel(git.RefLabel, "old")
			c.SetLabel(git.WatchLabel, "true")
			ApplyGitAssociation(c, gitAssociation{Repo: "https://github.com/org/app.git"}, false)

			gomega.Expect(c.containerInfo.Config.Labels).To(gomega.Equal(map[string]string{
				git.RepoLabel: "https://github.com/org/app.git",
			}))
		})
	})
})
