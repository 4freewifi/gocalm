package gocalm

import (
	"github.com/gorilla/handlers"
	"log"
	"net/http"
)

type Book struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

type BooksModel struct {
	Shelf map[string]Book
}

func (t *BooksModel) GetAll(w http.ResponseWriter, req *http.Request) {
	books := make([]Book, 0, len(t.Shelf))
	for _, book := range t.Shelf {
		books = append(books, book)
	}
	WriteJSON(books, w)
}

func (t *BooksModel) Post(w http.ResponseWriter, req *http.Request) {
	book := Book{}
	ReadJSON(&book, req)
	if book.Title == "" {
		panic(Error{
			StatusCode: http.StatusBadRequest,
			Message:    "Missing title",
		})
	}
	_, ok := t.Shelf[book.Title]
	if ok {
		panic(Error{
			StatusCode: http.StatusConflict,
			Message:    "Already exists",
		})
	}
	t.Shelf[book.Title] = book
	Write201("http://example.com", book.Title, w, req)
}

func (t *BooksModel) Get(w http.ResponseWriter, req *http.Request) {
	var book Book
	title, ok := Vars(req)["id"]
	if !ok {
		goto NotFound
	}
	book, ok = t.Shelf[title]
	if !ok {
		goto NotFound
	}
	WriteJSON(book, w)
	return

NotFound:
	panic(Error{
		StatusCode: http.StatusNotFound,
		Message:    "Not found",
	})
}

func (t *BooksModel) Put(w http.ResponseWriter, req *http.Request) {
	book := Book{}
	title, ok := Vars(req)["id"]
	if !ok {
		goto NotFound
	}
	ReadJSON(&book, req)
	if book.Title == "" {
		panic(Error{
			StatusCode: http.StatusBadRequest,
			Message:    "Missing title",
		})
	}
	if book.Title != title {
		panic(Error{
			StatusCode: http.StatusBadRequest,
			Message:    "Titles mismatch",
		})
	}
	t.Shelf[book.Title] = book
	WriteJSON(book, w)
	return

NotFound:
	panic(Error{
		StatusCode: http.StatusNotFound,
		Message:    "Not found",
	})
}

func (t *BooksModel) Delete(w http.ResponseWriter, req *http.Request) {
	title, ok := Vars(req)["id"]
	if !ok {
		goto NotFound
	}
	delete(t.Shelf, title)
	w.WriteHeader(http.StatusNoContent)
	w.Write(nil)
	return

NotFound:
	panic(Error{
		StatusCode: http.StatusNotFound,
		Message:    "Not found",
	})
}

func Example() {
	// Add comment `Output:` at the end of the function then `go
	// test` as a hack to start this example as a web service.
	books := BooksModel{
		Shelf: make(map[string]Book),
	}
	handler := NewHandler()
	booksRouter := handler.Path("/books")
	booksRouter.
		Get("Get books", books.GetAll).
		Post("Add a book", books.Post)
	booksRouter.
		SubPath("/_doc").
		Get("Get document", booksRouter.SelfIntroHandlerFunc)
	booksRouter.
		SubPath("/{id}").
		Get("Get a book", books.Get).
		Put("Replace a book", books.Put).
		Delete("Delete a book", books.Delete)

	wrapped := handlers.ContentTypeHandler(
		handlers.CompressHandler(ErrorHandler(handler)),
		"application/json",
	)
	log.Fatal(http.ListenAndServe(":8080", wrapped))
}
