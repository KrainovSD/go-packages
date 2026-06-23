package web

import (
	"encoding/json"
	"errors"
	"net/http"
)

type ErrorResponse struct {
	Message string
	Code    int
	Status  int
	Error   error
}

func SendError(w http.ResponseWriter, res ErrorResponse) {
	if res.Error != nil {
		if writer, ok := w.(*ResponseWriter); ok {
			writer.SetError(res.Error)
		}
	}
	var status = 500
	if res.Status != 0 {
		status = res.Status
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Message: res.Message,
		Code:    res.Code,
		Status:  status,
	})

}

func NotAuthorized(w http.ResponseWriter) {
	SendError(w, ErrorResponse{
		Message: "not authorized",
		Status:  401,
		Error:   ErrorNotAuthorized,
	})
}

func Forbidden(w http.ResponseWriter) {
	SendError(w, ErrorResponse{
		Message: "forbidden",
		Status:  403,
		Error:   ErrorForbidden,
	})
}

func BadRequest(w http.ResponseWriter) {
	SendError(w, ErrorResponse{
		Message: "bad request",
		Status:  400,
		Error:   ErrorBadRequest,
	})
}

func Conflict(w http.ResponseWriter) {
	SendError(w, ErrorResponse{
		Message: "conflict",
		Status:  409,
		Error:   ErrorConflict,
	})
}

func InternalServerError(w http.ResponseWriter) {
	SendError(w, ErrorResponse{
		Message: "internal server error",
		Status:  500,
		Error:   ErrorInternalServerError,
	})
}

var ErrorRequestTooLarge = errors.New("request too large")
var ErrorForbidden = errors.New("forbidden")
var ErrorNotAuthorized = errors.New("not authorized")
var ErrorNotFound = errors.New("not found")
var ErrorConflict = errors.New("conflict")
var ErrorBadRequest = errors.New("bad request")
var ErrorInternalServerError = errors.New("internal server error")
