package flags

import (
	"github.com/spf13/pflag"
)

type flagBuilder struct {
	flags              *pflag.FlagSet
	Deprecate          bool
	Hide               bool
	DeprecationMessage string
	Prefix             string
}

func NewDeprecator(flags *pflag.FlagSet, message string) *flagBuilder {
	return &flagBuilder{
		flags:              flags,
		Deprecate:          true,
		Hide:               false,
		DeprecationMessage: message,
		Prefix:             "",
	}
}

//goland:noinspection GoUnusedExportedFunction
func New(flags *pflag.FlagSet) *flagBuilder {
	return &flagBuilder{
		flags:              flags,
		Deprecate:          false,
		Hide:               false,
		DeprecationMessage: "",
		Prefix:             "",
	}
}

func (fb *flagBuilder) MarkFlag(name string) {
	if fb.Deprecate {
		must(fb.flags.MarkDeprecated(name, fb.DeprecationMessage))
	}
	if fb.Hide {
		must(fb.flags.MarkHidden(name))
	}
}

func (fb *flagBuilder) ResolveName(name string) string {
	if fb.Prefix == "" {
		return name
	}
	return fb.Prefix + name
}

func (fb *flagBuilder) StringP(name string, shorthand string, value string, usage string) {
	fullName := fb.ResolveName(name)
	fb.flags.StringP(fullName, shorthand, value, usage)
	fb.MarkFlag(fullName)
}

func (fb *flagBuilder) StringSliceP(name string, shorthand string, value []string, usage string) {
	fullName := fb.ResolveName(name)
	fb.flags.StringSliceP(fullName, shorthand, value, usage)
	fb.MarkFlag(fullName)
}

func (fb *flagBuilder) StringArrayP(name string, shorthand string, value []string, usage string) {
	fullName := fb.ResolveName(name)
	fb.flags.StringArrayP(fullName, shorthand, value, usage)
	fb.MarkFlag(fullName)
}

func (fb *flagBuilder) BoolP(name string, shorthand string, value bool, usage string) {
	fullName := fb.ResolveName(name)
	fb.flags.BoolP(fullName, shorthand, value, usage)
	fb.MarkFlag(fullName)
}

func (fb *flagBuilder) IntP(name string, shorthand string, value int, usage string) {
	fullName := fb.ResolveName(name)
	fb.flags.IntP(fullName, shorthand, value, usage)
	fb.MarkFlag(fullName)
}

func (fb *flagBuilder) String(name string, value string, usage string) {
	fb.StringP(name, "", value, usage)
}

func (fb *flagBuilder) StringSlice(name string, value []string, usage string) {
	fb.flags.StringSliceP(name, "", value, usage)
}

func (fb *flagBuilder) StringArray(name string, value []string, usage string) {
	fb.flags.StringArrayP(name, "", value, usage)
}

func (fb *flagBuilder) Bool(name string, value bool, usage string) {
	fb.BoolP(name, "", value, usage)
}

func (fb *flagBuilder) Int(name string, value int, usage string) {
	fb.IntP(name, "", value, usage)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
