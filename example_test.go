package gocalm

import (
	"github.com/gorilla/handlers"
	"log"
	"net/http"
)

func Example() {
	// Add comment `Output:` at the end of the function then `go
	// test` as a hack to start this example as a web service.
	model := MockModel(make(map[string]JSONObject))
	handler := NewHandler()
	router := handler.Path("/stuff")
	router.Get("Get a list of stuff", model.GetAll).
		Post("Add stuff", model.Post)
	router.SubPath("/_doc").
		Get("Read document", router.SelfIntroHandlerFunc)
	router.SubPath("/{id}").
		Get("Get stuff", model.Get).
		Put("Replace stuff", model.Put).
		Delete("Remove stuff", model.Delete)
	wrapped := ResContentTypeHandler(handler, JSON_TYPE)
	wrapped = ErrorHandler(wrapped)
	wrapped = handlers.CompressHandler(wrapped)
	wrapped = handlers.ContentTypeHandler(wrapped, JSON_TYPE)
	log.Fatal(http.ListenAndServe(":8080", wrapped))
}
