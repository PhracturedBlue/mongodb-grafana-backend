import {QueryCtrl} from 'app/plugins/sdk';
import _ from 'lodash';
import './css/query-editor.css!'

export class MongoDBDatasourceQueryCtrl extends QueryCtrl {

  constructor($scope, $injector, templateSrv, $q, uiSegmentSrv) {
    super($scope, $injector);

    this.scope = $scope;
    this.target.collection = this.target.collection || "collection"
    this.target.target = this.target.target || '[]';
    this.target.type = this.target.type || 'timeserie';
    this.target.rawQuery = true;
    this.collectionSegment = uiSegmentSrv.newSegment(this.target.collection)
    this.uiSegmentSrv = uiSegmentSrv
  }

  getOptions(query) {
    return this.datasource.metricFindQuery(query || '');
  }

  getCollections() {
    return this.datasource
      .metricFindQuery("list_collections")
      .then(this.transformToSegments({}))
      .catch(this.handleQueryError.bind(this));
    //return Promise.resolve(_.map(["col1", "col2"], x => {
    //    return this.uiSegmentSrv.newSegment(x);
    //}))
  }

  collectionChanged() {
    this.target.collection = this.collectionSegment.value;
    this.panelCtrl.refresh();
  }

  onChangeInternal() {
    this.panelCtrl.refresh(); // Asks the panel to refresh data.
  }

  transformToSegments() {
    return results => {
      return _.map(results, segment => {
        return this.uiSegmentSrv.newSegment({
          value: segment.text,
          expandable: segment.expandable,
        });
      });
    }
  }

  handleQueryError(err) {
    this.error = err.message || 'Failed to issue metric query';
    return [];
  }
}

MongoDBDatasourceQueryCtrl.templateUrl = 'partials/query.editor.html';

