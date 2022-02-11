module github.com/containrrr/watchtower

go 1.12

// Use non-vulnerable runc (until github.com/containerd/containerd v1.6.0 is stable)
replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.3

require (
	github.com/containerd/containerd v1.5.9 // indirect
	github.com/containrrr/shoutrrr v0.5.2
	github.com/docker/cli v20.10.8+incompatible
	github.com/docker/distribution v2.8.0+incompatible
	github.com/docker/docker v20.10.8+incompatible
	github.com/docker/docker-credential-helpers v0.6.1 // indirect
	github.com/docker/go-connections v0.4.0
	github.com/johntdyer/slack-go v0.0.0-20180213144715-95fac1160b22 // indirect
	github.com/johntdyer/slackrus v0.0.0-20180518184837-f7aae3243a07
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c // indirect
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/prometheus/client_golang v1.7.1
	github.com/robfig/cron v0.0.0-20180505203441-b41be1df6967
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.3
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sys v0.0.0-20210831042530-f4d43177bf5e // indirect
)
