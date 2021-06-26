package container_test

import (
	wt "github.com/containrrr/watchtower/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("container utils", func() {
	Describe("ShortID", func() {
		When("given a normal image ID", func() {
			When("it contains a sha256 prefix", func() {
				It("should return that ID in short version", func() {
					actual := shortID("sha256:0123456789abcd00000000001111111111222222222233333333334444444444")
					Expect(actual).To(Equal("0123456789ab"))
				})
			})
			When("it doesn't contain a prefix", func() {
				It("should return that ID in short version", func() {
					actual := shortID("0123456789abcd00000000001111111111222222222233333333334444444444")
					Expect(actual).To(Equal("0123456789ab"))
				})
			})
		})
		When("given a short image ID", func() {
			When("it contains no prefix", func() {
				It("should return the same string", func() {
					Expect(shortID("0123456789ab")).To(Equal("0123456789ab"))
				})
			})
			When("it contains a the sha256 prefix", func() {
				It("should return the ID without the prefix", func() {
					Expect(shortID("sha256:0123456789ab")).To(Equal("0123456789ab"))
				})
			})
		})
		When("given an ID with an unknown prefix", func() {
			It("should return a short version of that ID including the prefix", func() {
				Expect(shortID("md5:0123456789ab")).To(Equal("md5:0123456789ab"))
				Expect(shortID("md5:0123456789abcdefg")).To(Equal("md5:0123456789ab"))
				Expect(shortID("md5:01")).To(Equal("md5:01"))
			})
		})
	})
})

func shortID(id string) string {
	// Proxy to the types implementation, relocated due to package dependency resolution
	return wt.ImageID(id).ShortID()
}
