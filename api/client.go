package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type RequestMethod string

const (
	MethodGet     RequestMethod = http.MethodGet
	MethodPost    RequestMethod = http.MethodPost
	MethodDelete  RequestMethod = http.MethodDelete
	MethodPut     RequestMethod = http.MethodPut
	MethodPatch   RequestMethod = http.MethodPatch
	MethodOptions RequestMethod = http.MethodOptions
)

type RequestContentType string

const (
	ContentTypeJSON   RequestContentType = "application/json"
	ContentTypeText   RequestContentType = "text/plain"
	ContentTypeForm   RequestContentType = "application/x-www-form-urlencoded"
	ContentTypeStream RequestContentType = "application/octet-stream"
)

type Client struct {
	client *http.Client
}

type ClientOptions struct {
	Tracing             bool
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	NoRedirect          bool
}

func NewClient(opts *ClientOptions) (*Client, error) {
	var maxIdleConns = opts.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 100
	}
	var maxIdleConnsPerHost = opts.MaxIdleConnsPerHost
	if maxIdleConnsPerHost == 0 {
		maxIdleConnsPerHost = 20
	}
	var transport = &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		IdleConnTimeout:     90 * time.Second,
	}
	var client = http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
	}
	if opts.NoRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	if opts.Tracing {
		client.Transport = otelhttp.NewTransport(transport)
	}
	return &Client{
		client: &client,
	}, nil
}

func (c *Client) Client() *http.Client {
	return c.client
}

func (c *Client) Close() {
	c.client.CloseIdleConnections()
}

type Request struct {
	Url           string
	Queries       map[string][]string
	Headers       map[string]string
	Method        RequestMethod
	ContentType   RequestContentType
	ContentLength int64
	Body          io.Reader
	Ctx           context.Context
	Timeout       time.Duration
	Debug         bool
}

type Response struct {
	*http.Response
	cancel context.CancelFunc
}

func (c *Client) Send(request Request) (*Response, error) {
	var err error
	var ctx = context.Background()
	if request.Ctx != nil {
		ctx = request.Ctx
	}
	var cancel context.CancelFunc
	if request.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
	}
	var requestUrl *url.URL
	if requestUrl, err = url.Parse(request.Url); err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("parse url %s: %w", request.Url, err)
	}
	if len(request.Queries) > 0 {
		var queries = requestUrl.Query()
		for k, vArr := range request.Queries {
			for _, v := range vArr {
				queries.Add(k, v)
			}
		}
		requestUrl.RawQuery = queries.Encode()
	}
	var req *http.Request
	if req, err = http.NewRequestWithContext(ctx, string(request.Method), requestUrl.String(), request.Body); err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("create request host %s, path %s: %w", requestUrl.Host, requestUrl.Path, err)
	}
	if request.ContentLength != 0 {
		req.ContentLength = request.ContentLength
	}
	if len(request.Headers) > 0 {
		for k, v := range request.Headers {
			req.Header.Add(k, v)
		}
	}
	if request.ContentType != "" {
		req.Header.Add("Content-Type", string(request.ContentType))
	}
	var res *http.Response
	if res, err = c.client.Do(req); err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, fmt.Errorf("do request host %s, path %s: %w", requestUrl.Host, requestUrl.Path, err)
	}
	if request.Debug {
		fmt.Printf("request: %s, status: %d", requestUrl.String(), res.StatusCode)
	}
	return &Response{Response: res, cancel: cancel}, nil

}

func (r *Response) Read(maxSize int64) ([]byte, error) {
	var content []byte
	var err error
	var reader io.ReadCloser = r.Body
	if maxSize > 0 {
		reader = http.MaxBytesReader(nil, reader, maxSize)
	}
	if content, err = io.ReadAll(reader); err != nil {
		return nil, fmt.Errorf("read request host %s, path %s: %w", r.Request.URL.Host, r.Request.URL.Path, err)
	}
	return content, nil
}

func (r *Response) Close() {
	r.Body.Close()
	if r.cancel != nil {
		r.cancel()
	}
}

type RequestWithRead struct {
	Request
	IsBadStatus func(status int) bool
	MaxSize     int64
}

type ResponseWithRead struct {
	*http.Response
	Data []byte
}

var ErrBadStatusCode = errors.New("bad status code")

func (r *Client) SendWithRead(req RequestWithRead) (*ResponseWithRead, error) {
	var err error
	var res *Response
	if res, err = r.Send(req.Request); err != nil {
		return nil, err
	}
	defer res.Close()
	var data []byte
	if data, err = res.Read(req.MaxSize); err != nil {
		return nil, err
	}
	if (req.IsBadStatus == nil && res.StatusCode >= 400) || (req.IsBadStatus != nil && req.IsBadStatus(res.StatusCode)) {
		return nil, fmt.Errorf("%w: %d, host %s, path %s", ErrBadStatusCode, res.StatusCode, res.Request.URL.Host, res.Request.URL.Path)
	}
	return &ResponseWithRead{
		Response: res.Response,
		Data:     data,
	}, nil

}
