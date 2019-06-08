import _ from "lodash";

export class GenericDatasource {

  constructor(instanceSettings, $q, backendSrv, templateSrv, timeSrv) {
    this.type = instanceSettings.type;
    this.url = instanceSettings.url;
    this.name = instanceSettings.name;
    this.db = { 'url' : instanceSettings.jsonData.mongodb_url, 'db' : instanceSettings.jsonData.mongodb_db }
    this.id = instanceSettings.id;
    this.q = $q;
    this.backendSrv = backendSrv;
    this.templateSrv = templateSrv;
    this.timeSrv = timeSrv;
    this.withCredentials = instanceSettings.withCredentials;
    this.headers = {'Content-Type': 'application/json'};
    if (typeof instanceSettings.basicAuth === 'string' && instanceSettings.basicAuth.length > 0) {
      this.headers['Authorization'] = instanceSettings.basicAuth;
    }
  }

  query(options) {
    var query = this.buildQueryParameters(options);
    query.targets = query.targets.filter(t => !t.hide);
    query.db = this.db

    if (query.targets.length <= 0) {
      return this.q.when({data: []});
    }

    return this.doRequest(query)
    .then(result => {
      var res= [];
      _.forEach(result.data.results, r => {
        _.forEach(r.series, s => {
          res.push({target: s.name, datapoints: s.points});
        })
        _.forEach(r.tables, t => {
          t.type = 'table';
          t.refId = r.refId;
          res.push(t);
        })
      })

      result.data = res;
      return result;
    });
  }

  buildQueryParameters(options) {
    //remove placeholder targets
    options.targets = _.filter(options.targets, target => {
      return target.target !== 'select metric';
    });

    var targets = _.map(options.targets, target => {
      return {
        queryType: 'query',
        target: this.templateSrv.replace(target.target, options.scopedVars, ''),
        refId: target.refId,
        hide: target.hide,
        type: target.type || 'timeserie',
        datasourceId: this.id
      };
    });

    options.targets = targets;

    return options;
  }

  testDatasource() {
    var query = {
        range: this.timeSrv.timeRange(),
        targets: [{
          queryType: 'testConnection',
          target: '',
          refId: "A",
          datasourceId: 0
        }]
      };
    return this.doRequest(query)
    .then(response => {
      if (response.status === 200) {
        return { status: response.data.status, message: response.data.message, title: response.data.display_status };
      }
    });
  }

  annotationQuery(options) {
    var query = this.templateSrv.replace(options.annotation.query, {}, 'glob');
    var annotationQuery = {
      range: options.range,
      annotation: {
        name: options.annotation.name,
        datasource: options.annotation.datasource,
        enable: options.annotation.enable,
        iconColor: options.annotation.iconColor,
        query: query
      },
      rangeRaw: options.rangeRaw
    };

    if (this.templateSrv.getAdhocFilters) {
      query.adhocFilters = this.templateSrv.getAdhocFilters(this.name);
    } else {
      query.adhocFilters = [];
    }

    return this.doDirectRequest({
      url: this.url + '/annotations',
      method: 'POST',
      data: annotationQuery
    }).then(result => {
      response.data.$$status = result.status;
      response.data.$$config = result.config;
      return result.data;
    });
  }

  metricFindQuery(query) {
    var range = this.timeSrv.timeRange();
    var targets = [{
      queryType: 'search',
      target: this.templateSrv.replace(query, null, ''),
      db: this.db,
      datasourceId: this.id,
      refId: "search",
    }];
    var options = {
      range: range,
      targets: targets
    };
    return this.doRequest(options).then(this.mapToTextValue);
  }

  mapToTextValue(result) {
    var table = result.data.results.search.tables[0];

    if (!table) {
      return [];
    }

    return _.map(table.rows, (row, i) => {
      if (row.length > 1) {
        return { text: row[0], value: row[1] };
      } else if (_.isObject(row[0])) {
        return { text: row[0], value: i};
      }
      return { text: row[0], value: row[0] };
    });
  }

  doDirectRequest(options) {
    options.withCredentials = this.withCredentials;
    options.headers = this.headers;

    return this.backendSrv.datasourceRequest(options);
  }

  doRequest(options) {
    return this.backendSrv.datasourceRequest({
      url: '/api/tsdb/query',
      method: 'POST',
      data: {
        from: options.range.from.valueOf().toString(),
        to: options.range.to.valueOf().toString(),
        queries: options.targets,
      }
    });
  }

  getTagKeys(options) {
    return new Promise((resolve, reject) => {
      this.doRequest({
        url: this.url + '/tag-keys',
        method: 'POST',
        data: options
      }).then(result => {
        return resolve(result.data);
      });
    });
  }

  getTagValues(options) {
    return new Promise((resolve, reject) => {
      this.doRequest({
        url: this.url + '/tag-values',
        method: 'POST',
        data: options
      }).then(result => {
        return resolve(result.data);
      });
    });
  }

}
