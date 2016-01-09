package gocalm

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"reflect"
)

// ReadJSON unmarshals request body in JSON into v.
func ReadJSON(v interface{}, req *http.Request) {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		panic(HTTPError{
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
	_, err = w.Write(b)
	if err != nil {
		panic(err)
	}
}

// Write201 is a helper functon for POST to send back the absolute URL
// of the new resource.
func Write201(id string, w http.ResponseWriter, req *http.Request) {
	proto := req.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	u := fmt.Sprintf("%s://%s%s/%s", proto, req.Host, req.URL.String(), id)
	glog.Infof("Post Location: %s", u)
	w.Header().Set("Location", u)
	w.WriteHeader(http.StatusCreated)
	_, err := w.Write(nil)
	if err != nil {
		panic(err)
	}
}

// HTTPError fits interface `error` and can be handled by ErrorHandler to
// generate status code and error message.
type HTTPError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

func (t HTTPError) Error() string {
	return fmt.Sprintf("%d %s", t.StatusCode, t.Message)
}

// Similar to http.Error, except content is JSON
func Error(w http.ResponseWriter, error string, code int) {
	w.WriteHeader(code)
	WriteJSON(HTTPError{
		StatusCode: code,
		Message:    error,
	}, w)
}

func handleError(err error, w http.ResponseWriter, req *http.Request) {
	switch t := err.(type) {
	case HTTPError:
		Error(w, t.Message, t.StatusCode)
	case *HTTPError:
		Error(w, t.Message, t.StatusCode)
	default:
		Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ErrorHandler decorates http.Handler to catch and handle error.
func ErrorHandler(h http.Handler) http.Handler {
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
		h.ServeHTTP(w, req)
	}
	return http.HandlerFunc(wrapped)
}

// Set Content-Type to contentTypes...
func ResContentTypeHandler(h http.Handler, contentTypes ...string,
) http.Handler {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		header := w.Header()
		for _, t := range contentTypes {
			header.Add(CONTENT_TYPE, t)
		}
		h.ServeHTTP(w, req)
	}
	return http.HandlerFunc(wrapped)
}

// Mount tries to find http.HandlerFunc with specific names such as
// "GetAll", "Post", "Get", "Put", "Patch", "Delete" through the
// methods of a type, and mount them to a default set of paths. An
// optional map[string]string could be provided as method descriptions
// instead of the default ones. Router.SelfIntroHandlerFunc is mounted
// at "/_doc"
func Mount(r *Router, v reflect.Value, d map[string]string) {
	glog.V(1).Infof("Model: %s", v.Type().Name())
	r.SubPath("/_doc").Get("Document", r.SelfIntroHandlerFunc)
	idPath := r.SubPath("/{id}")
	getDesc := func(key, def string) string {
		if d == nil {
			return def
		}
		desc, ok := d[key]
		if !ok {
			return def
		}
		return desc
	}
	numMethod := v.NumMethod()
	for i := 0; i < numMethod; i++ {
		method := v.Type().Method(i)
		glog.V(1).Infof("%d %s", i, method.Name)
		f, ok := v.Method(i).Interface().(func(http.ResponseWriter, *http.Request))
		if !ok {
			glog.V(1).Infof("%s is not a http.HandlerFunc, skip.",
				method.Name)
			continue
		}
		switch method.Name {
		case "GetAll":
			desc := getDesc("GetAll", "Get a list of objects")
			glog.V(1).Infof("Mount %s to %s with description %s",
				method.Name, "GET /", desc)
			r.Get(desc, f)
		case "Post":
			desc := getDesc("Post", "Add an object")
			glog.V(1).Infof("Mount %s to %s with description %s",
				method.Name, "POST /", desc)
			r.Post(desc, f)
		case "Get":
			desc := getDesc("Get", "Get an object")
			glog.V(1).Infof("Mount %s to %s with description %s",
				method.Name, "GET /{id}", desc)
			idPath.Get(desc, f)
		case "Put":
			desc := getDesc("Put", "Replace an object")
			glog.V(1).Infof("Mount %s to %s with description %s",
				method.Name, "PUT /{id}", desc)
			idPath.Put(desc, f)
		case "Patch":
			desc := getDesc("Patch", "Patch an object")
			glog.V(1).Infof("Mount %s to %s with description %s",
				method.Name, "PATCH /{id}", desc)
			idPath.Patch(desc, f)
		case "Delete":
			desc := getDesc("Delete", "Delete an object")
			glog.V(1).Infof("Mount %s to %s with description %s",
				method.Name, "DELETE /{id}", desc)
			idPath.Delete(desc, f)
		default:
			glog.V(1).Infof("Skip %s", method.Name)
		}
	}
}
