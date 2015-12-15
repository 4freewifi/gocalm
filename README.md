# gocalm

gocalm is a RESTful service framework carefully designed to work with
net/http and [goroute][] but it is not tightly coupled to goroute. It is
encouraged to store necessary data in self-defined context struct and
keep the interface clean. Check the typical usage in calm_test.go .

Need memcached to run `go test` and if caching is enabled.

## API

Visit <http://godoc.org/github.com/4freewifi/gocalm>

[goroute]: <http://godoc.org/github.com/4freewifi/goroute>

# License

Copyright 2015 John Lee <john@4free.com.tw>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
