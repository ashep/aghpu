package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net"
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
	id        string
	debug     bool
	dumpDir   string
	reqTries  int
	userAgent string

	reqErrH    ErrorHandler
	reqNum     int
	currentURL *url.URL

	cli *http.Client
	l   *logger.Logger
}

// ErrorHandler is HTTP request error handler
type ErrorHandler func(c *Cli, req *http.Request, err error, tryN int) error

// Client returns underlying http client
func (c *Cli) Client() *http.Client {
	return c.cli
}

// SetRequestErrorHandler sets HTTP request error handler
func (c *Cli) SetRequestErrorHandler(fn ErrorHandler) {
	c.reqErrH = fn
}

// CurrentURL returns last requested URL taking redirects into account
func (c *Cli) CurrentURL() *url.URL {
	return c.currentURL
}

// Reset resets the client
func (c *Cli) Reset() error {
	c.currentURL = nil

	if j, err := cookiejar.New(&cookiejar.Options{}); err == nil {
		c.cli.Jar = j
	} else {
		return err
	}

	return nil
}

// DumpTransaction dumps an HTTP transaction content into a file
func (c *Cli) DumpTransaction(req *http.Request, resp *http.Response, reqBody []byte, respBody []byte) {
	// Create a dump file
	fPath := filepath.Join(c.dumpDir, fmt.Sprintf("%04d.txt", c.reqNum))
	f, err := os.Create(fPath)
	if err != nil {
		c.l.Err("error creating http dump file %v: %v", fPath, err)
		return
	}
	defer func() {
		_ = f.Close()
	}()

	// Dump method and URL
	if _, err := f.WriteString(fmt.Sprintf("%v %v\n\n", req.Method, req.URL)); err != nil {
		c.l.Err("failed to write string: %s", err.Error())
		return
	}

	// Dump request headers
	for k, h := range req.Header {
		for _, v := range h {
			if _, err := f.Write([]byte(fmt.Sprintf("%v: %v\n", k, v))); err != nil {
				c.l.Err("error writing http dump file %v: %v", fPath, err)
				return
			}
		}
	}
	if _, err := f.WriteString("\n"); err != nil {
		c.l.Err("failed to write string: %s", err.Error())
		return
	}

	// Dump request body
	if len(reqBody) > 0 {
		if _, err := f.Write(reqBody); err != nil {
			c.l.Err("error writing http dump file %v: %v", fPath, err)
			return
		}
	} else {
		if _, err := f.Write([]byte("EMPTY BODY")); err != nil {
			c.l.Err("error writing http dump file %v: %v", fPath, err)
			return
		}
	}
	if _, err := f.WriteString("\n"); err != nil {
		c.l.Err("failed to write string: %s", err.Error())
		return
	}

	// Request and response separator
	if _, err := f.WriteString("\n---\n\n"); err != nil {
		c.l.Err("failed to write string: %s", err.Error())
		return
	}

	// Dump response headers
	for k, h := range resp.Header {
		for _, v := range h {
			if _, err := f.Write([]byte(fmt.Sprintf("%v: %v\n", k, v))); err != nil {
				c.l.Err("error writing http dump file %v: %v", fPath, err)
				return
			}
		}
	}
	if _, err := f.WriteString("\n"); err != nil {
		c.l.Err("failed to write string: %s", err.Error())
		return
	}

	// Dump response body
	if len(respBody) > 0 {
		if _, err := f.Write(respBody); err != nil {
			c.l.Err("error writing http dump file %v: %v", fPath, err)
			return
		}
	} else {
		if _, err := f.Write([]byte("EMPTY BODY")); err != nil {
			c.l.Err("error writing http dump file %v: %v", fPath, err)
			return
		}
	}
}

// Request performs an HTTP request
func (c *Cli) Request(method, u string, header http.Header, body []byte) (*http.Response, []byte, error) {
	var (
		err      error
		req      *http.Request
		resp     *http.Response
		respBody []byte
	)

	// Ensure headers
	if header == nil {
		header = http.Header{}
	}
	if header.Get("User-Agent") == "" {
		header.Set("User-Agent", c.userAgent)
	}
	if header.Get("Accept") == "" {
		header.Set("Accept", "*/*")
	}
	if header.Get("Accept-Language") == "" {
		header.Set("Accept-Language", "en-US,en;q=0.9,ru;q=0.8,uk;q=0.7")
	}
	if header.Get("Cache-Control") == "" {
		header.Set("Cache-Control", "max-age=0")
	}
	if header.Get("Referer") == "" && c.currentURL != nil {
		header.Set("Referer", c.currentURL.String())
	}

	// Create request
	req, err = http.NewRequest(method, u, strings.NewReader(string(body)))
	if err != nil {
		return nil, nil, err
	}
	req.Header = header

	// Send request
	c.reqNum++
	tryN := 1
	for ; ; tryN++ {
		c.currentURL = req.URL
		resp, err = c.cli.Do(req)

		if err == nil {
			break
		}

		c.l.Err("req #%d(%v): %v %v; error: %v", c.reqNum, tryN, method, u, err)

		if tryN == c.reqTries {
			return nil, nil, err
		}

		if c.reqErrH != nil {
			if hErr := c.reqErrH(c, req, err, tryN); hErr != nil {
				return nil, nil, fmt.Errorf("%v, %v", err, hErr)
			}
		}
	}
	c.l.Debug("req #%d(%v): %v %v; status: %v", c.reqNum, tryN, method, u, resp.Status)

	// Load response body
	defer func() {
		_ = resp.Body.Close()
	}()
	if respBody, err = ioutil.ReadAll(resp.Body); err != nil {
		return resp, nil, fmt.Errorf("error while reading response body: %v", err)
	}

	if c.debug {
		c.DumpTransaction(req, resp, body, respBody)
	}

	// Check response status
	if resp.StatusCode >= 400 {
		return resp, respBody, fmt.Errorf("HTTP response status: %v", resp.Status)
	}

	return resp, respBody, err
}

