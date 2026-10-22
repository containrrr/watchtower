package git

import "testing"

// FuzzValidateRef verifies user-supplied refs never panic.
func FuzzValidateRef(f *testing.F) {
	f.Add("main")
	f.Add("v1.2.3")
	f.Add("release/1.2")
	f.Add("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	f.Add("foo..bar")
	f.Add("has space")
	f.Add("")
	f.Add("refs/heads/main")
	f.Add("refs/tags/v1.2.3")
	f.Add("@{")
	f.Add("..")
	f.Add("a\x00b")

	f.Fuzz(func(_ *testing.T, ref string) {
		_ = validateRef(ref)
	})
}
