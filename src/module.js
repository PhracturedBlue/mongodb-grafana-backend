import {MongoDBDatasource} from './datasource';
import {MongoDBDatasourceQueryCtrl} from './query_ctrl';
import {MongoDBConfigCtrl} from './config_ctrl';

class MongoDBQueryOptionsCtrl {}
MongoDBQueryOptionsCtrl.templateUrl = 'partials/query.options.html';

class MongoDBAnnotationsQueryCtrl {}
MongoDBAnnotationsQueryCtrl.templateUrl = 'partials/annotations.editor.html'

export {
  MongoDBDatasource as Datasource,
  MongoDBDatasourceQueryCtrl as QueryCtrl,
  MongoDBConfigCtrl as ConfigCtrl,
  MongoDBQueryOptionsCtrl as QueryOptionsCtrl,
  MongoDBAnnotationsQueryCtrl as AnnotationsQueryCtrl
};
