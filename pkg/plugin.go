package main

import (
	"github.com/grafana/grafana_plugin_model/go/datasource"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
)

var pluginLogger = hclog.New(&hclog.LoggerOptions{
	Name:  "mongodb-grafana",
	Level: hclog.LevelFromString("DEBUG"),
})

func main() {
	//log.SetOutput(os.Stderr) // the plugin sends logs to the host process on strErr
	pluginLogger.Debug("Running GRPC server")

	plugin.Serve(&plugin.ServeConfig{

		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "grafana_plugin_type",
			MagicCookieValue: "datasource",
		},
		Plugins: map[string]plugin.Plugin{
			"backend-datasource": &datasource.DatasourcePluginImpl{Plugin: &MongoDBDatasource{
				logger: pluginLogger,
			}},
		},

		// A non-nil value here enables gRPC serving for this plugin...
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
