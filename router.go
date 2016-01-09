package gocalm

import (
	"encoding/json"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
)

// Vars wraps around gorilla/mux.Vars . Use it to get variable value
// defined inside template string.
func Vars(req *http.Request) map[string]string {
	return mux.Vars(req)
}

// Handler fits the http.Handler interface
type Handler mux.Router

// Router handles different request methods under the same path
// template.
type Router struct {
	root     *mux.Router
	path     string
	router   *mux.Router
	methods  map[string]string
	children []*Router
}

// NewHandler returns a new Handler
func NewHandler() *Handler {
	return (*Handler)(mux.NewRouter())
}

// Path returns a pointer to a new Router to handle requests whose
// path matches the given template string.
func (t *Handler) Path(tpl string) *Router {
	muxRouter := (*mux.Router)(t)
	router := &Router{
		root:     muxRouter,
		path:     tpl,
		router:   muxRouter.Path(tpl).Subrouter(),
		methods:  make(map[string]string),
		children: make([]*Router, 0),
	}
	router.addMethod(OPTIONS, OPTIONS_DESC, router.options)
	return router
}

func (t *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	muxRouter := (*mux.Router)(t)
	muxRouter.ServeHTTP(w, req)
}

// SubPath returns a pointer to a new Router to handle a sub path
// under the original Router.
func (t *Router) SubPath(tpl string) *Router {
	path := t.path + tpl
	router := &Router{
		root:     t.root,
		path:     path,
		router:   t.root.Path(path).Subrouter(),
		methods:  make(map[string]string),
		children: make([]*Router, 0),
	}
	router.addMethod(OPTIONS, OPTIONS_DESC, router.options)
	t.children = append(t.children, router)
	return router
}

func (t *Router) addMethod(method string, desc string, f http.HandlerFunc) {
	t.methods[method] = desc
	t.router.Methods(method).HandlerFunc(f)
}

func (t *Router) options(w http.ResponseWriter, req *http.Request) {
	keys := make([]string, 0, len(t.methods))
	for key, _ := range t.methods {
		keys = append(keys, key)
	}
	w.Header().Set("Allow", strings.Join(keys, ","))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(nil)
	if err != nil {
		glog.Error(err)
		panic(err)
	}
}

// Get binds f to the GET method of the Router's path with description.
func (t *Router) Get(desc string, f http.HandlerFunc) *Router {
	t.addMethod(GET, desc, f)
	return t
}

// Post binds f to the POST method of the Router's path with description.
func (t *Router) Post(desc string, f http.HandlerFunc) *Router {
	t.addMethod(POST, desc, f)
	return t
}

// Put binds f to the PUT method of the Router's path with description.
func (t *Router) Put(desc string, f http.HandlerFunc) *Router {
	t.addMethod(PUT, desc, f)
	return t
}

// Patch binds f to the PATCH method of the Router's path with description.
func (t *Router) Patch(desc string, f http.HandlerFunc) *Router {
	t.addMethod(PATCH, desc, f)
	return t
}

// Delete binds f to the DELETE method of the Router's path with description.
func (t *Router) Delete(desc string, f http.HandlerFunc) *Router {
	t.addMethod(DELETE, desc, f)
	return t
}

// MethodIntro contains method with its description.
type MethodIntro struct {
	Method      string `json:"method"`
	Description string `json:"description"`
}

// SelfIntro contains all methods and their descriptions under the
// specified path.
type SelfIntro struct {
	Path    string        `json:"path"`
	Methods []MethodIntro `json:"methods"`
}

// SelfIntro returns SelfIntro of the Router itself and all routers
// under it.
func (t *Router) SelfIntro() []SelfIntro {
	intros := make([]SelfIntro, 0)
	var recursive func(router *Router)
	recursive = func(router *Router) {
		methods := make([]MethodIntro, 0, len(router.methods))
		for method, desc := range router.methods {
			methods = append(methods, MethodIntro{
				Method:      method,
				Description: desc,
			})
		}
		intros = append(intros, SelfIntro{
			Path:    router.path,
			Methods: methods,
		})
		for _, child := range router.children {
			recursive(child)
		}
	}
	recursive(t)
	return intros
}

// SelfIntroHandlerFunc wraps around SelfIntro() as an
// http.HandlerFunc to output in JSON format.
func (t *Router) SelfIntroHandlerFunc(
	w http.ResponseWriter, req *http.Request) {
	b, err := json.MarshalIndent(t.SelfIntro(), "", "  ")
	if err != nil {
		panic(err)
	}
	_, err = w.Write(b)
	if err != nil {
		panic(err)
	}
}
