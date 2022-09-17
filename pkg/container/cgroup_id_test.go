package container

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetRunningContainerID", func() {
	When("a matching container ID is found", func() {
		It("should return that container ID", func() {
			cid := getRunningContainerIDFromString(`
15:name=systemd:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
14:misc:/
13:rdma:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
12:pids:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
11:hugetlb:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
10:net_prio:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
9:perf_event:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
8:net_cls:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
7:freezer:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
6:devices:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
5:blkio:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
4:cpuacct:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
3:cpu:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
2:cpuset:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
1:memory:/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
0::/docker/991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377
			`)
			Expect(cid).To(BeEquivalentTo(`991b6b42691449d3ce90192ff9f006863dcdafc6195e227aeefa298235004377`))
		})
	})
	When("no matching container ID could be found", func() {
		It("should return that container ID", func() {
			cid := getRunningContainerIDFromString(`14:misc:/`)
			Expect(cid).To(BeEmpty())
		})
	})
})

//
