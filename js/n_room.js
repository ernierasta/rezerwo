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
      var ctl = zoom.closest('.plan').planView;
      if (!ctl) return;
      if (zoom.dataset.zoom === 'in') ctl.zoomBy(1.4);
      else if (zoom.dataset.zoom === 'out') ctl.zoomBy(1 / 1.4);
      else ctl.reset();
    }
  });

  // --- powiększanie i przesuwanie planu --------------------------------------
  // Powiększenie zmienia viewBox SVG, ramka planu zostaje tej samej wielkości.
  // Po powiększeniu plan przesuwa się palcem albo myszą we wszystkich
  // kierunkach, dwa palce przybliżają (także bez powiększenia przyciskiem),
  // Ctrl + kółko (i gest na touchpadzie) też. Przesunięcie nie wybiera krzesła.

  var MAX_SCALE = 6;
  var DRAG_PX = 6; // od tylu pikseli ruch to przesuwanie, a nie kliknięcie

  function planView(plan) {
    var svg = plan.querySelector('svg');
    if (!svg || !svg.viewBox || !svg.viewBox.baseVal) return null;
    var W = svg.viewBox.baseVal.width, H = svg.viewBox.baseVal.height;
    var s = 1, x = 0, y = 0;

    function apply() {
      var w = W / s, h = H / s;
      x = Math.min(Math.max(x, 0), W - w);
      y = Math.min(Math.max(y, 0), H - h);
      svg.setAttribute('viewBox', x + ' ' + y + ' ' + w + ' ' + h);
      plan.classList.toggle('zoomed', s > 1.001);
    }

    // punkt sali pod punktem ekranu
    function roomPoint(cx, cy) {
      var r = svg.getBoundingClientRect();
      return {x: x + (cx - r.left) / r.width * (W / s), y: y + (cy - r.top) / r.height * (H / s)};
    }

    // nowa skala, punkt sali p zostaje pod punktem ekranu (cx, cy)
    function zoomTo(ns, p, cx, cy) {
      s = Math.min(Math.max(ns, 1), MAX_SCALE);
      var r = svg.getBoundingClientRect();
      x = p.x - (cx - r.left) / r.width * (W / s);
      y = p.y - (cy - r.top) / r.height * (H / s);
      apply();
    }

    function zoomBy(f) {
      var r = svg.getBoundingClientRect();
      var cx = r.left + r.width / 2, cy = r.top + r.height / 2;
      zoomTo(s * f, roomPoint(cx, cy), cx, cy);
    }

    var pointers = {};
    var gesture = null; // {type: 'pan'|'pinch', ...}
    var dragged = false;

    function list() { return Object.keys(pointers).map(function (k) { return pointers[k]; }); }

    function startGesture() {
      var pts = list();
      if (pts.length >= 2) {
        var a = pts[0], b = pts[1];
        var mx = (a.x + b.x) / 2, my = (a.y + b.y) / 2;
        gesture = {type: 'pinch', dist: Math.hypot(a.x - b.x, a.y - b.y) || 1, s: s, p: roomPoint(mx, my)};
      } else if (pts.length === 1) {
        gesture = {type: 'pan', cx: pts[0].x, cy: pts[0].y, x: x, y: y, moving: false};
      } else {
        gesture = null;
      }
    }

    svg.addEventListener('pointerdown', function (e) {
      if (e.pointerType === 'mouse' && e.button !== 0) return;
      pointers[e.pointerId] = {x: e.clientX, y: e.clientY};
      dragged = false;
      startGesture();
    });

    svg.addEventListener('pointermove', function (e) {
      if (!pointers[e.pointerId] || !gesture) return;
      pointers[e.pointerId] = {x: e.clientX, y: e.clientY};

      if (gesture.type === 'pinch') {
        var pts = list();
        if (pts.length < 2) return;
        var a = pts[0], b = pts[1];
        var d = Math.hypot(a.x - b.x, a.y - b.y) || 1;
        dragged = true;
        zoomTo(gesture.s * d / gesture.dist, gesture.p, (a.x + b.x) / 2, (a.y + b.y) / 2);
        e.preventDefault();
        return;
      }

      // przesuwanie tylko po powiększeniu - inaczej strona przewija się normalnie
      if (s <= 1.001) return;
      var dx = e.clientX - gesture.cx, dy = e.clientY - gesture.cy;
      if (!gesture.moving) {
        if (Math.hypot(dx, dy) < DRAG_PX) return;
        gesture.moving = true;
        dragged = true;
        plan.classList.add('panning');
        try { svg.setPointerCapture(e.pointerId); } catch (err) { /* nic */ }
      }
      var r = svg.getBoundingClientRect();
      x = gesture.x - dx / r.width * (W / s);
      y = gesture.y - dy / r.height * (H / s);
      apply();
      e.preventDefault();
    });

    function up(e) {
      if (!pointers[e.pointerId]) return;
      delete pointers[e.pointerId];
      plan.classList.remove('panning');
      startGesture(); // po puszczeniu jednego palca z dwóch dalej przesuwamy
    }
    svg.addEventListener('pointerup', up);
    svg.addEventListener('pointercancel', up);
    // Na dotyku palec jest najpierw przypięty do krzesła/stołu pod nim; gdy
    // przepinamy go na cały SVG, krzesło dostaje lostpointercapture, które
    // bąbelkuje tutaj - to nie jest koniec gestu, więc liczy się tylko SVG.
    svg.addEventListener('lostpointercapture', function (e) {
      if (e.target === svg) up(e);
    });

    // kliknięcie kończące przesuwanie nie może wybrać krzesła
    svg.addEventListener('click', function (e) {
      if (dragged) {
        dragged = false;
        e.stopPropagation();
        e.preventDefault();
      }
    }, true);

    svg.addEventListener('wheel', function (e) {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      zoomTo(s * Math.exp(-e.deltaY * 0.01), roomPoint(e.clientX, e.clientY), e.clientX, e.clientY);
    }, {passive: false});

    apply();
    return {zoomBy: zoomBy, reset: function () { s = 1; x = 0; y = 0; apply(); }};
  }

  document.querySelectorAll('.plan.room').forEach(function (plan) {
    plan.planView = planView(plan);
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
    f.action = '/order';

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
