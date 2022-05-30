#!/bin/bash

# go build -o compiler/main compiler/main.go

#echo a:{"test":1}:test/views/index.xhtml
# echo 'a:H4sIAAAAAAAA/6pWKkktLlGyUjCsBQAAAP//AQAA//+jJmhxCwAAAA==:../test/views/index.xhtml' | go run compiler/main.go

#echo a:{"test": [1, 2, 3]}:test/views/index.xhtml
# echo 'a:H4sIAAAAAAAA/6pWKkktLlGyUog21FEw0lEwjq0FAAAA//8BAAD//wvjorgTAAAA:../test/views/index.xhtml' | go run compiler/main.go

#echo a:{"test": {a: 1, b: 2, c: 3}}:test/views/index.xhtml
# echo -e 'a:H4sIAAAAAAAA/6pWKkktLlGyUqhWSlSyUjDUUVBKUrJSMNJRUEpWslIwrq0FAAAA//8BAAD//3Cj5VAiAAAA:test/views/index.xhtml' | go run compiler/main.go

echo -e 'set:root=/home/shaynejr/WebDev/NPM/turbx/test/views\nset:ext=xhtml\na:H4sIAAAAAAAA/6pWKkktLlGyUqhWSlSyUjDUUVBKUrJSMNJRUEpWslIwrq0FAAAA//8BAAD//3Cj5VAiAAAA:index' | go run compiler/main.go
# echo -e 'set:root=/home/shaynejr/WebDev/NPM/turbx/test/views\nset:ext=xhtml\nset:cache_component=embed\na:H4sIAAAAAAAA/6pWKkktLlGyUqhWSlSyUjDUUVBKUrJSMNJRUEpWslIwrq0FAAAA//8BAAD//3Cj5VAiAAAA:index' | go run compiler/main.go
