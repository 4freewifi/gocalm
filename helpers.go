package gocalm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// ReadJSON unmarshals request body in JSON into v.
func ReadJSON(v interface{}, req *http.Request) {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		panic(Error{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
	}
}

// WriteJSON marshals v then write to w.
func WriteJSON(v interface{}, w http.ResponseWriter) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	w.Write(b)
}

// Write201 is a helper functon for POST to send back the absolute URI
// of the new resource. `host` must be something like
// http://example.com , with no trailing slash.
func Write201(host, id string, w http.ResponseWriter, req *http.Request) {
	s := fmt.Sprintf("%s%s/%s", host, req.URL.String(), id)
	absoluteURI, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Location", absoluteURI.String())
	w.WriteHeader(http.StatusCreated)
	w.Write(nil)
}

// Error fits interface `error` and can be handled by ErrorHandler to
// generate status code and error message.
type Error struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

func (t Error) Error() string {
	return fmt.Sprintf("%d %s", t.StatusCode, t.Message)
}

func handleError(err error, w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch t := err.(type) {
	case Error:
		w.WriteHeader(t.StatusCode)
		WriteJSON(t, w)
	case *Error:
		w.WriteHeader(t.StatusCode)
		WriteJSON(t, w)
	default:
		e := Error{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		}
		w.WriteHeader(e.StatusCode)
		WriteJSON(e, w)
	}
}

// ErrorHandler decorates http.Handler to catch and handle error.
func ErrorHandler(handler http.Handler) http.Handler {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			err, ok := r.(error)
			if ok {
				handleError(err, w, req)
				return
			}
			panic(r)
		}()
		handler.ServeHTTP(w, req)
	}
	return http.HandlerFunc(wrapped)
}
