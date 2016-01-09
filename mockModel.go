package gocalm

import (
	"errors"
	"net/http"
)

const (
	ID = "id"
)

type JSONObject map[string]interface{}

// A general mocker that keeps everything with a "id" field.
type MockModel map[string]JSONObject

func (t MockModel) GetAll(w http.ResponseWriter, req *http.Request) {
	list := make([]JSONObject, 0, len(t))
	for id, obj := range t {
		obj[ID] = id
		list = append(list, obj)
	}
	WriteJSON(list, w)
}

func (t MockModel) Post(w http.ResponseWriter, req *http.Request) {
	var obj JSONObject
	ReadJSON(&obj, req)
	v, ok := obj[ID]
	if !ok {
		panic(HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    `Missing field "id"`,
		})
	}
	id, ok := v.(string)
	if !ok {
		panic(HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    `Field "id" must be of type "string".`,
		})
	}
	_, ok = t[id]
	if ok {
		panic(HTTPError{
			StatusCode: http.StatusConflict,
			Message:    "Already exists",
		})
	}
	t[id] = obj
	Write201(id, w, req)
}

func getID(req *http.Request) (id string) {
	id, ok := Vars(req)[ID]
	if !ok {
		panic(errors.New(`Must have variable "` + ID + `" in path`))
	}
	return
}

func (t MockModel) Get(w http.ResponseWriter, req *http.Request) {
	obj, ok := t[getID(req)]
	if !ok {
		panic(HTTPError{
			StatusCode: http.StatusNotFound,
			Message:    "Not found",
		})
	}
	WriteJSON(obj, w)
	return
}

func (t MockModel) Put(w http.ResponseWriter, req *http.Request) {
	id := getID(req)
	_, ok := t[id]
	if !ok {
		panic(HTTPError{
			StatusCode: http.StatusNotFound,
			Message:    "Not found",
		})
	}
	var obj JSONObject
	ReadJSON(&obj, req)
	obj[ID] = id
	t[id] = obj
	WriteJSON(obj, w)
}

func (t MockModel) Delete(w http.ResponseWriter, req *http.Request) {
	id := getID(req)
	_, ok := t[id]
	if !ok {
		panic(HTTPError{
			StatusCode: http.StatusNotFound,
			Message:    "Not found",
		})
	}
	delete(t, id)
	w.WriteHeader(http.StatusNoContent)
	_, err := w.Write(nil)
	if err != nil {
		panic(err)
	}
}
