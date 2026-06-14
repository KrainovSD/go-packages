package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type RequestMethod int

const (
	METHOD_GET RequestMethod = iota
	METHOD_POST
	METHOD_DELETE
	METHOD_PUT
	METHOD_PATCH
	METHOD_OPTIONS
)

func (m RequestMethod) String() string {
	switch m {
	case METHOD_GET:
		return "GET"
	case METHOD_POST:
		return "POST"
	case METHOD_DELETE:
		return "DELETE"
	case METHOD_PUT:
		return "PUT"
	case METHOD_PATCH:
		return "PATCH"
	case METHOD_OPTIONS:
		return "OPTIONS"
	default:
		return "UNKNOWN"
	}
}

type RequestContentType int

const (
	CONTENT_TYPE_JSON RequestContentType = iota
	CONTENT_TYPE_TEXT
	CONTENT_TYPE_FORM
	CONTENT_TYPE_STREAM
)

func (m RequestContentType) String() string {
	switch m {
	case CONTENT_TYPE_JSON:
		return "application/json"
	case CONTENT_TYPE_TEXT:
		return "text/plain"
	case CONTENT_TYPE_FORM:
		return "application/x-www-form-urlencoded"
	case CONTENT_TYPE_STREAM:
		return "application/octet-stream"

	default:
		return "UNKNOWN"
	}
}

type Client struct {
	client *http.Client
}
type Request struct {
	Url         string
	Queries     map[string][]string
	Headers     map[string]string
	Method      RequestMethod
	ContentType RequestContentType
	Body        io.Reader
	Ctx         context.Context
	Timeout     time.Duration
	MaxSize     int64
	Debug       bool
}
type Response struct {
	Data   []byte
	Status int
	Header http.Header
}

type ClientOptions struct {
	Tracing             bool
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
}

func CreateClient(opts ClientOptions) (*Client, error) {
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
	if opts.Tracing {
		client.Transport = otelhttp.NewTransport(transport)
	}
	return &Client{
		client: &client,
	}, nil
}

func (c *Client) Send(request Request) (Response, error) {
	var result Response
	var content []byte
	var err error
	var req *http.Request
	var res *http.Response
	var requestUrl *url.URL
	var ctx = context.Background()
	if request.Ctx != nil {
		ctx = request.Ctx
	}
	if request.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	if requestUrl, err = url.Parse(request.Url); err != nil {
		return result, fmt.Errorf("parse url %s: %w", request.Url, err)
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
	if request.Debug {
		fmt.Println(requestUrl.String())
	}
	if req, err = http.NewRequestWithContext(ctx, request.Method.String(), requestUrl.String(), request.Body); err != nil {
		return result, fmt.Errorf("create request url %s: %w", request.Url, err)
	}
	if len(request.Headers) > 0 {
		for k, v := range request.Headers {
			req.Header.Add(k, v)
		}
	}
	req.Header.Add("Content-Type", request.ContentType.String())

	if res, err = c.client.Do(req); err != nil {
		return result, fmt.Errorf("do request url %s: %w", request.Url, err)
	}
	defer res.Body.Close()
	var maxSize = request.MaxSize
	if maxSize == 0 {
		maxSize = 50 << 20
	}
	if content, err = io.ReadAll(io.LimitReader(res.Body, maxSize)); err != nil {
		return result, fmt.Errorf("read request url %s: %w", request.Url, err)
	}

	if res.StatusCode >= 400 {
		err = errors.New("bad status code " + strconv.Itoa(res.StatusCode) + ". Description:" + string(content))
		return result, fmt.Errorf("bad status request url %s: %w", request.Url, err)
	}

	return Response{
		Data:   content,
		Status: res.StatusCode,
		Header: res.Header,
	}, nil

}
