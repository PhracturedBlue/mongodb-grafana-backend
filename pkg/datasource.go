package main

import (
	"fmt"
	"strings"
	"strconv"
	"time"
	"errors"
	"reflect"

	simplejson "github.com/bitly/go-simplejson"

	"golang.org/x/net/context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/grafana/grafana_plugin_model/go/datasource"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
)

type JsonDatasource struct {
	plugin.NetRPCUnsupportedPlugin
	logger hclog.Logger
}

func (t *JsonDatasource) Query(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	t.logger.Debug("Query", "datasource", tsdbReq.Datasource.Name, "TimeRange", tsdbReq.TimeRange)
	json, err := simplejson.NewJson([]byte(tsdbReq.Queries[0].ModelJson))
	if err  != nil {
		return nil, err
	}
	queryType := json.Get("queryType").MustString()
	t.logger.Debug(fmt.Sprintf("Request: %+v", tsdbReq))

	var res *datasource.DatasourceResponse
	switch queryType {
	case "testConnection":
		res, err = t.executeTestConnection(ctx, tsdbReq)
	//case "metricsQuery":
	//	//return t.executeMetricsQuery(ctx, tsdbReq)
	//	return nil, nil
	case "timeSeriesQuery":
		fallthrough
	default:
		res, err = t.executeTimSeriesQuery(ctx, tsdbReq)
	}
	// Ths is a work-around for the 'Metric request error'
	if res == nil && err != nil {
		response := &datasource.DatasourceResponse{}
		qr := datasource.QueryResult{
			RefId:  "A",
			Error:  fmt.Sprintf("%s", err),
		}
		response.Results = append(response.Results, &qr)
		t.logger.Error(fmt.Sprintf("%s", err))
		return response, nil
	}
	return res, err
}

