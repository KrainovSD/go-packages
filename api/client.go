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

type RequestMethod string

const (
	METHOD_GET     RequestMethod = "GET"
	METHOD_POST                  = "POST"
	METHOD_DELETE                = "DELETE"
	METHOD_PUT                   = "PUT"
	METHOD_PATCH                 = "PATCH"
	METHOD_OPTIONS               = "OPTIONS"
)

type RequestContentType string

const (
	CONTENT_TYPE_JSON   RequestContentType = "application/json"
	CONTENT_TYPE_TEXT                      = "text/plain"
	CONTENT_TYPE_FORM                      = "application/x-www-form-urlencoded"
	CONTENT_TYPE_STREAM                    = "application/octet-stream"
)

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

func (c *Client) Close() {
	c.client.CloseIdleConnections()
}

func (c *Client) Send(request Request) (Response, error) {
	var err error
	var ctx = context.Background()
	if request.Ctx != nil {
		ctx = request.Ctx
	}
	if request.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	var requestUrl *url.URL
	if requestUrl, err = url.Parse(request.Url); err != nil {
		return Response{}, fmt.Errorf("parse url %s: %w", request.Url, err)
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
		return Response{}, fmt.Errorf("create request url %s: %w", request.Url, err)
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
		return Response{}, fmt.Errorf("do request url %s: %w", request.Url, err)
	}
	if request.Debug {
		fmt.Printf("request: %s, status: %d", requestUrl.String(), res.StatusCode)
	}
	defer res.Body.Close()
	var maxSize = request.MaxSize
	if maxSize == 0 {
		maxSize = 50 << 20
	}
	var content []byte
	if content, err = io.ReadAll(io.LimitReader(res.Body, maxSize)); err != nil {
		return Response{}, fmt.Errorf("read request url %s: %w", request.Url, err)
	}

	if res.StatusCode >= 400 {
		err = errors.New("bad status code " + strconv.Itoa(res.StatusCode) + ". Description:" + string(content))
		return Response{
				Data:   content,
				Status: res.StatusCode,
				Header: res.Header,
			},
			fmt.Errorf("bad status request url %s: %w", request.Url, err)
	}
	return Response{
		Data:   content,
		Status: res.StatusCode,
		Header: res.Header,
	}, nil

}
