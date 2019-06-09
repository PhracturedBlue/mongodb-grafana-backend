import {MongoDBDatasource} from './datasource';
import {MongoDBDatasourceQueryCtrl} from './query_ctrl';

class MongoDBConfigCtrl {}
MongoDBConfigCtrl.templateUrl = 'partials/config.html';

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