func (t *JsonDatasource) executeTestConnection(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	client, _, err := t.getClient(ctx, tsdbReq)
	if err != nil {
		return nil, err
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	err = client.Disconnect(ctx)
	if err != nil {
		return nil, err
	}
	response := &datasource.DatasourceResponse{}
	return response, nil
}

func (t *JsonDatasource) executeTimSeriesQuery(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	response := &datasource.DatasourceResponse{}

	client, db, err := t.getClient(ctx, tsdbReq)
	if err != nil {
		return nil, err
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	t.logger.Debug("Connected to MongoDB")

	for _, query := range tsdbReq.Queries {
		queryObj, err := t.parseTarget(query.ModelJson, tsdbReq)
		if err != nil {
			return nil, err
		}
		collection := client.Database(db).Collection(queryObj.Collection)
		t.logger.Debug(fmt.Sprintf("Sending: %+v", queryObj.Aggregate))
		resp, err := collection.Aggregate(ctx, queryObj.Aggregate, nil)
		if err != nil {
			return nil, err
		}
		defer resp.Close(ctx)
		t.logger.Debug(fmt.Sprintf("Response: %+v", resp))

		var res *datasource.QueryResult
		if queryObj.Type == "timeserie" {
			t.logger.Debug("Time series Query")
			res, err = t.parseTimeseriesResponse(ctx, query, resp)
		} else {
			t.logger.Debug("Table Query")
			res, err = t.parseTableResponse(ctx, query, resp)
		}
		if err != nil {
			return nil, err
		}
		response.Results = append(response.Results, res)
	}

	err = client.Disconnect(ctx)
	if err != nil {
		return nil, err
	}
	t.logger.Debug("Connection to MongoDB closed.")
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (t *JsonDatasource) getClient(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*mongo.Client, string, error) {
	json, err := simplejson.NewJson([]byte(tsdbReq.Datasource.JsonData))
	if err  != nil {
		return nil, "", err
	}
	uri := json.Get("mongodb_url").MustString("mongodb://localhost:27017")
	db := json.Get("mongodb_db").MustString("test")

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	return client, db, err
}
type QueryObj struct {
	Collection string
	Aggregate interface{}
	Type string
}

func (t *JsonDatasource) parseTarget(queryStr string, tsdbReq *datasource.DatasourceRequest) (*QueryObj, error) {
	queryObj := QueryObj{
		Collection: "",
		Aggregate: nil,
		Type: "",
	}


	t.logger.Debug("QueryString: ", queryStr)
	query, err := simplejson.NewJson([]byte(queryStr))
	if err  != nil {
		return nil, err
	}
	queryObj.Type = query.Get("type").MustString("timeserie")
	target := query.Get("target").MustString()
	from := tsdbReq.TimeRange.FromEpochMs
	to := tsdbReq.TimeRange.ToEpochMs
	maxDataPoints := query.Get("maxDataPoints").MustInt(1)
	if maxDataPoints == 0 {
		maxDataPoints = 1
	}
	sepIdx := strings.Index(target, "(")
	if sepIdx == -1 {
		return nil, errors.New("Could not locate db command")
	}
	sections := strings.Split(target[3:sepIdx], ".")
	if sections[1] != "aggregate" {
		return nil, errors.New("Only 'aggregate' queries are supported")
	}
	target = target[sepIdx+1:len(target)-1]
	queryObj.Collection = sections[0]
	target = strings.Replace(target, "\"$from\"", "{\"$date\": {\"$numberLong\": \"" + strconv.FormatInt(from, 10) + "\"}}", -1)
	target = strings.Replace(target, "\"$to\"", "{\"$date\": {\"$numberLong\": \"" + strconv.FormatInt(to, 10) + "\"}}", -1)
	target = strings.Replace(target, "\"$maxDataPoints\"", strconv.Itoa(maxDataPoints), -1)
	t.logger.Debug("Target: ", target)

	err = bson.UnmarshalExtJSON([]byte(target), true, &queryObj.Aggregate)
	if err != nil {
		t.logger.Error(fmt.Sprintf("Failed: %+v", err))
		return nil, err
	}
	return &queryObj, nil
}


type TimeSeries struct {
    Name       string   `json:"name" bson:"name"`
    Value      float64  `json:"value" bson:"value"`
    Timestamp  time.Time    `json:"ts" bson:"ts"`
}

func (t *JsonDatasource) parseTimeseriesResponse(ctx context.Context, query *datasource.Query, resp *mongo.Cursor) (*datasource.QueryResult, error) {
	qr := datasource.QueryResult{
		RefId:  query.RefId,
		Series: make([]*datasource.TimeSeries, 0),
		Tables: make([]*datasource.Table, 0),
	}
	names := make(map[string]*datasource.TimeSeries)
	for resp.Next(ctx) {
		result := TimeSeries{}
		err := resp.Decode(&result)
		if err != nil {
			return nil, err
		}
		t.logger.Debug(fmt.Sprintf("Return: %+v", result))
		ts, ok := names[result.Name]
		if ! ok {
			ts = &datasource.TimeSeries{Name: result.Name}
			names[result.Name] = ts
		}
		ts.Points = append(ts.Points, &datasource.Point{Timestamp: result.Timestamp.UnixNano() / 1000000, Value: result.Value})
	}
	for _, v:= range names {
		qr.Series = append(qr.Series, v)
	}
	return &qr, nil
}

func (t *JsonDatasource) parseTableResponse(ctx context.Context, query *datasource.Query, resp *mongo.Cursor) (*datasource.QueryResult, error) {
	qr := datasource.QueryResult{
		RefId:  query.RefId,
		Series: make([]*datasource.TimeSeries, 0),
		Tables: make([]*datasource.Table, 0),
	}
	table := datasource.Table{
		Columns: make([]*datasource.TableColumn, 0),
		Rows:    make([]*datasource.TableRow, 0),
	}
	for resp.Next(ctx) {
		// var document bson.M
		document := make(map[string]interface{})
		err := resp.Decode(&document)
		if err != nil {
			return nil, err
		}
		t.logger.Debug(fmt.Sprintf("Return: %+v", document))
		row := make([]*datasource.RowValue, 0)
		for key, value := range document {
			idx := -1
			for k, v := range table.Columns {
				if v.Name == key {
					idx = k
					break
				}
			}
			if idx == -1 {
				table.Columns = append(table.Columns, &datasource.TableColumn{Name: key})
				idx = len(table.Columns) - 1
			}
			t.logger.Debug(fmt.Sprintf("Index (%s): %d", key, idx))
			for len(row) < idx + 1 {
				row = append(row, nil)
			}
			rv := datasource.RowValue{}
			if fval, ok := value.(float64); ok {
				rv.Kind = datasource.RowValue_TYPE_DOUBLE
				rv.DoubleValue = fval
			} else if ival, ok := value.(int64); ok {
				rv.Kind = datasource.RowValue_TYPE_INT64
				rv.Int64Value = ival
			} else if sval, ok := value.(string); ok {
				rv.Kind = datasource.RowValue_TYPE_STRING
				rv.StringValue = sval
			} else if tval, ok := value.(primitive.DateTime); ok {
				rv.Kind = datasource.RowValue_TYPE_INT64
				rv.Int64Value = int64(tval)
			} else {
				return nil, errors.New(fmt.Sprintf("Could not handle type %s of %s", reflect.TypeOf(value), key))
			}
			row[idx] = &rv
		}
		table.Rows = append(table.Rows, &datasource.TableRow{Values: row})
	}
	qr.Tables = append(qr.Tables, &table)
	return &qr, nil
}

