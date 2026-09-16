// n_answers.js - tabela odpowiedzi formularza.
// Ten sam komponent co tabela rezerwacji (n_table.js); kolumny pochodzą z pól
// formularza, więc powstają dopiero po wczytaniu danych.

(function () {
  'use strict';

  var page = document.getElementById('ans-page');
  if (!page) return;

  var formID = page.dataset.formId;
  var api = '/api/forms/' + formID + '/answers';

  var el = {
    table: document.getElementById('ans-table'),
    search: document.getElementById('ans-search'),
    quick: document.getElementById('ans-quick'),
    cols: document.getElementById('ans-cols'),
    notif: document.getElementById('ans-notif'),
    bar: document.getElementById('ans-bar'),
    barInfo: document.getElementById('ans-bar-info'),
    sum: document.getElementById('ans-sum'),
    detail: document.getElementById('ans-detail')
  };

  var data = {rows: [], columns: [], moneyField: ''};
  var quick = 'all';
  var table;

  function dateS(unix) {
    if (!unix || unix <= 0) return '';
    var d = new Date(unix * 1000);
    return d.getDate() + '.' + (d.getMonth() + 1) + '.' + d.getFullYear();
  }

  function statusHTML(status) {
    if (!status) return '';
    var cls = status === 'payed' ? 'paid' : status === 'ordered' ? 'ord' : 'off';
    var label = RZ.t('status.' + status);
    return '<span class="st ' + cls + '">' + (label === 'status.' + status ? status : label) + '</span>';
  }

  function columns() {
    var cols = [
      {key: 'name', label: RZ.t('res.person'), text: function (r) { return (r.name + ' ' + r.surname).trim(); },
        render: function (r) { return document.createTextNode((r.name + ' ' + r.surname).trim()); }},
      {key: 'email', label: 'e-mail'},
      {key: 'status', label: RZ.t('res.status'), render: function (r) { return statusHTML(r.status); }},
      {key: 'created', label: RZ.t('form.created'), type: 'date', render: function (r) { return dateS(r.created); }},
      {key: 'last_mail', label: 'mail', type: 'date',
        render: function (r) { return dateS(r.last_mail) + (r.mail_count ? ' (' + r.mail_count + ')' : ''); }}
    ];
    // kolumny z pól formularza
    data.columns.forEach(function (c) {
      cols.push({
        key: 'v_' + c.key,
        label: c.label,
        text: function (r) { return r.values[c.key] || ''; },
        render: function (r) { return document.createTextNode(r.values[c.key] || ''); }
      });
    });
    return cols;
  }

  function quickFilter(row) {
    switch (quick) {
      case 'payed': return row.status === 'payed';
      case 'unpaid': return row.status !== 'payed';
      case 'nomail': return !row.last_mail;
      default: return true;
    }
  }

  function buildColumnChooser(cols) {
    el.cols.innerHTML = '';
    cols.forEach(function (c) {
      var lbl = document.createElement('label');
      lbl.className = 'colopt';
      var box = document.createElement('input');
      box.type = 'checkbox';
      box.checked = !table.hiddenColumns()[c.key];
      box.addEventListener('change', function () { table.toggleColumn(c.key, !box.checked); });
      lbl.appendChild(box);
      lbl.appendChild(document.createTextNode(' ' + c.label));
      el.cols.appendChild(lbl);
    });
  }

  function updateBar(rows) {
    if (!rows.length) { el.bar.hidden = true; return; }
    el.bar.hidden = false;
    var money = sumMoney(rows);
    el.barInfo.textContent = rows.length + (money ? ' · ' + money : '');
  }

  function sumMoney(rows) {
    if (!data.moneyField) return '';
    var total = 0;
    rows.forEach(function (r) {
      var v = parseInt(String(r.values[data.moneyField] || '').replace(/[^\d-]/g, ''), 10);
      if (!isNaN(v)) total += v;
    });
    if (!total) return '';
    return String(total).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
  }

  function updateSum(rows) {
    var money = sumMoney(rows);
    el.sum.textContent = RZ.t('count.answer.many', rows.length) + (money ? ' · ' + money : '');
  }

  function showDetail(row) {
    el.detail.hidden = false;
    el.detail.innerHTML = '';

    var h = document.createElement('h4');
    h.textContent = (row.name + ' ' + row.surname).trim() || row.email;
    el.detail.appendChild(h);

    function line(k, v) {
      var d = document.createElement('div');
      d.className = 'kv-row';
      d.innerHTML = '<span class="muted"></span><span></span>';
      d.children[0].textContent = k;
      d.children[1].textContent = v || '—';
      el.detail.appendChild(d);
    }

    line('e-mail', row.email);
    line(RZ.t('res.status'), row.status ? RZ.t('status.' + row.status) : '');
    line(RZ.t('form.created'), dateS(row.created));
    line('mail', dateS(row.last_mail) + (row.mail_count ? ' (' + row.mail_count + ')' : ''));
    data.columns.forEach(function (c) { line(c.label, row.values[c.key]); });

    var close = document.createElement('button');
    close.className = 'btn quiet sm';
    close.textContent = RZ.t('common.collapse');
    close.addEventListener('click', function () { el.detail.hidden = true; });
    el.detail.appendChild(close);
  }

  function ids() {
    return table.selected().map(function (r) { return r.id; });
  }

  function changeStatus(status) {
    if (!ids().length) return;
    RZ.postJSON(api + '/status', {ids: ids(), status: status})
      .then(function (resp) { data.rows = resp.rows; table.setRows(resp.rows); table.clearSelection(); RZ.toast(RZ.t('common.saved')); })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function removeSelected() {
    var list = ids();
    if (!list.length) return;
    if (!confirm(RZ.t('res.delete_confirm').replace('%d', list.length))) return;
    RZ.postJSON(api, {ids: list}, 'DELETE')
      .then(function (resp) { data.rows = resp.rows; table.setRows(resp.rows); table.clearSelection(); RZ.toast(RZ.t('common.saved')); })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function sendMail() {
    var list = ids();
    if (!list.length) return;
    var notifID = Number(el.notif.value);
    if (!notifID) { RZ.toast(RZ.t('res.mail_no_template')); return; }
    if (!confirm(RZ.t('res.mail_confirm').replace('%d', list.length))) return;
    RZ.postJSON(api + '/mail', {ids: list, notification_id: notifID})
      .then(function (resp) { data.rows = resp.rows; table.setRows(resp.rows); table.clearSelection(); RZ.toast(resp.msg); })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function wire(cols) {
    el.search.addEventListener('input', function () { table.setSearch(el.search.value); });
    el.quick.addEventListener('click', function (e) {
      var b = e.target.closest('button[data-quick]');
      if (!b) return;
      quick = b.dataset.quick;
      el.quick.querySelectorAll('button').forEach(function (x) { x.classList.toggle('on', x === b); });
      table.setRowFilter(quickFilter);
    });
    document.getElementById('ans-payed').addEventListener('click', function () { changeStatus('payed'); });
    document.getElementById('ans-ordered').addEventListener('click', function () { changeStatus('ordered'); });
    document.getElementById('ans-mail').addEventListener('click', sendMail);
    document.getElementById('ans-delete').addEventListener('click', removeSelected);
    document.getElementById('ans-none').addEventListener('click', function () { table.clearSelection(); });
    buildColumnChooser(cols);
  }

  RZ.loadLocale(document.documentElement.lang || 'pl')
    .then(function () { return fetch(api).then(function (r) { return r.json(); }); })
    .then(function (d) {
      data.rows = d.rows || [];
      data.columns = d.columns || [];
      data.moneyField = d.money_field || '';
      (d.notifications || []).forEach(function (n) {
        var o = document.createElement('option');
        o.value = n.id;
        o.textContent = n.name;
        el.notif.appendChild(o);
      });
      var cols = columns();
      table = NTable.create(el.table, {
        columns: cols,
        rows: data.rows,
        storageKey: 'rezerwo.answers.' + formID,
        rowFilter: quickFilter,
        onSelect: updateBar,
        onRowClick: showDetail,
        onRender: updateSum
      });
      wire(cols);
    })
    .catch(function (e) { el.table.textContent = RZ.t('common.save_error') + ': ' + e.message; });
})();
