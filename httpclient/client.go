package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ashep/aghpu/logger"
	"github.com/ashep/aghpu/util"

	"github.com/PuerkitoBio/goquery"
)

// Cli is a HTTP client
type Cli struct {
	client    *http.Client
	debug     bool
	dumpDir   string
	id        string
	log       *logger.Logger
	reqNum    int
	userAgent string
}

// DumpTransaction dumps an HTTP transaction content into a file
func (c *Cli) DumpTransaction(req *http.Request, resp *http.Response, reqBody *[]byte, respBody *[]byte) {
	// Create a dump file
	fPath := filepath.Join(c.dumpDir, fmt.Sprintf("%04d.txt", c.reqNum))
	f, err := os.Create(fPath)
	if err != nil {
		c.log.Err("error creating http dump file %v: %v", fPath, err)
		return
	}
	defer f.Close()

	// Dump method and URL
	f.WriteString(fmt.Sprintf("%v %v\n\n", req.Method, req.URL))

	// Dump request headers
	for k, h := range req.Header {
		for _, v := range h {
			if _, err := f.Write([]byte(fmt.Sprintf("%v: %v\n", k, v))); err != nil {
				c.log.Err("error writing http dump file %v: %v", fPath, err)
				return
			}
		}
	}
	f.WriteString("\n")

	// Dump request body
	if len(*reqBody) > 0 {
		if _, err := f.Write(*reqBody); err != nil {
			c.log.Err("error writing http dump file %v: %v", fPath, err)
			return
		}
	} else {
		if _, err := f.Write([]byte("EMPTY BODY")); err != nil {
			c.log.Err("error writing http dump file %v: %v", fPath, err)
			return
		}
	}

	f.WriteString("\n")

	// Request and response separator
	f.WriteString("\n---\n\n")

	// Dump response headers
	for k, h := range resp.Header {
		for _, v := range h {
			if _, err := f.Write([]byte(fmt.Sprintf("%v: %v\n", k, v))); err != nil {
				c.log.Err("error writing http dump file %v: %v", fPath, err)
				return
			}
		}
	}
	f.WriteString("\n")

	// Dump response body
	if len(*respBody) > 0 {
		if _, err := f.Write(*respBody); err != nil {
			c.log.Err("error writing http dump file %v: %v", fPath, err)
			return
		}
	} else {
		if _, err := f.Write([]byte("EMPTY BODY")); err != nil {
			c.log.Err("error writing http dump file %v: %v", fPath, err)
			return
		}
	}
}

// Request performs an HTTP request
func (c *Cli) Request(method, u string, header *http.Header, body []byte) (*http.Response, *[]byte, error) {
	var (
		err      error
		req      *http.Request
		resp     *http.Response
		respBody []byte
	)

	// Ensure headers
	if header == nil {
		header = &http.Header{}
	}
	if header.Get("user-agent") == "" {
		header.Add("user-agent", c.userAgent)
	}
	if header.Get("accept") == "" {
		header.Add("accept", "*/*")
	}
	if header.Get("accept-language") == "" {
		header.Add("accept-language", "en-US,en;q=0.9,ru;q=0.8,uk;q=0.7")
	}
	if header.Get("cache-control") == "" {
		header.Add("cache-control", "max-age=0")
	}

	// Create request
	req, err = http.NewRequest(method, u, strings.NewReader(string(body)))
	if err != nil {
		return nil, nil, err
	}
	req.Header = *header

	// Send request
	c.reqNum++
	resp, err = c.client.Do(req)
	if err != nil {
		c.log.Err("req #%d: %v %v; error: %v", c.reqNum, method, u, err)
	} else {
		c.log.Debug("req #%d: %v %v; status: %v", c.reqNum, method, u, resp.Status)
	}

	// Load response body
	if resp != nil {
		defer resp.Body.Close()
		if respBody, err = ioutil.ReadAll(resp.Body); err != nil {
			return resp, nil, fmt.Errorf("error while reading response body: %v", err)
		}

		if c.debug {
			c.DumpTransaction(req, resp, &body, &respBody)
		}
	}

	// Check response status
	if resp.StatusCode >= 400 {
		return resp, &respBody, fmt.Errorf("HTTP response status: %v", resp.Status)
	}

	return resp, &respBody, err
}

