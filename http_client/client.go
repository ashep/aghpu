package http_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ashep/aghpu/logger"
	"github.com/ashep/aghpu/util"

	"github.com/PuerkitoBio/goquery"
)

type Client struct {
	client    *http.Client
	debug     bool
	dumpDir   string
	id        string
	log       *logger.Logger
	reqNum    int
	userAgent string
}

// Dump an HTTP transaction content into a file
func (c *Client) DumpTransaction(req *http.Request, resp *http.Response, reqBody *[]byte, respBody *[]byte) {
	fPath := filepath.Join(c.dumpDir, fmt.Sprintf("%04d.txt", c.reqNum))
	f, err := os.Create(fPath)
	if err != nil {
		c.log.Err("error creating http dump file %v: %v", fPath, err)
		return
	}
	defer f.Close()

	// Request headers
	for k, h := range req.Header {
		for _, v := range h {
			if _, err := f.Write([]byte(fmt.Sprintf("%v: %v\n", k, v))); err != nil {
				c.log.Err("error writing http dump file %v: %v", fPath, err)
				return
			}
		}
	}
	f.Write([]byte("\n"))

	// Request body
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

	f.Write([]byte("\n"))

	// Request and response separator
	f.Write([]byte("\n---\n\n"))

	// Response headers
	for k, h := range resp.Header {
		for _, v := range h {
			if _, err := f.Write([]byte(fmt.Sprintf("%v: %v\n", k, v))); err != nil {
				c.log.Err("error writing http dump file %v: %v", fPath, err)
				return
			}
		}
	}
	f.Write([]byte("\n"))

	// Response body
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

// Perform an HTTP request
func (c *Client) Request(method, u string, header *http.Header, body []byte) (*http.Response, *[]byte, error) {
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
	c.reqNum += 1
	resp, err = c.client.Do(req)
	if err != nil {
		c.log.Debug("req #%d: %v %v; error: %v", c.reqNum, method, u, err)
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

// Perform a GET request
func (c *Client) Get(u string, args *url.Values, header *http.Header) (*[]byte, error) {
	if args != nil {
		u = util.CombineUrl(u, "", args)
	}

	_, body, err := c.Request("GET", u, header, []byte(""))
	return body, err
}

// Perform a GET request and transform response into a goquery document
func (c *Client) GetQueryDoc(u string, args *url.Values, header *http.Header) (*goquery.Document, error) {
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

func (c *Client) GetJson(u string, args *url.Values, header *http.Header, data interface{}) error {
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

// Get a file
func (c *Client) GetFile(u string, args *url.Values, header *http.Header, fPath string) error {
	body, err := c.Get(u, args, header)
	if err != nil {
		return err
	}

	f, err := os.Create(fPath)
	if err != nil {
		return fmt.Errorf("error creating file %v: %v", fPath, err)
	}

	f.Write(*body)
	f.Close()

	return nil
}

// Perform a POST request
func (c *Client) Post(u string, args *url.Values, header *http.Header) (*[]byte, error) {
	if header == nil {
		header = &http.Header{}
	}
	if header.Get("Content-Type") == "" {
		header.Add("Content-Type", "application/x-www-form-urlencoded")
	}

	_, body, err := c.Request("POST", u, header, []byte(args.Encode()))
	return body, err
}

// Instantiate a client
func New(name string, debug bool, dumpDir, userAgent string) (*Client, error) {
	var err error

	sId := fmt.Sprintf("%d", time.Now().Unix())
	log := logger.New(name, logger.LvInfo)

	if debug {
		log.Info("debug mode enabled")
		log.SetLevel(logger.LvDebug)

		// Calculate dump directory
		dumpDir, err = filepath.Abs(dumpDir)
		if err != nil {
			return nil, err
		}
		dumpDir = filepath.Join(dumpDir, sId)

		// Make dump directory
		err = os.MkdirAll(dumpDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("failed to create dump directory: %v\n", err)
		}
		log.Info("dump directory: %v\n", dumpDir)
	}

	c := http.Client{}

	c.Jar, err = cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, err
	}

	return &Client{
		client:    &c,
		debug:     debug,
		dumpDir:   dumpDir,
		id:        sId,
		log:       log,
		userAgent: userAgent,
	}, nil
}
