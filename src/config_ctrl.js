export class MongoDBConfigCtrl {

  constructor($scope) {
    this.current.jsonData = this.current.jsonData || {};
    this.current.jsonData.mongodb_url = this.current.jsonData.mongodb_url || "mongodb://localhost:27017";
    this.current.jsonData.mongodb_db = this.current.jsonData.mongodb_db || "";
    this.current.jsonData.stages = this.current.jsonData.stages || []
  }
  removeStage(map) {
    const index = _.indexOf(this.current.jsonData.stages, map);
    this.current.jsonData.stages.splice(index, 1);
    this.render();
  }

  addStage() {
    this.current.jsonData.stages.push({ name: '', stage: '' });
  }

}
MongoDBConfigCtrl.templateUrl = 'partials/config.html';
