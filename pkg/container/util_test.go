package container_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/containrrr/watchtower/pkg/container"
)

var _ = Describe("container utils", func() {
	Describe("ShortID", func() {
		When("given a normal image ID", func() {
			When("it contains a sha256 prefix", func() {
				It("should return that ID in short version", func() {
					actual := ShortID("sha256:0123456789abcd00000000001111111111222222222233333333334444444444")
					Expect(actual).To(Equal("0123456789ab"))
				})
			})
			When("it doesn't contain a prefix", func() {
				It("should return that ID in short version", func() {
					actual := ShortID("0123456789abcd00000000001111111111222222222233333333334444444444")
					Expect(actual).To(Equal("0123456789ab"))
				})
			})
		})
		When("given a short image ID", func() {
			When("it contains no prefix", func() {
				It("should return the same string", func() {
					Expect(ShortID("0123456789ab")).To(Equal("0123456789ab"))
				})
			})
			When("it contains a the sha256 prefix", func() {
				It("should return the ID without the prefix", func() {
					Expect(ShortID("sha256:0123456789ab")).To(Equal("0123456789ab"))
				})
			})
		})
		When("given an ID with an unknown prefix", func() {
			It("should return a short version of that ID including the prefix", func() {
				Expect(ShortID("md5:0123456789ab")).To(Equal("md5:0123456789ab"))
				Expect(ShortID("md5:0123456789abcdefg")).To(Equal("md5:0123456789ab"))
				Expect(ShortID("md5:01")).To(Equal("md5:01"))
			})
		})
	})
})
