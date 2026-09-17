// n_reservations.js - tabela rezerwacji imprezy.
// Widok "Zamówienia" (wiersz = zamówienie jednego adresu) i "Miejsca"
// (wiersz = krzesło, jak w starym raporcie). Akcje działają na zaznaczonych
// wierszach i zmieniają tabelę dopiero po odpowiedzi serwera.

(function () {
  'use strict';

  var page = document.getElementById('res-page');
  if (!page) return;

  var eventID = page.dataset.eventId;
  var api = '/api/events/' + eventID + '/reservations';

  var el = {
    table: document.getElementById('res-table'),
    search: document.getElementById('res-search'),
    view: document.getElementById('res-view'),
    quick: document.getElementById('res-quick'),
    bar: document.getElementById('res-bar'),
    barInfo: document.getElementById('res-bar-info'),
    sum: document.getElementById('res-sum'),
    notif: document.getElementById('res-notif'),
    detail: document.getElementById('res-detail')
  };

  var data = {rows: [], currency: '', notifications: [], addons: {}};
  var view = 'orders';   // orders | seats
  var quick = 'all';     // all | ordered | payed | unpaid7 | nomail
  var table;

  function money(n) {
    return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, ' ') + (data.currency ? ' ' + data.currency : '');
  }

  function dateS(unix) {
    if (!unix || unix <= 0) return '';
    var d = new Date(unix * 1000);
    return d.getDate() + '.' + (d.getMonth() + 1) + '.' + d.getFullYear();
  }

  function statusHTML(status) {
    var cls = status === 'payed' ? 'paid' : status === 'ordered' ? 'ord' : 'off';
    return '<span class="st ' + cls + '">' + RZ.t('status.' + status) + '</span>';
  }

  // Zamówienie: rezerwacje jednego zamówienia (a gdy go nie ma - jednego adresu).
  function toOrders(rows) {
    var byEmail = {}, order = [];
    rows.forEach(function (r) {
      var key = r.order_id ? ('order-' + r.order_id) : (r.email || ('res-' + r.id));
      if (!byEmail[key]) {
        byEmail[key] = {
          id: key,
          orderID: r.order_id || 0,
          ids: [],
          person: (r.name + ' ' + r.surname).trim() || r.email,
          email: r.email,
          phone: r.phone,
          seats: [],
          price: 0,
          status: 'payed',
          ordered_date: r.ordered_date,
          payed_date: r.payed_date,
          last_mail: r.last_mail,
          mail_count: 0,
          notes: r.notes || ''
        };
        order.push(key);
      }
      var o = byEmail[key];
      o.ids.push(r.id);
      o.seats.push({room: r.room, chair: r.chair});
      o.price += r.price;
      o.mail_count = Math.max(o.mail_count, r.mail_count);
      o.last_mail = Math.max(o.last_mail, r.last_mail);
      if (r.status !== 'payed') o.status = r.status === 'ordered' ? 'ordered' : o.status;
      if (r.notes && !o.notes) o.notes = r.notes;
    });
    return order.map(function (k) {
      var o = byEmail[k];
      o.seatsText = seatsLabel(o.seats);
      o.seatCount = o.seats.length;
      return o;
    });
  }

  // seatsLabel skraca listę miejsc: nazwa sali raz, numery po przecinku,
  // ciągi zwijane do zakresu ("Balkon: 301–304, 312").
  function seatsLabel(seats) {
    var byRoom = {}, order = [];
    seats.forEach(function (s) {
      if (!byRoom[s.room]) { byRoom[s.room] = []; order.push(s.room); }
      byRoom[s.room].push(Number(s.chair));
    });

    return order.map(function (room) {
      var nums = byRoom[room].sort(function (a, b) { return a - b; });
      var parts = [], start = nums[0], prev = nums[0];

      function flush() {
        if (start === prev) parts.push(String(start));
        else if (prev === start + 1) parts.push(start + ', ' + prev);
        else parts.push(start + '–' + prev);
      }

      for (var i = 1; i < nums.length; i++) {
        if (nums[i] === prev + 1) { prev = nums[i]; continue; }
        flush();
        start = prev = nums[i];
      }
      flush();
      return room + ': ' + parts.join(', ');
    }).join(' · ');
  }

  function orderColumns() {
    return [
      {key: 'person', label: RZ.t('res.person')},
      {key: 'seatsText', label: RZ.t('res.seats')},
      {key: 'seatCount', label: '#', type: 'num', align: 'right'},
      {key: 'status', label: RZ.t('res.status'), render: function (r) { return statusHTML(r.status); }},
      {key: 'price', label: RZ.t('res.amount'), type: 'num', align: 'right', render: function (r) { return money(r.price); }},
      {key: 'email', label: 'e-mail'},
      {key: 'phone', label: 'tel.'},
      {key: 'ordered_date', label: RZ.t('status.ordered'), type: 'date', render: function (r) { return dateS(r.ordered_date); }},
      {key: 'payed_date', label: RZ.t('status.payed'), type: 'date', render: function (r) { return dateS(r.payed_date); }},
      {key: 'last_mail', label: 'mail', type: 'date', render: function (r) { return dateS(r.last_mail) + (r.mail_count ? ' (' + r.mail_count + ')' : ''); }}
    ];
  }

  function seatColumns() {
    return [
      {key: 'chair', label: RZ.t('res.seats'), type: 'num'},
      {key: 'room', label: RZ.t('field.rooms')},
      {key: 'name', label: RZ.t('res.person'), text: function (r) { return (r.name + ' ' + r.surname).trim(); },
        render: function (r) { return document.createTextNode((r.name + ' ' + r.surname).trim()); }},
      {key: 'status', label: RZ.t('res.status'), render: function (r) { return statusHTML(r.status); }},
      {key: 'price', label: RZ.t('res.amount'), type: 'num', align: 'right', render: function (r) { return money(r.price); }},
      {key: 'email', label: 'e-mail'},
      {key: 'phone', label: 'tel.'},
      {key: 'ordered_date', label: RZ.t('status.ordered'), type: 'date', render: function (r) { return dateS(r.ordered_date); }},
      {key: 'payed_date', label: RZ.t('status.payed'), type: 'date', render: function (r) { return dateS(r.payed_date); }},
      {key: 'notes', label: 'uwagi'}
    ];
  }

  function quickFilter(row) {
    var week = 7 * 24 * 3600;
    var now = Math.floor(Date.now() / 1000);
    switch (quick) {
      case 'ordered': return row.status === 'ordered';
      case 'payed': return row.status === 'payed';
      case 'unpaid7': return row.status === 'ordered' && row.ordered_date > 0 && (now - row.ordered_date) > week;
      case 'nomail': return !row.last_mail;
      default: return true;
    }
  }

  function currentRows() {
    var rows = data.rows.filter(function (r) { return r.status === 'ordered' || r.status === 'payed'; });
    return view === 'orders' ? toOrders(rows) : rows;
  }

  // ids zaznaczonych rezerwacji (w widoku zamówień jedno zaznaczenie = kilka krzeseł)
  function selectedIDs() {
    var ids = [];
    table.selected().forEach(function (r) {
      if (r.ids) ids = ids.concat(r.ids);
      else ids.push(r.id);
    });
    return ids;
  }

  function buildTable() {
    table = NTable.create(el.table, {
      columns: view === 'orders' ? orderColumns() : seatColumns(),
      rows: currentRows(),
      storageKey: 'rezerwo.res.' + view,
      rowFilter: quickFilter,
      onSelect: updateBar,
      onRowClick: showDetail,
      onRender: updateSum
    });
  }

  function updateBar(rows) {
    if (!rows.length) {
      el.bar.hidden = true;
      return;
    }
    var price = 0, seats = 0;
    rows.forEach(function (r) {
      price += r.price;
      seats += r.ids ? r.ids.length : 1;
    });
    el.bar.hidden = false;
    el.barInfo.textContent = rows.length + ' · ' + money(price) + ' · ' + RZ.t('count.seat.many', seats);
  }

  function updateSum(rows) {
    var payed = 0, waiting = 0, seats = 0;
    rows.forEach(function (r) {
      seats += r.ids ? r.ids.length : 1;
      if (r.status === 'payed') payed += r.price; else waiting += r.price;
    });
    el.sum.textContent = rows.length + ' · ' + seats + ' · ' +
      RZ.t('status.payed') + ': ' + money(payed) + ' · ' + RZ.t('res.awaiting') + ': ' + money(waiting);
  }

  function showDetail(row) {
    var ids = row.ids || [row.id];
    el.detail.hidden = false;
    el.detail.innerHTML = '';

    var h = document.createElement('h4');
    h.textContent = row.person || ((row.name || '') + ' ' + (row.surname || '')).trim();
    el.detail.appendChild(h);

    var lines = [
      [RZ.t('res.seats'), row.seatsText || (row.room + ': ' + row.chair)],
      [RZ.t('res.amount'), money(row.price)],
      [RZ.t('res.status'), RZ.t('status.' + row.status)],
      ['e-mail', row.email || ''],
      ['tel.', row.phone || ''],
      [RZ.t('status.ordered'), dateS(row.ordered_date)],
      [RZ.t('status.payed'), dateS(row.payed_date)],
      ['mail', dateS(row.last_mail) + (row.mail_count ? ' (' + row.mail_count + ')' : '')]
    ];
    lines.forEach(function (l) {
      var d = document.createElement('div');
      d.className = 'kv-row';
      d.innerHTML = '<span class="muted">' + l[0] + '</span><span>' + (l[1] || '—') + '</span>';
      el.detail.appendChild(d);
    });

    var orderID = row.orderID || row.order_id || 0;
    if (orderID) showAddons(orderID, row);

    var notes = document.createElement('textarea');
    notes.className = 'inp';
    notes.value = row.notes || '';
    el.detail.appendChild(notes);

    var save = document.createElement('button');
    save.className = 'btn sm';
    save.textContent = RZ.t('common.save');
    save.addEventListener('click', function () {
      RZ.postJSON(api + '/notes', {ids: ids, notes: notes.value}, 'PATCH')
        .then(function (resp) { setRows(resp.rows); RZ.toast(RZ.t('common.saved')); })
        .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
    });
    el.detail.appendChild(save);

    var close = document.createElement('button');
    close.className = 'btn quiet sm';
    close.textContent = RZ.t('common.collapse');
    close.addEventListener('click', function () { el.detail.hidden = true; });
    el.detail.appendChild(close);

    showHistory(ids);
  }

  // dodatki zamówione przez gościa i dopłaty dopisane przez admina
  function showAddons(orderID, row) {
    var box = document.createElement('div');
    box.className = 'addon-list';
    box.innerHTML = '<h5>' + RZ.t('addons.in_order') + '</h5>';

    var items = (data.addons && data.addons[orderID]) || [];
    if (!items.length) {
      var none = document.createElement('p');
      none.className = 'muted';
      none.textContent = RZ.t('addons.none_in_order');
      box.appendChild(none);
    }

    items.forEach(function (it) {
      var line = document.createElement('div');
      line.className = 'kv-row item';

      var name = document.createElement('span');
      name.textContent = it.description;
      line.appendChild(name);

      var right = document.createElement('span');
      right.className = 'num';
      right.textContent = money(it.amount * it.price);
      line.appendChild(right);

      var actions = document.createElement('span');
      actions.className = 'item-actions';

      var pay = document.createElement('button');
      pay.className = 'link';
      pay.textContent = it.status === 'payed' ? RZ.t('status.payed') : RZ.t('res.mark_payed');
      pay.addEventListener('click', function () {
        itemAction('/orders/' + orderID + '/items/' + it.id + '/status',
          {status: it.status === 'payed' ? 'ordered' : 'payed'}, 'POST', row);
      });
      actions.appendChild(pay);

      if (it.kind === 'surcharge') {
        var del = document.createElement('button');
        del.className = 'link';
        del.textContent = RZ.t('common.delete');
        del.addEventListener('click', function () {
          itemAction('/orders/' + orderID + '/items/' + it.id, null, 'DELETE', row);
        });
        actions.appendChild(del);
      }

      line.appendChild(actions);
      box.appendChild(line);
    });

    // dopisanie dopłaty (osobna pozycja, nie zmienia statusu miejsc)
    var form = document.createElement('div');
    form.className = 'surcharge-form';

    var desc = document.createElement('input');
    desc.className = 'inp';
    desc.placeholder = RZ.t('addons.description');

    var amount = document.createElement('input');
    amount.className = 'inp num';
    amount.type = 'number';
    amount.min = '1';
    amount.value = '1';
    amount.title = RZ.t('addons.amount');

    var price = document.createElement('input');
    price.className = 'inp num';
    price.type = 'number';
    price.min = '0';
    price.value = '0';
    price.title = RZ.t('addons.price');

    var add = document.createElement('button');
    add.className = 'btn sm';
    add.textContent = RZ.t('addons.surcharge_add');
    add.addEventListener('click', function () {
      if (!desc.value.trim()) { RZ.toast(RZ.t('addons.need_description')); return; }
      itemAction('/orders/' + orderID + '/items',
        {description: desc.value, amount: Number(amount.value) || 1, price: Number(price.value) || 0},
        'POST', row);
    });

    form.appendChild(desc);
    form.appendChild(amount);
    form.appendChild(price);
    form.appendChild(add);
    box.appendChild(form);

    el.detail.appendChild(box);
  }

  // itemAction wywołuje API pozycji i odświeża panel szczegółów
  function itemAction(path, body, method, row) {
    RZ.postJSON(api.replace('/reservations', '') + path, body, method)
      .then(function (resp) {
        if (resp.addons) data.addons = resp.addons;
        RZ.toast(RZ.t('common.saved'));
        showDetail(row);
      })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  // historia zmian statusu: kto i kiedy oznaczył miejsca jako zapłacone
  function showHistory(ids) {
    var box = document.createElement('div');
    box.className = 'history';
    box.innerHTML = '<h5>' + RZ.t('res.history') + '</h5>';
    el.detail.appendChild(box);

    fetch(api + '/history?ids=' + ids.join(','))
      .then(function (r) { return r.json(); })
      .then(function (d) {
        var rows = d.rows || [];
        if (!rows.length) {
          var p = document.createElement('p');
          p.className = 'muted';
          p.textContent = RZ.t('res.history_empty');
          box.appendChild(p);
          return;
        }
        rows.forEach(function (h) {
          var line = document.createElement('div');
          line.className = 'kv-row';
          line.innerHTML = '<span class="muted"></span><span></span>';
          line.children[0].textContent = dateS(h.date);
          line.children[1].textContent = RZ.t('status.' + h.status) + (h.admin ? ' · ' + h.admin : '');
          box.appendChild(line);
        });
      })
      .catch(function () { /* historia jest dodatkiem, brak jej nie psuje panelu */ });
  }

  function setRows(rows) {
    data.rows = rows;
    table.setRows(currentRows());
  }

  function changeStatus(status) {
    var ids = selectedIDs();
    if (!ids.length) return;
    RZ.postJSON(api + '/status', {ids: ids, status: status})
      .then(function (resp) {
        setRows(resp.rows);
        table.clearSelection();
        RZ.toast(RZ.t('common.saved'));
      })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function removeSelected() {
    var ids = selectedIDs();
    if (!ids.length) return;
    if (!confirm(RZ.t('res.delete_confirm').replace('%d', ids.length))) return;
    RZ.postJSON(api, {ids: ids}, 'DELETE')
      .then(function (resp) {
        setRows(resp.rows);
        table.clearSelection();
        RZ.toast(RZ.t('common.saved'));
      })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function sendMail() {
    var ids = selectedIDs();
    if (!ids.length) return;
    var notifID = Number(el.notif.value);
    if (!notifID) { RZ.toast(RZ.t('res.mail_no_template')); return; }
    var emails = {};
    table.selected().forEach(function (r) { if (r.email) emails[r.email] = true; });
    var count = Object.keys(emails).length;
    if (!confirm(RZ.t('res.mail_confirm').replace('%d', count))) return;

    RZ.postJSON(api + '/mail', {ids: ids, notification_id: notifID})
      .then(function (resp) {
        setRows(resp.rows);
        table.clearSelection();
        RZ.toast(resp.msg);
      })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function wire() {
    el.search.addEventListener('input', function () { table.setSearch(el.search.value); });

    el.view.addEventListener('click', function (e) {
      var b = e.target.closest('button[data-view]');
      if (!b) return;
      view = b.dataset.view;
      el.view.querySelectorAll('button').forEach(function (x) { x.classList.toggle('on', x === b); });
      el.table.innerHTML = '';
      buildTable();
    });

    el.quick.addEventListener('click', function (e) {
      var b = e.target.closest('button[data-quick]');
      if (!b) return;
      quick = b.dataset.quick;
      el.quick.querySelectorAll('button').forEach(function (x) { x.classList.toggle('on', x === b); });
      table.setRowFilter(quickFilter);
    });

    document.getElementById('act-payed').addEventListener('click', function () { changeStatus('payed'); });
    document.getElementById('act-ordered').addEventListener('click', function () { changeStatus('ordered'); });
    document.getElementById('act-mail').addEventListener('click', sendMail);
    document.getElementById('act-delete').addEventListener('click', removeSelected);
    document.getElementById('act-none').addEventListener('click', function () { table.clearSelection(); });
  }

  RZ.loadLocale(document.documentElement.lang || 'pl')
    .then(function () { return fetch(api).then(function (r) { return r.json(); }); })
    .then(function (d) {
      data.rows = d.rows || [];
      data.currency = d.currency || '';
      data.addons = d.addons || {};
      (d.notifications || []).forEach(function (n) {
        var o = document.createElement('option');
        o.value = n.id;
        o.textContent = n.name;
        el.notif.appendChild(o);
      });
      buildTable();
      wire();
    })
    .catch(function (e) {
      el.table.textContent = RZ.t('common.save_error') + ': ' + e.message;
    });
})();
