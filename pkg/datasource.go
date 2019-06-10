package main

import (
	"fmt"
	"strings"
	"sort"
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

type MongoDBDatasource struct {
	plugin.NetRPCUnsupportedPlugin
	logger hclog.Logger
}

func (t *MongoDBDatasource) Query(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	t.logger.Debug("Query", "datasource", tsdbReq.Datasource.Name, "TimeRange", tsdbReq.TimeRange)
	json, err := simplejson.NewJson([]byte(tsdbReq.Queries[0].ModelJson))
	if err  != nil {
		return nil, err
	}
	queryType := json.Get("queryType").MustString()
	t.logger.Debug(fmt.Sprintf("Request: %+v", tsdbReq))

	var res *datasource.DatasourceResponse
	switch queryType {
	case "metricsQuery":
		return t.executeMetricsQuery(ctx, tsdbReq)
	case "timeSeriesQuery":
		fallthrough
	default:
		res, err = t.executeTimeseriesQuery(ctx, tsdbReq)
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

func (t *MongoDBDatasource) executeMetricsQuery(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	dbopts, err := t.getClient(ctx, tsdbReq)
	if err != nil {
		return nil, err
	}
	query := tsdbReq.Queries[0]
	queryJson, err := simplejson.NewJson([]byte(query.ModelJson))
	if err  != nil {
		return nil, err
	}
	target := queryJson.Get("target").MustString()
	response := &datasource.DatasourceResponse{}
	table := datasource.Table{
		Columns: []*datasource.TableColumn{
                        &datasource.TableColumn{Name: "value"},
		},
		Rows:    make([]*datasource.TableRow, 0),
	}
	qr := datasource.QueryResult{
		RefId:  query.RefId,
		Series: make([]*datasource.TimeSeries, 0),
		Tables: []*datasource.Table{&table},
	}
	t.logger.Debug("Got Metrics Target: " + target)
	switch target {
	case "ping":
		err = dbopts.client.Ping(ctx, nil)
	case "list_collections":
		collections, err := dbopts.client.Database(dbopts.db).ListCollectionNames(ctx, bson.D{{}})
		t.logger.Debug(fmt.Sprintf("List Collections: (%s) %v -> %+v", dbopts.db, err, collections))
		if (err == nil) {
			sort.Strings(collections)
			for _, k := range collections {
				rv := datasource.RowValue{Kind: datasource.RowValue_TYPE_STRING, StringValue: k}
				row := make([]*datasource.RowValue, 0)
				row = append(row, &rv)
				table.Rows = append(table.Rows, &datasource.TableRow{Values: row})
			}
		}
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported Metric: %s", query))
	}
	if err != nil {
		return nil, err
	}
	err = dbopts.client.Disconnect(ctx)
	if err != nil {
		return nil, err
	}
	response.Results = append(response.Results, &qr)
	return response, nil
}

func (t *MongoDBDatasource) executeTimeseriesQuery(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*datasource.DatasourceResponse, error) {
	response := &datasource.DatasourceResponse{}

	dbopts, err := t.getClient(ctx, tsdbReq)
	if err != nil {
		return nil, err
	}
	client := dbopts.client
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	t.logger.Debug("Connected to MongoDB")

	for _, query := range tsdbReq.Queries {
		queryObj, err := t.parseTarget(query, tsdbReq, dbopts.json)
		if err != nil {
			return nil, err
		}
		collection := client.Database(dbopts.db).Collection(queryObj.Collection)
		t.logger.Debug("Sending: " + t.bsonToJson(queryObj.Aggregate))
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

type DbOpts struct {
	client *mongo.Client
	json *simplejson.Json
	db string
}

func (t *MongoDBDatasource) getClient(ctx context.Context, tsdbReq *datasource.DatasourceRequest) (*DbOpts, error) {
	json, err := simplejson.NewJson([]byte(tsdbReq.Datasource.JsonData))
	if err  != nil {
		return nil, err
	}
	uri := json.Get("mongodb_url").MustString("mongodb://localhost:27017")
	db := json.Get("mongodb_db").MustString("test")

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	dbopts := DbOpts{
		client: client,
		json: json,
		db: db,
		}
	return &dbopts, err
}
type QueryObj struct {
	Collection string
	Aggregate bson.A
	Type string
}

func (t *MongoDBDatasource) parseTarget(query *datasource.Query, tsdbReq *datasource.DatasourceRequest, dbopts *simplejson.Json) (*QueryObj, error) {
	queryObj := QueryObj{
		Collection: "",
		Aggregate: nil,
		Type: "",
	}


	t.logger.Debug("QueryString: ", query.ModelJson)
	queryJson, err := simplejson.NewJson([]byte(query.ModelJson))
	if err  != nil {
		return nil, err
	}
	queryObj.Type = queryJson.Get("type").MustString("timeserie")
	target := queryJson.Get("target").MustString()
	from := tsdbReq.TimeRange.FromEpochMs
	to := tsdbReq.TimeRange.ToEpochMs
	// intervalMs := query.IntervalMs
	maxDataPoints := query.MaxDataPoints
	if maxDataPoints == 0 {
		maxDataPoints = 1
	}
	queryObj.Collection = queryJson.Get("collection").MustString()
	if queryObj.Collection == "" {
		return nil, errors.New("No collection specified")
	}
	target = varSub(target, from, to, maxDataPoints)
	t.logger.Debug("Target: ", target)

	err = bson.UnmarshalExtJSON([]byte(target), true, &queryObj.Aggregate)
	if err != nil {
		t.logger.Error(fmt.Sprintf("Failed: %+v", err))
		return nil, err
	}
	t.logger.Debug(fmt.Sprintf("All Items: %+v", queryObj.Aggregate))
	var newAggregate bson.A
	for k, v := range queryObj.Aggregate {
		if dict, ok := v.(primitive.D); ok {
			for _, _stage := range dbopts.Get("stages").MustArray() {
				stage, ok := _stage.(map[string]interface{})
				if ok == false {
					continue
				}
				if dict[0].Key == "$" + stage["name"].(string) {
					t.logger.Debug(fmt.Sprintf("Stage: %s ==> %s", stage["name"], stage["stage"].(string)))
					var replace string
					if sval, ok := dict[0].Value.(string); ok {
						replace = sval
					} else {
						res, err := bson.MarshalExtJSON(dict[0].Value, true, true)
						if err != nil {
							return nil, errors.New(fmt.Sprintf("Can't Marshall result: (%s) %+v", reflect.TypeOf(dict[0].Value), dict[0].Value))
						}
						replace = string(res)
					}
					newval := strings.Replace(stage["stage"].(string), "$QUERY", replace[1:len(replace)-1], -1)
					newval = varSub(newval, from, to, maxDataPoints)
					t.logger.Debug(fmt.Sprintf("Found %s ==> >%s<", stage["name"], newval))
					var replStage bson.A
					err = bson.UnmarshalExtJSON([]byte("[" + newval + "]"), true, &replStage)
					if err != nil {
						return nil, errors.New(fmt.Sprintf("Failed to parse stage macro: %s", err))
					}
					for _, v1 := range replStage {
						newAggregate = append(newAggregate, v1)
					}
					v = nil
					break
				}
			}
		}
		if v != nil {
			newAggregate = append(newAggregate, v)
			t.logger.Debug(fmt.Sprintf("Item type: %s ==> %s", reflect.TypeOf(k), reflect.TypeOf(v)))
			t.logger.Debug(fmt.Sprintf("Item: %+v", v))
		}
	}
	queryObj.Aggregate = newAggregate
	t.logger.Debug(fmt.Sprintf("Replaced Items: %+v", queryObj.Aggregate))
	return &queryObj, nil
}


type TimeSeries struct {
    Name       string   `json:"name" bson:"name"`
    Value      float64  `json:"value" bson:"value"`
    Timestamp  time.Time    `json:"ts" bson:"ts"`
}

func (t *MongoDBDatasource) parseTimeseriesResponse(ctx context.Context, query *datasource.Query, resp *mongo.Cursor) (*datasource.QueryResult, error) {
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
		// t.logger.Debug(fmt.Sprintf("Return: %+v", result))
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

func (t *MongoDBDatasource) parseTableResponse(ctx context.Context, query *datasource.Query, resp *mongo.Cursor) (*datasource.QueryResult, error) {
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
		// t.logger.Debug(fmt.Sprintf("Return: %+v", document))
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
			} else if ival, ok := value.(int32); ok {
				rv.Kind = datasource.RowValue_TYPE_INT64
				rv.Int64Value = int64(ival)
			} else if sval, ok := value.(string); ok {
				rv.Kind = datasource.RowValue_TYPE_STRING
				rv.StringValue = sval
			} else if bval, ok := value.(bool); ok {
				rv.Kind = datasource.RowValue_TYPE_BOOL
				rv.BoolValue = bval
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

func (t *MongoDBDatasource) bsonToJson(data bson.A) (string) {
	var strs []string
	for _, v := range data {
		res, err := bson.MarshalExtJSON(v, true, true)
		if err != nil {
			t.logger.Error(fmt.Sprintf("Failed to Marshal data: %s", err))
			strs = append(strs, fmt.Sprintf("%+v", v))
		} else {
			strs = append(strs, string(res))
		}
	}
	return "[" + strings.Join(strs, ",") + "]"
}

func varSub(target string, from int64, to int64, maxDataPoints int64) (string) {
	target = strings.Replace(target, "\"$from\"", "{\"$date\": {\"$numberLong\": \"" + strconv.FormatInt(from, 10) + "\"}}", -1)
	target = strings.Replace(target, "\"$to\"", "{\"$date\": {\"$numberLong\": \"" + strconv.FormatInt(to, 10) + "\"}}", -1)
	target = strings.Replace(target, "\"$maxDataPoints\"", strconv.FormatInt(maxDataPoints, 10), -1)
	return target
}

