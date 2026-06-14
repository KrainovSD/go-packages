package storage

import (
	"context"
	"fmt"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type CreateMinioOptions struct {
	Url          string
	AccessKey    string
	SecretKey    string
	Secure       bool
	Location     string
	Buckets      []string
	CreateBucket bool
	Tracing      bool
}

func CreateMinio(ctx context.Context, opts *CreateMinioOptions) (*minio.Client, error) {
	var creds *credentials.Credentials
	if opts.AccessKey != "" && opts.SecretKey != "" {
		creds = credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, "")
	}
	var minioOpts = &minio.Options{
		Creds:  creds,
		Secure: opts.Secure,
	}
	if opts.Tracing {
		minioOpts.Transport = otelhttp.NewTransport(http.DefaultTransport)
	}
	var err error
	var client *minio.Client
	if client, err = minio.New(opts.Url, minioOpts); err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	for _, bucket := range opts.Buckets {
		var exists bool
		if exists, err = client.BucketExists(ctx, bucket); err != nil {
			return nil, fmt.Errorf("check bucket exists: %w", err)
		}
		if exists || !opts.CreateBucket {
			continue
		}
		if err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{
			Region: opts.Location,
		}); err != nil {
			return nil, fmt.Errorf("create bucket %s: %w", bucket, err)
		}
	}
	return client, nil
}
