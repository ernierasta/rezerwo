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

  var data = {rows: [], currency: '', notifications: []};
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

  // Zamówienie: wszystkie rezerwacje jednego adresu e-mail.
  function toOrders(rows) {
    var byEmail = {}, order = [];
    rows.forEach(function (r) {
      var key = r.email || ('res-' + r.id);
      if (!byEmail[key]) {
        byEmail[key] = {
          id: key,
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
      o.seats.push(r.room + ': ' + r.chair);
      o.price += r.price;
      o.mail_count = Math.max(o.mail_count, r.mail_count);
      o.last_mail = Math.max(o.last_mail, r.last_mail);
      if (r.status !== 'payed') o.status = r.status === 'ordered' ? 'ordered' : o.status;
      if (r.notes && !o.notes) o.notes = r.notes;
    });
    return order.map(function (k) {
      var o = byEmail[k];
      o.seatsText = o.seats.join(', ');
      o.seatCount = o.seats.length;
      return o;
    });
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
    el.barInfo.textContent = rows.length + ' · ' + money(price) + ' · ' + RZ.t('count.seat.many', seats).replace('%d', seats);
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
