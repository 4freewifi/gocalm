package gocalm

import (
	"github.com/gorilla/handlers"
	"github.com/johncylee/goutil"
	"log"
	"net/http"
	"reflect"
)

func Example() {
	// Add comment `Output:` at the end of the function then `go
	// test` as a hack to start this example as a web service.
	model := MockModel(make(map[string]JSONObject))
	handler := NewHandler()
	router := handler.Path("/stuff")
	Mount(router, reflect.ValueOf(model), nil)
	wrapped := goutil.HeadHandler(handler)
	wrapped = ResContentTypeHandler(wrapped, JSON_TYPE)
	wrapped = ErrorHandler(wrapped)
	wrapped = handlers.CompressHandler(wrapped)
	wrapped = handlers.ContentTypeHandler(wrapped, JSON_TYPE)
	log.Fatal(http.ListenAndServe(":8080", wrapped))
}
