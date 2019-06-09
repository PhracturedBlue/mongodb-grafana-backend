all: grunt build

grunt:
	grunt --force

build: 
	go build -o ./dist/mongodb-be-plugin_linux_amd64 ./pkg/

rsync:
	rsync -a dist/ grafana/plugins/mongodb-grafana-backend/dist/
