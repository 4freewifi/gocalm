// Copyright 2013 John Lee <john@0xlab.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		if reflect.ValueOf(v).Kind() != reflect.Chan {
			WriteJSON(v, w)
			return
		}
		c, ok := v.(chan interface{})
		if !ok {
			panic(errors.New(
				"type must be chan interface{}"))
		}
		w.Write([]byte{'['})
		i := 0
		for vv := range c {
			if err, ok := vv.(error); ok {
				panic(err)
			}
			if i != 0 {
				w.Write([]byte{','})
			}
			WriteJSON(vv, w)
			i++
		}
		w.Write([]byte{']'})
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
