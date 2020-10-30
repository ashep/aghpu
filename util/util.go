package util

import (
	"net/url"
	"regexp"
	"strings"
)

func TidyHtmlText(s string) string {
	// Replace multiple whitspaces with one
	s = regexp.MustCompile(`(\s{2,}|\x{00A0})`).ReplaceAllLiteralString(s, " ")
	// s = regexp.MustCompile(`\x{00A0}`).ReplaceAllLiteralString(s, " ")

	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.Trim(s, " ")

	return s
}

func CombineUrl(a string, b string, args *url.Values) string {
	aUrl, err := url.Parse(a)
	if err != nil {
		return ""
	}

	if b != "" {
		bUrl, err := url.Parse(b)
		if err != nil {
			return ""
		}

		aUrl.Path += bUrl.Path
		aUrl.Path = strings.ReplaceAll(aUrl.Path, "//", "/")
	}

	if args != nil {
		q := aUrl.Query()
		for k, v := range *args {
			for _, sv := range v {
				q.Add(k, sv)
			}
		}

		aUrl.RawQuery = q.Encode()
	}

	return aUrl.String()
}
