# MongoDB datasource for Grafana

## Features
Allows MongoDB to be used as a data source for Grafana by providing a backend plugin [API](https://grafana.com/docs/plugins/developing/backend-plugins-guide/) into MongoDB aggregation queries

This package was forked from https://github.com/JamesOsgood/mongodb-grafana which is a purely frontend plugin.

## Requirements

* **Grafana** > 3.x.x
* **MongoDB** > 3.4.x

## Installation

The git repo does not include the dist/ dirctory needed for grafana.  Either download a pre-built release from
https://github.com/PhracturedBlue/mongodb-grafana-backend/releases or build via `make`

### Install the Grafana plugin components

* Copy the whole mongodb-grafana dir into the Grafana plugins dir ( /usr/local/var/lib/grafana/plugins )
* Restart the Grafana server. If installed via Homebrew, this will be `brew services restart grafana`

## Examples

Create a new data source of type MongoDB as shown below. The MongoDB details are :

* **MongoDB URL** - `mongodb://rpiread:rpiread@rpi-sensor-data-shard-00-00-ifxxs.mongodb.net:27017,rpi-sensor-data-shard-00-01-ifxxs.mongodb.net:27017,rpi-sensor-data-shard-00-02-ifxxs.mongodb.net:27017/test?ssl=true&replicaSet=rpi-sensor-data-shard-0&authSource=admin`
* **MongoDB Database** - `rpi`

<img src="media/mongodb_config.png" alt="Sample Data Source" style="width: 500px;"/>

Then save the data source

#### Using stage macros

Because MongoDB isn't ideally suited to extracting timeseries data, aggregate pipelines 
can become quite long with repeated boilerplate for each query.  The MongoDB datasource
provides the ability to define macros which can be used in panel queries to reduce the
amount of duplicated boilerplate.  These are very similar to grafana's constant variables
except that (a) they support per-query confguration, and (b) they work with alerts

```
timegroup ==>
{
  "$match": {
    "ts": {
      "$gte": "$from", "$lt": "$to"
    },
    $QUERY
  }
},
{
  "$bucketAuto": {
    "groupBy": "$ts",
    "buckets" : "$maxDataPoints",
    "output": {
      "root": {"$push": "$$ROOT"}
    }
  }
},
{$unwind": "$root"},
{
  "$project: {
    "_id": 1, "data": "$root.data", "ts": "$root.ts"
  }
}
```
The above example assumes a schema with timeseries stored in `ts` and data stored under `data`
It will filter data by timerange and parameters specified in the panel qury, and then group
into the number of points requested in `maxDataPoints`.  Note that the macro is not legal JSON
syntax because `$QUERY` is unquoted.  The QUERY value will be substituted before JSON
parsing and the result after substitution will be legal JSON.

To make use of the above filter, a query may look like:
```
db.timeseries.aggregate(
[
  "$timegroup": {"data.name": "a"},
  {
    "$project": {
      "_id": 0,
      "ts": 1,
      "value": "$data.value"
    }
  }
])
```

The above would be combined into a single JSON:
```
db.timeseries.aggregate(
[
  {
    "$match": {
      "ts": {
        "$gte": "$from", "$lt": "$to"
      },
      "data.name": "a"
    }
  },
  {
    "$bucketAuto": {
      "groupBy": "$ts",
      "buckets" : "$maxDataPoints",
      "output": {
        "root": {"$push": "$$ROOT"}
      }
    }
  },
  {$unwind": "$root"},
  {
    "$project: {
      "_id": 1, "data": "$root.data", "ts": "$root.ts"
    }
  }
  {
    "$project": {
      "_id": 0,
      "ts": 1,
      "value": "$data.value"
    }
  }
])
```


#### Example 1 - Simple aggregate to rename fields

Import the dashboard in `examples\RPI MongoDB - Atlas.json`

This should show a graph of light sensor values from a Raspberry PI with an [EnviroPHAT](https://thepihut.com/products/enviro-phat) board feeding readings every minute into a MongoDB Atlas database.

<img src="media/sample_dashboard.png" alt="Sample Dashboard" style="width: 800px;"/>

Clicking on the title of the graph allows you to see the aggregation query being run against the 'RPI Atlas' data source

<img src="media/sample_query.png" alt="Sample Query" style="width: 800px;"/>

The query here is

```javascript
db.sensor_value.aggregate ( [ 
{ "$match" :    {   "sensor_type" : "$sensor",   "host_name" : "$host",   "ts" : { "$gte" : "$from", "$lte" : "$to" } }  },        
 {"$sort" : {"ts" : 1}},            
 {"$project" :   {  "name" : "value",   "value" : "$sensor_value",  "ts" : "$ts",  "_id" : 0} } ])
 ```

 The API is expecting back documents with the following fields

 * `name` - Name of the series ( will be displayed on the graph)
 * `value` - The float value of the point
 * `ts` - The time of the point as a BSON date

 These documents are then converted into the [Grafana API](http://docs.grafana.org/plugins/developing/datasources/)

`$from` and `$to` are expanded by the plugin as BSON dates based on the range settings on the UI.

## Template Variables

`$sensor` and `$host` are template variables that are filled in by Grafana based on the drop down. The sample template queries are shown below. They expect documents to be returned with a single `_id` field.


<img src="media/sample_template.png" alt="Sample Templates" style="width: 800px;"/>

#### Example 2 - Using $bucketAuto to push data point aggregation to the server

Grafana tells the backend server the date range along with the size of the buckets that should be used to calculate points. Therefore it's possible to use the MongoDB aggregation operator [$bucketAuto](https://docs.mongodb.com/manual/reference/operator/aggregation/bucketAuto/) to automatically bucket the data points into display points. To support this the backend provides the `$maxDataPoints` macro so that queries such as the one below can be written

```javascript
db.sensor_value.aggregate( [ 
{  "$match" :  {  "sensor_type" : "$sensor", "host_name" : "$host" , "ts" : { "$gte" : "$from", "$lt" : "$to" }}},
{  "$bucketAuto" :  { "groupBy" : "$ts",  
                           "buckets" : "$maxDataPoints", 
                            "output" :  {  "maxValue" : { "$max" : "$sensor_value" }  }   }   },  
{  "$project" :  {  "name" : "value",  "value" : "$maxValue",  "ts" : "$_id.min",  "_id" : 0  }  }  ]  )
```    
Note that ```_id``` field of the bucketAuto output contains the start and end of the bucket so we can use that as the ```ts``` value

The dashboard in `examples\RPI MongoDB Bucket - Atlas.json` shows this.

#### Example 3 - Using a Tabel Panel

<img src="media/table_panel.png" alt="Table Panel" style="width: 800px;"/>

Table panels are now supported with queries of the form

```javascript
db.sensor_value.aggregate(
[
    {  "$match" :  {  "ts" : { "$gte" : "$from", "$lt" : "$to"  }}},
    {  "$group":    {  "_id":  { "sensor_name" : "$sensor_name",  "sensor_type" : "$sensor_type"  }, "cnt" : { "$sum" : 1 },  "ts" : { "$max" : "$ts" }  } }, 
    { "$project": {  "name" : { "$concat" : ["$_id.sensor_name",":","$_id.sensor_type" ]},  "value" : "$cnt",  "ts" : 1, "_id" : 0} } 
])
```    

The dashboard in `examples\Sensor Values Count - Atlas.json` shows this.
