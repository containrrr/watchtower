package templates

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var Funcs = template.FuncMap{
	"ToUpper": strings.ToUpper,
	"ToLower": strings.ToLower,
	"ToJSON":  toJSON,
	"Title":   cases.Title(language.AmericanEnglish).String,
}

func toJSON(v interface{}) string {
	var bytes []byte
	var err error
	if bytes, err = json.MarshalIndent(v, "", "  "); err != nil {
		return fmt.Sprintf("failed to marshal JSON in notification template: %v", err)
	}
	return string(bytes)
}
