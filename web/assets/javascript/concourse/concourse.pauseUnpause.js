concourse.PauseUnpause = function ($el, pauseCallback, unpauseCallback) {
  this.$el = $el;
  this.pauseCallback = pauseCallback === undefined ? function(){} : pauseCallback;
  this.unpauseCallback = unpauseCallback === undefined ? function(){} : unpauseCallback;
  this.pauseBtn = this.$el.find('.js-pauseUnpause').pausePlayBtn();
  this.pauseEndpoint = "/api/v1/" + this.$el.data('endpoint') + "/pause";
  this.unPauseEndpoint = "/api/v1/" + this.$el.data('endpoint') + "/unpause";
};

concourse.PauseUnpause.prototype.bindEvents = function () {
  var _this = this;

  _this.$el.delegate('.js-pauseUnpause.disabled', 'click', function (event) {
    _this.pause();
  });

  _this.$el.delegate('.js-pauseUnpause.enabled', 'click', function (event) {
    _this.unpause();
  });
};

concourse.PauseUnpause.prototype.pause = function (pause) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.pauseEndpoint,
  }).done(function (resp, jqxhr) {
    _this.pauseBtn.enable();
    _this.pauseCallback();
  }).error(_this.requestError.bind(_this));
};


concourse.PauseUnpause.prototype.unpause = function (event) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.unPauseEndpoint
  }).done(function (resp) {
    _this.pauseBtn.disable();
    _this.unpauseCallback();
  }).error(_this.requestError.bind(_this));
};

concourse.PauseUnpause.prototype.requestError = function (resp) {
  this.pauseBtn.error();

  if (resp.status == 401) {
    concourse.redirect("/login");
  }
};
