// n_order.js - krok zamówienia: odliczanie blokady miejsc i anulowanie.
// Miejsca są już zapisane ze statusem "marked"; ticker.go kasuje je po czasie,
// więc gość widzi, ile czasu zostało.

(function () {
  'use strict';

  var page = document.getElementById('order-step');
  if (!page) return;

  var minutes = Number(page.dataset.minutes || 5);
  var deadline = Date.now() + minutes * 60 * 1000;
  var out = document.getElementById('countdown');
  var timer;

  function tick() {
    var left = deadline - Date.now();
    if (left <= 0) {
      clearInterval(timer);
      out.textContent = RZ.t('order.expired');
      return;
    }
    var m = Math.floor(left / 60000);
    var s = Math.floor((left % 60000) / 1000);
    out.textContent = m + ':' + (s < 10 ? '0' : '') + s;
  }

  RZ.ready.then(function () {
    tick();
    timer = setInterval(tick, 1000);
  });

  document.getElementById('cancel-order').addEventListener('click', function () {
    var data = {
      rooms: document.getElementById('rooms').value,
      sits: document.getElementById('sits').value,
      'event-id': Number(page.dataset.eventId)
    };
    fetch('/api/ordercancel', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(data)
    }).finally(function () {
      location.href = '/n/res/' + page.dataset.userUrl + '?event-id=' + page.dataset.eventId;
    });
  });
})();
