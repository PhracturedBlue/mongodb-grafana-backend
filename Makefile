all: grunt build

grunt:
	grunt 

build: 
	go build -o ./dist/mongodb-plugin_linux_amd64 ./pkg/
