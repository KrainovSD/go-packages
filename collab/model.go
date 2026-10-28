package collab

import (
	"context"
	"net/http"
)

type StateStore interface {
	Read(name string) []byte
	Write(name string, content []byte)
	Delete(name string)
}

type DocumentService interface {
	ExtractName(URL string) (string, bool)
	Get(ctx context.Context, name string, r *http.Request) (string, error)
	Auth(ctx context.Context, name string, r *http.Request) error
	Save(ctx context.Context, name string, content string) error
}

type OnOpenRoom func(name string)
type OnCloseRoom func(name string)
type OnOpenConnection func(name string, r *http.Request)
type OnCloseConnection func(name string, r *http.Request)

const (
	messageSync           uint64 = 0
	messageAwareness      uint64 = 1
	messageAuth           uint64 = 2
	messageQueryAwareness uint64 = 3
)

const textName = "codemirror"
