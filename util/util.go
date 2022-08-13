package util

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	csv "github.com/tushar2708/altcsv"
)

// GetExecDir returns directory where program executable is located.
func GetExecDir() string {
	r, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return r
}

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
func CombineURL(a string, b string, args url.Values) string {
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
		for k, v := range args {
			for _, sv := range v {
				q.Add(k, sv)
			}
		}

		aURL.RawQuery = q.Encode()
	}

	return aURL.String()
}

// GetStrSliceIndex returns position of string in a slice
func GetStrSliceIndex(s []string, em string) int {
	for i := range s {
		if s[i] == em {
			return i
		}
	}

	return -1
}

// AppendUniqueString appends a string to a slice of strings only if the slice doesn't contain the same string
func AppendUniqueString(s []string, em string) []string {
	if GetStrSliceIndex(s, em) == -1 {
		s = append(s, em)
	}

	return s
}

// ReplaceChars replaces all `chars` in `s` with `repl`
func ReplaceChars(s, chars, repl string) string {
	for i := 0; i < len(chars); i++ {
		s = strings.ReplaceAll(s, string(chars[i]), repl)
	}

	return s
}

// SanitizeFilename makes filename sane
func SanitizeFilename(s, repl string) string {
	return ReplaceChars(s, " \\/,:;`~+!\"'#$%^&*(){}[]", repl)
}

// StrSliceToMap merges two string slices into a map of strings
func StrSliceToMap(keys, values []string) map[string]string {
	r := make(map[string]string)
	for i, k := range keys {
		r[k] = values[i]
	}

	return r
}

// MapStrBoolToSlice returns keys of string keyed map as a slice.
func MapStrBoolToSlice(m map[string]bool) []string {
	var r []string
	for k, _ := range m {
		r = append(r, k)
	}
	return r
}

// CSVToMap loads a CSV file into a map
func CSVToMap(fPath string) ([]map[string]string, error) {
	fp, err := os.Open(fPath)
	if err != nil {
		return nil, err
	}
	defer func(fp *os.File) { _ = fp.Close() }(fp)

	rd := csv.NewReader(fp)
	records, err := rd.ReadAll()
	if err != nil {
		return nil, err
	}

	fNames := make([]string, 0)
	r := make([]map[string]string, 0)
	for i, row := range records {
		if i == 0 {
			fNames = row
			continue
		}

		rowMap := make(map[string]string)
		for k, col := range row {
			rowMap[fNames[k]] = col
		}

		r = append(r, rowMap)
	}

	return r, nil
}
