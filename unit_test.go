package gocalm

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

var book1 = []byte(`{
  "title": "The Adventures of Tom Sawyer",
  "author": "Mark Twain"
}`)

var book2 = []byte(`{
  "title": "The Adventures of Tom Sawyer",
  "author": "Ghost Writer"
}`)

func TestRouter(t *testing.T) {

	handler := NewHandler()

	books := &BooksModel{
		Shelf: make(map[string]Book),
	}

	booksRouter := handler.Path("/books")
	booksRouter.
		Get("Get books", books.GetAll).
		Post("Add book", books.Post)
	booksRouter.
		SubPath("/_doc").
		Get("Get document", booksRouter.SelfIntroHandlerFunc)
	booksRouter.
		SubPath("/{id}").
		Get("Get book", books.Get).
		Put("Replace book", books.Put).
		Delete("Delete book", books.Delete)

	type testcase struct {
		Path   string
		Method string
		Input  []byte
		Output string
	}
	tests := []testcase{
		{
			"/books/_doc",
			"GET",
			nil,
			`[
  {
    "path": "/books",
    "methods": [
      {
        "method": "OPTIONS",
        "description": "Get available methods"
      },
      {
        "method": "GET",
        "description": "Get books"
      },
      {
        "method": "POST",
        "description": "Add book"
      }
    ]
  },
  {
    "path": "/books/_doc",
    "methods": [
      {
        "method": "OPTIONS",
        "description": "Get available methods"
      },
      {
        "method": "GET",
        "description": "Get document"
      }
    ]
  },
  {
    "path": "/books/{id}",
    "methods": [
      {
        "method": "OPTIONS",
        "description": "Get available methods"
      },
      {
        "method": "GET",
        "description": "Get book"
      },
      {
        "method": "PUT",
        "description": "Replace book"
      },
      {
        "method": "DELETE",
        "description": "Delete book"
      }
    ]
  }
]`,
		}, {
			"/books",
			"GET",
			nil,
			"[]",
		}, {
			"/books",
			"POST",
			book1,
			"http://example.com/books/The%20Adventures%20of%20Tom%20Sawyer",
		}, {
			"/books",
			"GET",
			nil,
			`[
  {
    "title": "The Adventures of Tom Sawyer",
    "author": "Mark Twain"
  }
]`,
		}, {
			"/books/The%20Adventures%20of%20Tom%20Sawyer",
			"GET",
			nil,
			string(book1),
		}, {
			"/books/The%20Adventures%20of%20Tom%20Sawyer",
			"PUT",
			book2,
			string(book2),
		}, {
			"/books/The%20Adventures%20of%20Tom%20Sawyer",
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
	err := Error{
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
