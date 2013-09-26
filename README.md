# gocalm

gocalm is a RESTful service framework carefully designed to work with
net/http and goroute but it is not tightly coupled to goroute. It is
encouraged to store necessary data in self-defined context struct and
keep the interface clean. Check the typical usage in calm_test.go .

## kvpairs

kvpairs is a map[string]string as an argument to communicate with
Model to specify the data to retrieve/modify.  It is designed to work
with <http://godoc.org/github.com/johncylee/goroute> but it's not
tightly coupled.  gocalm will also automatically parse query values in
URL to put into kvpairs.  It will overwrite existing values, so it's
best not to use duplicated parameter names.

## API

Visit <http://godoc.org/github.com/johncylee/gocalm>
