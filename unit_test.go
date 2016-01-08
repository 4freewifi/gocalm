package gocalm

import (
	"bytes"
	"github.com/gorilla/handlers"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const (
	HOST = "foo.bar"
)

var book1 = []byte(`{
  "author": "Mark Twain",
  "id": "1",
  "title": "The Adventures of Tom Sawyer"
}`)

var book2 = []byte(`{
  "author": "Ghost Writer",
  "id": "1",
  "title": "The Adventures of Tom Sawyer"
}`)

func TestRouter(t *testing.T) {

	model := MockModel(make(map[string]JSONObject))
	handler := NewHandler()
	router := handler.Path("/stuff")
	Mount(router, reflect.ValueOf(model), nil)
	wrapped := ResContentTypeHandler(handler, JSON_TYPE)
	wrapped = handlers.ContentTypeHandler(wrapped, JSON_TYPE)

	type testcase struct {
		Path   string
		Method string
		Input  []byte
		Output string
	}
	tests := []testcase{
		{
			"/stuff",
			"GET",
			nil,
			"[]",
		}, {
			"/stuff",
			"POST",
			book1,
			"http://" + HOST + "/stuff/1",
		}, {
			"/stuff",
			"GET",
			nil,
			`[
  {
    "author": "Mark Twain",
    "id": "1",
    "title": "The Adventures of Tom Sawyer"
  }
]`,
		}, {
			"/stuff/1",
			"GET",
			nil,
			string(book1),
		}, {
			"/stuff/1",
			"PUT",
			book2,
			string(book2),
		}, {
			"/stuff/1",
			"DELETE",
			nil,
			"",
		},
	}

	for _, test := range tests {
		input := bytes.NewBuffer(test.Input)
		req, err := http.NewRequest(test.Method, test.Path, input)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = HOST
		t.Logf("Testing %s %s", test.Method, test.Path)
		req.Header.Set(CONTENT_TYPE, JSON_TYPE)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
		t.Logf("Response: %d %s", w.Code, w.Body.String())
		typ := w.HeaderMap.Get(CONTENT_TYPE)
		if typ != JSON_TYPE {
			t.Fatalf("Expect Content-Type: %s, Got: %s", JSON_TYPE,
				typ)
		}
		var output string
		if test.Method == "POST" {
			output = w.HeaderMap.Get("Location")
		} else {
			output = w.Body.String()
		}
		if test.Output != output {
			t.Fatalf("Expect: %s, Got: %s", test.Output, output)
		}
	}
}

func causeError(w http.ResponseWriter, req *http.Request) {
	err := HTTPError{
		StatusCode: 400,
		Message:    "Test error",
	}
	panic(err)
}

func TestError(t *testing.T) {
	handler := http.HandlerFunc(causeError)
	ts := httptest.NewServer(ErrorHandler(handler))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	msg, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d %s", res.StatusCode, msg)
}
