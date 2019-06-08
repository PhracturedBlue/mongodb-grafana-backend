all: grunt build rsync

grunt:
	grunt --force

build: 
	go build -o ./dist/mongodb-plugin_linux_amd64 ./pkg/

rsync:
	rsync -a dist/ grafana/plugins/mongodb-grafana/dist/
