package util

import (
	"net/url"
	"regexp"
	"strings"
)

// TidyHTMLText cleans an HTML text
func TidyHTMLText(s string) string {
	// Replace multiple whitspaces with one
	s = regexp.MustCompile(`(\s{2,}|\x{00A0})`).ReplaceAllLiteralString(s, " ")
	// s = regexp.MustCompile(`\x{00A0}`).ReplaceAllLiteralString(s, " ")

	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.Trim(s, " ")

	return s
}

// CombineURL combines two URLs
func CombineURL(a string, b string, args *url.Values) string {
	aURL, err := url.Parse(a)
	if err != nil {
		return ""
	}

	if b != "" {
		bURL, err := url.Parse(b)
		if err != nil {
			return ""
		}

		aURL.Path += bURL.Path
		aURL.Path = strings.ReplaceAll(aURL.Path, "//", "/")
	}

	if args != nil {
		q := aURL.Query()
		for k, v := range *args {
			for _, sv := range v {
				q.Add(k, sv)
			}
		}

		aURL.RawQuery = q.Encode()
	}

	return aURL.String()
}

// IndexString returns position of string in a slice
func IndexString(s []string, em string) int {
	for i := range s {
		if s[i] == em {
			return i
		}
	}

	return -1
}

// AppendUniqueString appends a string to a slice
func AppendUniqueString(s []string, em string) []string {
	if IndexString(s, em) == -1 {
		s = append(s, em)
	}

	return s
}
