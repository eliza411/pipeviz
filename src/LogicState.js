var _ = require('../bower_components/lodash/dist/lodash');

function LogicState(obj, path, container) {
    _.assign(this, obj);
    this._path = path;
    this._container = container;
}

LogicState.prototype.vType = function() {
    return 'logic';
};

LogicState.prototype.name = function() {
    if (_.has(this, 'nick')) {
        return this.nick;
    }

    var p = this._path.split('/');
    return p[p.length-1];
};

module.exports = LogicState;
