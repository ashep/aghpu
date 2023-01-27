package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"

	"eyeons.com/parser/aghpu/logger"
	"eyeons.com/parser/aghpu/util"
)

// Cli is a HTTP client
type Cli struct {
	mux           *sync.Mutex
	handlingError bool

	id        string
	dump      bool
	dumpDir   string
	reqTries  int
	userAgent string

	errorHandler ErrorHandler
	reqNum       int32

	cli *http.Client
	l   *logger.Logger
}

// ErrorHandler is HTTP request error handler
type ErrorHandler func(ctx context.Context, c *Cli, req *http.Request, rsp *http.Response, err error, tryN int) error

// New instantiates a client
func New(ctx context.Context, name string, dumpDir, ua, prxURL string, dump bool, log *logger.Logger) (*Cli, error) {
	var err error

	if log == nil {
		if log, err = logger.New(name, logger.LvInfo, ".", ""); err != nil {
			return nil, err
		}
	}

	sID := fmt.Sprintf("%d", time.Now().Unix())

	if dump {
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
		mux:       &sync.Mutex{},
		cli:       &c,
		dump:      dump,
		dumpDir:   dumpDir,
		id:        sID,
		l:         log,
		userAgent: ua,
		reqTries:  10,
	}

	return cli, nil
}

// Client returns underlying http client
func (c *Cli) Client() *http.Client {
	return c.cli
}

// SetErrorHandler sets HTTP request error handler
func (c *Cli) SetErrorHandler(fn ErrorHandler) {
	c.errorHandler = fn
}

// Reset resets the client
func (c *Cli) Reset() error {
	j, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	c.cli.Jar = j

	return nil
}

// DumpTransaction dumps an HTTP transaction content into a file
func (c *Cli) DumpTransaction(
	req *http.Request,
	resp *http.Response,
	reqBody, respBody []byte,
	tryNum int,
) {
	// Create a dump file
	fPath := filepath.Join(c.dumpDir, fmt.Sprintf("%04d-%02d.txt", c.reqNum, tryNum))
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

	// DoRequest and response separator
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

func (c *Cli) newRequest(ctx context.Context, method, u string, header http.Header, body []byte) (*http.Request, error) {
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

	req, err := http.NewRequestWithContext(ctx, method, u, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header = header

	return req, nil
}

// DoRequest performs an HTTP request
func (c *Cli) DoRequest(
	ctx context.Context,
	method,
	u string,
	header http.Header,
	body []byte,
) (*http.Response, []byte, error) {
	var (
		err     error
		req     *http.Request
		rsp     *http.Response
		rspBody []byte
	)

	reqNum := c.reqNum
	tryNum := 1
	for ; ; tryNum++ {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
			// While handling error, it's allowed to work only to error handler, others must wait
			if c.handlingError {
				if _, ok := ctx.Value("errorHandler").(bool); !ok {
					c.l.Debug("waiting for client readiness")
					time.Sleep(time.Second)
					continue
				}
			}
		}

		reqNum = atomic.AddInt32(&c.reqNum, 1)

		req, err = c.newRequest(ctx, method, u, header.Clone(), body)
		if err != nil {
			return nil, nil, err
		}

		rsp, err = c.cli.Do(req)
		if err == nil && rsp.StatusCode > 199 && rsp.StatusCode < 300 {
			break
		} else if err == nil {
			err = errors.New(rsp.Status)
		}
		c.l.Err("req #%d(%v): %v %v; error: %v", reqNum, tryNum, method, u, err)

		if rsp != nil {
			if rb, re := ioutil.ReadAll(rsp.Body); re == nil && c.dump {
				c.DumpTransaction(req, rsp, body, rb, tryNum)
			}
			_ = rsp.Body.Close()
		}

		if c.errorHandler != nil {
			if c.handlingError {
				return nil, nil, fmt.Errorf("error is already being handled by another goroutine")
			}

			c.mux.Lock()
			c.handlingError = true
			hErr := c.errorHandler(context.WithValue(ctx, "errorHandler", true), c, req, rsp, err, tryNum)
			c.handlingError = false
			c.mux.Unlock()
			if hErr != nil {
				return nil, nil, fmt.Errorf("%v, %v", err, hErr)
			}
		}

		if tryNum == c.reqTries || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, nil, err
		}

		time.Sleep(time.Second * time.Duration(tryNum))
	}

	c.l.Debug("req #%d(%v): %v %v; status: %v", reqNum, tryNum, method, u, rsp.Status)

	defer func() {
		_ = rsp.Body.Close()
	}()
	if rspBody, err = ioutil.ReadAll(rsp.Body); err != nil {
		return rsp, nil, fmt.Errorf("error while reading response body: %v", err)
	}

	if c.dump {
		c.DumpTransaction(req, rsp, body, rspBody, tryNum)
	}

	// Check response status
	if rsp.StatusCode >= 400 {
		return rsp, rspBody, fmt.Errorf("HTTP response status: %v", rsp.Status)
	}

	return rsp, rspBody, err
}