// Get perform a GET request
func (c *Cli) Get(u string, args url.Values, header http.Header) ([]byte, error) {
	if args != nil {
		u = util.CombineURL(u, "", args)
	}

	_, body, err := c.Request("GET", u, header, []byte(""))
	return body, err
}

// GetQueryDoc performs a GET request and transform response into a goquery document
func (c *Cli) GetQueryDoc(u string, args url.Values, header http.Header) (*goquery.Document, error) {
	body, err := c.Get(u, args, header)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// GetJSON performs a GET HTTP request and parses the response into a JSON
func (c *Cli) GetJSON(u string, args url.Values, header http.Header, data interface{}) error {
	if header == nil {
		header = http.Header{}
	}
	if header.Get("X-Requested-With") == "" {
		header.Set("X-Requested-With", "XMLHttpRequest")
	}

	body, err := c.Get(u, args, header)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, data)
}

// GetFile gets a file and stores it on the disk.
//
// If fPath doesn't contain an extension, it will be added automatically.
// In case of success file extension returned
func (c *Cli) GetFile(u string, args url.Values, header http.Header, fPath string) (string, error) {
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
		cType := resp.Header.Get("Content-Type")
		cType = strings.ReplaceAll(cType, "/jpg", "/jpeg")

		fExtArr, err := mime.ExtensionsByType(cType)
		if err != nil || len(fExtArr) == 0 {
			return "", fmt.Errorf("unable to determine file extension for content type %q: %v", cType, err)
		}
		fExt = fExtArr[len(fExtArr)-1]
		fPath += fExt
	}

	// Write file to disk
	f, err := os.Create(fPath)
	if err != nil {
		return "", fmt.Errorf("error creating file %v: %v", fPath, err)
	}
	defer func() {
		_ = f.Close()
	}()

	if _, err := f.Write(body); err != nil {
		c.l.Err("failed to write to file: %s", err.Error())
	}

	return fExt, nil
}

// Post performs a POST request
func (c *Cli) Post(u string, args url.Values, header http.Header) ([]byte, error) {
	if header == nil {
		header = http.Header{}
	}
	if header.Get("Content-Type") == "" {
		header.Add("Content-Type", "application/x-www-form-urlencoded")
	}

	_, body, err := c.Request("POST", u, header, []byte(args.Encode()))
	return body, err
}

// GetExtIPAddrInfo returns information about client's external IP address
func (c *Cli) GetExtIPAddrInfo() (string, error) {
	var (
		b   []byte
		r   string
		err error
	)

	if b, err = c.Get("https://ifconfig.io/ip", nil, nil); err != nil {
		return r, err
	}
	r += fmt.Sprintf("address: %s", b)

	if b, err = c.Get("https://ifconfig.io/country_code", nil, nil); err != nil {
		return r, err
	}
	r = fmt.Sprintf("%v, region: %s", r, b)

	return strings.ReplaceAll(r, "\n", ""), nil
}

// New instantiates a client
func New(name string, dumpDir, ua, prxURL string, log *logger.Logger, onErr ErrorHandler) (*Cli, error) {
	var err error

	if log == nil {
		if log, err = logger.New(name, logger.LvInfo, ".", ""); err != nil {
			return nil, err
		}
	}

	debug := log.Level() >= logger.LvDebug
	sID := fmt.Sprintf("%d", time.Now().Unix())

	if debug {
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

	tr := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			if prxURL != "" {
				return url.Parse(prxURL)
			}
			return nil, nil
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	c := http.Client{
		Transport: tr,
		Timeout:   60 * time.Second,
	}

	c.Jar, err = cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, err
	}

	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
			"(KHTML, like Gecko) Chrome/86.0.4240.75 Safari/537.36"
	}

	cli := &Cli{
		cli:       &c,
		debug:     debug,
		dumpDir:   dumpDir,
		id:        sID,
		l:         log,
		userAgent: ua,
		reqErrH:   onErr,
		reqTries:  10,
	}

	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		cli.currentURL = req.URL
		return nil
	}

	return cli, nil
}