// Get perform a GET request
func (c *Cli) Get(u string, args *url.Values, header *http.Header) (*[]byte, error) {
	if args != nil {
		u = util.CombineURL(u, "", args)
	}

	_, body, err := c.Request("GET", u, header, []byte(""))
	return body, err
}

// GetQueryDoc performs a GET request and transform response into a goquery document
func (c *Cli) GetQueryDoc(u string, args *url.Values, header *http.Header) (*goquery.Document, error) {
	body, err := c.Get(u, args, header)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(*body))
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// GetJSON performs a GET HTTP request and parses the response into a JSON
func (c *Cli) GetJSON(u string, args *url.Values, header *http.Header, data interface{}) error {
	if header == nil {
		header = &http.Header{}
	}
	if header.Get("x-requested-with") == "" {
		header.Add("x-requested-with", "XMLHttpRequest")
	}

	body, err := c.Get(u, args, header)
	if err != nil {
		return err
	}

	return json.Unmarshal(*body, data)
}

// GetFile gets a file and stores it on the disk.
//
// If fPath doesn't contain an extension, it will be added automatically.
// In case of success file extension returned
func (c *Cli) GetFile(u string, args *url.Values, header *http.Header, fPath string) (string, error) {
	fExt := ""

	if args != nil {
		u = util.CombineURL(u, "", args)
	}

	resp, body, err := c.Request("GET", u, header, nil)
	if err != nil {
		return "", err
	}

	// Calculate file extension
	if !filepath.IsAbs(fPath) {
		if fPath, err = filepath.Abs(fPath); err != nil {
			return "", err
		}
	}
	if !regexp.MustCompile(`\.[a-zA-Z0-9]+$`).Match([]byte(fPath)) {
		fExtArr, err := mime.ExtensionsByType(resp.Header.Get("Content-Type"))
		if err != nil {
			return "", fmt.Errorf("unable to determine file extension: %v", err)
		}
		fExt = fExtArr[len(fExtArr)-1]
		fPath += fExt
	}

	f, err := os.Create(fPath)
	if err != nil {
		return "", fmt.Errorf("error creating file %v: %v", fPath, err)
	}

	f.Write(*body)
	f.Close()

	return fExt, nil
}

// Post performs a POST request
func (c *Cli) Post(u string, args *url.Values, header *http.Header) (*[]byte, error) {
	if header == nil {
		header = &http.Header{}
	}
	if header.Get("Content-Type") == "" {
		header.Add("Content-Type", "application/x-www-form-urlencoded")
	}

	_, body, err := c.Request("POST", u, header, []byte(args.Encode()))
	return body, err
}

// New instantiates a client
func New(name string, debug bool, dumpDir, userAgent string) (*Cli, error) {
	var err error

	sID := fmt.Sprintf("%d", time.Now().Unix())
	log := logger.New(name, logger.LvInfo)

	if debug {
		log.Info("debug mode enabled")
		log.SetLevel(logger.LvDebug)

		// Calculate dump directory
		dumpDir, err = filepath.Abs(dumpDir)
		if err != nil {
			return nil, err
		}
		dumpDir = filepath.Join(dumpDir, sID)

		// Make dump directory
		err = os.MkdirAll(dumpDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("failed to create dump directory: %v", err)
		}
		log.Info("dump directory: %v\n", dumpDir)
	}

	c := http.Client{}

	c.Jar, err = cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, err
	}

	return &Cli{
		client:    &c,
		debug:     debug,
		dumpDir:   dumpDir,
		id:        sID,
		log:       log,
		userAgent: userAgent,
	}, nil
}
