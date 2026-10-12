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

  // Podsumowanie kosztów: miejsca + po jednym wierszu na każdą wpisaną
  // dopłatę. Gość widzi pełny rachunek w trakcie wypełniania formularza,
  // a "Razem" na górze zawsze z walutą.
  var summary = document.getElementById('summary');
  if (summary) {
    var seatsTotal = Number(summary.dataset.seatsTotal || 0);
    var currency = summary.dataset.currency || '';
    var totalOut = document.getElementById('order-total');
    var totalField = document.querySelector('input[name="total-price"]');

    function money(n) {
      var s = String(n).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
      return currency ? s + ' ' + currency : s;
    }

    function recount() {
      var extra = 0;
      summary.querySelectorAll('.addon-line').forEach(function (line) {
        var input = document.getElementById(line.dataset.input);
        var amount = input ? (Number(input.value) || 0) : 0;
        var price = Number(line.dataset.price) || 0;
        if (amount <= 0) {
          line.hidden = true;
          return;
        }
        line.hidden = false;
        var sum = amount * price;
        extra += sum;
        line.querySelector('.qty').textContent = RZ.t('order.qty', amount);
        line.querySelectorAll('.num')[1].textContent = money(sum);
      });

      var sum = seatsTotal + extra;
      if (totalOut) totalOut.textContent = money(sum);
      if (totalField) totalField.value = sum;
    }

    var addons = document.getElementById('addons');
    if (addons) addons.addEventListener('input', recount);
    recount();
  }

  // walidacja z komunikatami w języku imprezy (form ma novalidate, bo natywne
  // dymki przeglądarki byłyby w jej języku, np. czeskim na polskiej imprezie)
  var form = document.getElementById('order-form');
  var fields = Array.prototype.slice.call(form.querySelectorAll('input:not([hidden]), select, textarea'));

  function showError(el, text) {
    var box = el.closest('.f');
    var err = box.querySelector('.ff-err');
    if (!err) {
      err = document.createElement('small');
      err.className = 'ff-err';
      err.setAttribute('role', 'alert');
      box.appendChild(err);
    }
    box.classList.toggle('bad', !!text);
    err.textContent = text;
    err.hidden = !text;
    if (text) el.setAttribute('aria-invalid', 'true');
    else el.removeAttribute('aria-invalid');
  }

  form.addEventListener('focusout', function (e) {
    var el = e.target;
    if (fields.indexOf(el) < 0) return;
    if (el.value !== '' || el.closest('.f').classList.contains('bad')) showError(el, RZ.inputError(el));
  });

  form.addEventListener('input', function (e) {
    var el = e.target;
    if (fields.indexOf(el) >= 0 && el.closest('.f').classList.contains('bad')) showError(el, RZ.inputError(el));
  });

  form.addEventListener('submit', function (e) {
    var first = null;
    fields.forEach(function (el) {
      var text = RZ.inputError(el);
      showError(el, text);
      if (text && !first) first = el;
    });
    if (first) {
      e.preventDefault();
      first.focus();
    } else if (form.dataset.sent) {
      e.preventDefault(); // drugie kliknięcie: miejsca są już zamawiane
    } else {
      form.dataset.sent = '1';
    }
  });

  RZ.noNativeTips(form);

  document.getElementById('cancel-order').addEventListener('click', function () {
    var data = {
      rooms: document.getElementById('rooms').value,
      sits: document.getElementById('sits').value,
      lock: document.getElementById('lock').value,
      'event-id': Number(page.dataset.eventId)
    };
    fetch('/api/ordercancel', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(data)
    }).finally(function () {
      location.href = '/res/' + page.dataset.userUrl + '?event-id=' + page.dataset.eventId;
    });
  });
})();
