package gocalm

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
)

var NotFound error = errors.New("Not found")
var TypeMismatch error = errors.New("Type mismatch")

type ModelInterface interface {
	Get(id string) (v interface{}, err error)
	GetAll() (v interface{}, err error)
	Put(id string, v interface{}) (err error)
	PutAll(v interface{}) (err error)
	Post(v interface{}) (err error)
	Delete(id string) (err error)
	DeleteAll() (err error)
}

type RESTHandler struct {
	Model          ModelInterface
	DataType       reflect.Type
	pathParameters map[string]string
}

func (h *RESTHandler) SetPathParameters(nvpairs map[string]string) {
	h.pathParameters = nvpairs
}

func (h *RESTHandler) NotFound(
	err error, w http.ResponseWriter, r *http.Request) {
	log.Println(err)
	http.Error(w, err.Error(), http.StatusNotFound)
}

func (h *RESTHandler) BadRequest(
	err error, w http.ResponseWriter, r *http.Request) {
	log.Println(err)
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func (h *RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Panicln(err)
			http.Error(w, fmt.Sprint(err),
				http.StatusInternalServerError)
		}
	}()
	id := h.pathParameters["id"]
	switch {
	case r.Method == "GET" && id != "":
		v, err := h.Model.Get(id)
		if err != nil {
			h.NotFound(err, w, r)
			return
		}
		WriteJSON(v, w)
	case r.Method == "GET" && id == "":
		v, err := h.Model.GetAll()
		if err != nil {
			h.NotFound(err, w, r)
			return
		}
		WriteJSON(v, w)
	case r.Method == "PUT" && id != "":
		v := reflect.New(h.DataType).Interface()
		err := ReadJSON(v, r)
		if err != nil {
			h.BadRequest(err, w, r)
			return
		}
		err = h.Model.Put(id, v)
		if err != nil {
			h.BadRequest(err, w, r)
			return
		}
		fmt.Fprint(w, "OK")
	case r.Method == "PUT" && id == "":
		// TODO: do not implement this until we have reflect.SliceOf
		panic(fmt.Errorf("Not implemented"))
	case r.Method == "POST" && id == "":
		v := reflect.New(h.DataType).Interface()
		err := ReadJSON(v, r)
		if err != nil {
			h.BadRequest(err, w, r)
			return
		}
		err = h.Model.Post(v)
		if err != nil {
			h.BadRequest(err, w, r)
			return
		}
		fmt.Fprint(w, "OK")
	case r.Method == "DELETE" && id != "":
		err := h.Model.Delete(id)
		if err != nil {
			h.NotFound(err, w, r)
			return
		}
		fmt.Fprint(w, "OK")
	case r.Method == "DELETE" && id == "":
		err := h.Model.DeleteAll()
		if err != nil {
			panic(err)
		}
		fmt.Fprint(w, "OK")
	default:
		err := fmt.Errorf("Unsupported request method: %s", r.Method)
		h.BadRequest(err, w, r)
		return
	}
	return
}