// Get perform a GET request
func (c *Cli) Get(ctx context.Context, u string, args url.Values, header http.Header) ([]byte, error) {
	if args != nil {
		u = util.CombineURL(u, "", args)
	}

	_, body, err := c.DoRequest(ctx, "GET", u, header, []byte(""))
	return body, err
}

// GetQueryDoc performs a GET request and transform response into a goquery document
func (c *Cli) GetQueryDoc(ctx context.Context, u string, args url.Values, header http.Header) (*goquery.Document, error) {
	body, err := c.Get(ctx, u, args, header)
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
func (c *Cli) GetJSON(ctx context.Context, u string, args url.Values, header http.Header, target interface{}) error {
	if header == nil {
		header = http.Header{}
	}
	if header.Get("X-Requested-With") == "" {
		header.Set("X-Requested-With", "XMLHttpRequest")
	}

	body, err := c.Get(ctx, u, args, header)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

// GetFile gets a file and stores it on the disk.
//
// If fPath doesn't contain an extension, it will be added automatically.
// In case of success file extension returned
func (c *Cli) GetFile(ctx context.Context, u string, args url.Values, header http.Header, fPath string) (string, error) {
	fExt := ""

	if args != nil {
		u = util.CombineURL(u, "", args)
	}

	resp, body, err := c.DoRequest(ctx, "GET", u, header, nil)
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
func (c *Cli) Post(ctx context.Context, u string, header http.Header, body []byte) ([]byte, error) {
	if header == nil {
		header = http.Header{}
	}

	_, rBody, err := c.DoRequest(ctx, "POST", u, header, body)
	return rBody, err
}

// PostForm posts a form
func (c *Cli) PostForm(ctx context.Context, u string, args url.Values, header http.Header) ([]byte, error) {
	if header == nil {
		header = http.Header{}
	}

	header.Add("Content-Type", "application/x-www-form-urlencoded")

	return c.Post(ctx, u, header, []byte(args.Encode()))
}

// PostJSON posts a JSON request
func (c *Cli) PostJSON(ctx context.Context, u string, header http.Header, data interface{}) ([]byte, error) {
	if header == nil {
		header = http.Header{}
	}

	header.Add("Content-Type", "application/json")

	dataB, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return c.Post(ctx, u, header, dataB)
}

// PostFormParseJSON performs a POST request and parses JSON response
func (c *Cli) PostFormParseJSON(ctx context.Context, u string, args url.Values, header http.Header, target interface{}) error {
	if header == nil {
		header = http.Header{}
	}

	resp, err := c.PostForm(ctx, u, args, header)
	if err != nil {
		return err
	}

	return json.Unmarshal(resp, target)
}

// PostJSONParseJSON performs a POST request having JSON body and parses JSON response
func (c *Cli) PostJSONParseJSON(ctx context.Context, u string, data interface{}, header http.Header, target interface{}) error {
	if header == nil {
		header = http.Header{}
	}

	if header.Get("Content-Type") == "" {
		header.Add("Content-Type", "application/json")
	}

	resp, err := c.PostJSON(ctx, u, header, data)
	if err != nil {
		return err
	}

	return json.Unmarshal(resp, target)
}

// GetExtIPAddrInfo returns information about client's external IP address
func (c *Cli) GetExtIPAddrInfo(ctx context.Context) (string, error) {
	var (
		b   []byte
		r   string
		err error
	)

	if b, err = c.Get(ctx, "https://ifconfig.io/ip", nil, nil); err != nil {
		return r, err
	}
	r += fmt.Sprintf("address: %s", b)

	if b, err = c.Get(ctx, "https://ifconfig.io/country_code", nil, nil); err != nil {
		return r, err
	}
	r = fmt.Sprintf("%v, region: %s", r, b)

	return strings.ReplaceAll(r, "\n", ""), nil
}
