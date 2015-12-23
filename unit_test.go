package gocalm

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
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
	router.Get("Get a list of stuff", model.GetAll).
		Post("Add stuff", model.Post)
	router.SubPath("/_doc").
		Get("Read document", router.SelfIntroHandlerFunc)
	router.SubPath("/{id}").
		Get("Get stuff", model.Get).
		Put("Replace stuff", model.Put).
		Delete("Remove stuff", model.Delete)

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
			"http://example.com/stuff/1",
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
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
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
