// n_room.js - plan sali dla gościa (SVG).
// Wybór miejsc, licznik kwoty, przełączanie sal, powiększanie i wysyłka
// zamówienia do istniejącego /order (te same pola co stara strona).

(function () {
  'use strict';

  var page = document.getElementById('guest');
  if (!page) return;

  var eventID = page.dataset.eventId;
  var currency = page.dataset.currency || '';
  var defaultPrice = Number(page.dataset.price || 0);
  var hideCosts = page.dataset.hidecosts === 'true';

  var selected = []; // {number, room, price, el}

  function money(n) {
    return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, ' ') + (currency ? ' ' + currency : '');
  }

  function seatLabel(g) {
    var table = g.dataset.table;
    if (table && table !== '0') return RZ.t('guest.seat', [table, g.dataset.number]);
    return g.dataset.number;
  }

  function total() {
    return selected.reduce(function (sum, s) { return sum + s.price; }, 0);
  }

  function render() {
    var list = document.getElementById('basket-list');
    var barSum = document.getElementById('bar-sum');
    list.innerHTML = '';

    if (!selected.length) {
      var p = document.createElement('p');
      p.className = 'empty';
      p.textContent = RZ.t('guest.nothing_selected');
      list.appendChild(p);
      barSum.textContent = RZ.t('guest.nothing_selected');
      return;
    }

    selected.forEach(function (s) {
      var row = document.createElement('div');
      row.className = 'seat';
      var name = document.createElement('span');
      name.textContent = seatLabel(s.el);
      row.appendChild(name);
      if (!hideCosts) {
        var price = document.createElement('span');
        price.className = 'num';
        price.textContent = money(s.price);
        row.appendChild(price);
      }
      list.appendChild(row);
    });

    if (!hideCosts) {
      var sum = document.createElement('div');
      sum.className = 'seat sum';
      sum.innerHTML = '<span></span><span class="num"></span>';
      sum.children[0].textContent = RZ.t('guest.total');
      sum.children[1].textContent = money(total());
      list.appendChild(sum);
    }

    barSum.textContent = selected.length + (hideCosts ? '' : ' · ' + money(total()));
  }

  function toggleChair(g) {
    if (g.dataset.status !== 'free') return;
    var rect = g.querySelector('rect');
    var idx = selected.findIndex(function (s) { return s.el === g; });

    if (idx >= 0) {
      selected.splice(idx, 1);
      rect.setAttribute('class', 'chair-free');
    } else {
      var price = Number(g.dataset.price) || defaultPrice;
      selected.push({el: g, number: g.dataset.number, room: g.dataset.room, price: price});
      rect.setAttribute('class', 'chair-selected');
    }
    render();
  }

  page.addEventListener('click', function (e) {
    var chair = e.target.closest('g.chair');
    if (chair) { toggleChair(chair); return; }

    var tab = e.target.closest('#roomtabs button[data-room]');
    if (tab) {
      document.querySelectorAll('#roomtabs button').forEach(function (b) { b.classList.toggle('on', b === tab); });
      document.querySelectorAll('.plan.room').forEach(function (p) {
        p.hidden = p.dataset.room !== tab.dataset.room;
      });
      return;
    }

    var zoom = e.target.closest('[data-zoom]');
    if (zoom) {
      var plan = zoom.closest('.plan');
      var svg = plan.querySelector('svg');
      var cur = Number(svg.dataset.scale || 1);
      if (zoom.dataset.zoom === 'in') cur = Math.min(cur * 1.4, 6);
      else if (zoom.dataset.zoom === 'out') cur = Math.max(cur / 1.4, 1);
      else cur = 1;
      svg.dataset.scale = cur;
      svg.style.width = (cur * 100) + '%';
      plan.style.overflow = cur > 1 ? 'auto' : 'hidden';
    }
  });

  var noSits = document.getElementById('nosits');
  if (noSits) {
    document.getElementById('nosits-ok').addEventListener('click', function () { noSits.hidden = true; });
    noSits.addEventListener('click', function (e) { if (e.target === noSits) noSits.hidden = true; });
  }

  function order() {
    if (!selected.length) {
      // pokazujemy tekst organizatora ("Nie wybrano miejsc"), jak stara strona
      if (noSits) noSits.hidden = false;
      else RZ.toast(RZ.t('guest.nothing_selected'));
      return;
    }
    var f = document.createElement('form');
    f.method = 'POST';
    f.action = '/n/order';

    var fields = {
      'event-id': eventID,
      'sits': selected.map(function (s) { return s.number; }).join(','),
      'rooms': selected.map(function (s) { return s.room; }).join(','),
      'prices': selected.map(function (s) { return s.price; }).join(','),
      'total-price': total(),
      'default-currency': currency
    };
    Object.keys(fields).forEach(function (k) {
      var i = document.createElement('input');
      i.type = 'hidden';
      i.name = k;
      i.value = fields[k];
      f.appendChild(i);
    });
    document.body.appendChild(f);
    f.submit();
  }

  document.getElementById('order').addEventListener('click', order);
  document.getElementById('order-mobile').addEventListener('click', order);
  document.getElementById('clear').addEventListener('click', function () {
    selected.forEach(function (s) { s.el.querySelector('rect').setAttribute('class', 'chair-free'); });
    selected = [];
    render();
  });

  RZ.ready.then(render);
})();
