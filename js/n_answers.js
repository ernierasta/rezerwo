// n_answers.js - tabela odpowiedzi formularza.
// Ten sam komponent co tabela rezerwacji (n_table.js); kolumny pochodzą z pól
// formularza, więc powstają dopiero po wczytaniu danych.
//
// Formularze z warunkami (formconds.go): serwer liczy dla każdego zgłoszenia,
// których pól gość nie widział (row.na) i które są wypełnione mimo ukrycia
// (row.out), patrz form_report.go. Stąd:
//   - "n/d" w komórkach zamiast pustych, kolumny pogrupowane w sekcje,
//   - filtr gałęzi (wartość pola, od którego coś zależy); kolumny, których
//     nikt z pokazanych nie widział, same się chowają,
//   - zestawienie liczone od tych, którzy pole widzieli, pola zależne
//     w grupach wg wyboru rodzica,
//   - CSV widoku: tylko pokazane wiersze i kolumny.

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
    detail: document.getElementById('ans-detail'),
    pane: document.getElementById('ans-pane'),
    branch: document.getElementById('ans-branch'),
    view: document.getElementById('ans-view'),
    summary: document.getElementById('ans-summary'),
    csvView: document.getElementById('ans-csv-view')
  };

  var data = {rows: [], columns: [], sections: [], moneyField: ''};
  var colByKey = {};
  var quick = 'all';
  var branch = null;            // {key, value} - filtr gałęzi
  var viewKey = 'rezerwo.answers.view.' + formID;
  var view = 'table';
  try { if (localStorage.getItem(viewKey) === 'summary') view = 'summary'; } catch (e) { /* prywatne okno */ }
  var table;
  var currentDetailID = null;   // id odpowiedzi pokazanej w panelu szczegółów
  var paneKey = 'rezerwo.pane';  // wspólne z raportem rezerwacji
  var paneOn = true;            // czy boczna kolumna ze szczegółami jest włączona
  try { paneOn = localStorage.getItem(paneKey) !== '0'; } catch (e) { /* prywatne okno */ }

  function dateS(unix) {
    if (!unix || unix <= 0) return '';
    var d = new Date(unix * 1000);
    return d.getDate() + '.' + (d.getMonth() + 1) + '.' + d.getFullYear();
  }

  function mailText(r) { return dateS(r.last_mail) + (r.mail_count ? ' (' + r.mail_count + ')' : ''); }

  function statusHTML(status) {
    if (!status) return '';
    var cls = status === 'payed' ? 'paid' : status === 'ordered' ? 'ord' : 'off';
    var label = RZ.t('status.' + status);
    return '<span class="st ' + cls + '">' + (label === 'status.' + status ? status : label) + '</span>';
  }

  // prep dokłada do wierszy zbiory pól "n/d" i "poza warunkiem".
  function prep(rows) {
    rows.forEach(function (r) {
      r._na = {};
      r._out = {};
      (r.na || []).forEach(function (k) { r._na[k] = true; });
      (r.out || []).forEach(function (k) { r._out[k] = true; });
      r.picks = r.picks || {};
    });
    return rows;
  }

  function sectionLabel(i) {
    return i >= 0 && data.sections[i] ? data.sections[i].label : '';
  }

  function columns() {
    var cols = [
      {key: 'name', label: RZ.t('res.person'), text: function (r) { return (r.name + ' ' + r.surname).trim(); },
        render: function (r) { return document.createTextNode((r.name + ' ' + r.surname).trim()); }},
      {key: 'email', label: 'e-mail'},
      {key: 'status', label: RZ.t('res.status'), render: function (r) { return statusHTML(r.status); },
        text: function (r) { return r.status ? RZ.t('status.' + r.status) : ''; }},
      {key: 'created', label: RZ.t('form.created'), type: 'date', text: function (r) { return dateS(r.created); },
        render: function (r) { return dateS(r.created); }},
      {key: 'last_mail', label: 'mail', type: 'date', text: mailText, render: mailText}
    ];
    var base = RZ.t('answers.base');
    cols.forEach(function (c) { c.group = base; });
    // kolumny z pól formularza, pogrupowane w sekcje; imię, nazwisko
    // i e-mail są już w kolumnach zgłoszenia
    data.columns.forEach(function (c) {
      if (c.role) return;
      cols.push({
        key: 'v_' + c.key,
        field: c,
        label: c.label,
        group: sectionLabel(c.section) || ' ',
        text: function (r) { return r._na[c.key] ? '' : (r.values[c.key] || ''); },
        render: function (r) {
          if (r._na[c.key] && !r.values[c.key]) return document.createTextNode(RZ.t('answers.na'));
          return document.createTextNode(r.values[c.key] || '');
        },
        cellClass: function (r) { return r._out[c.key] ? 'out' : r._na[c.key] ? 'na' : ''; },
        cellTitle: function (r) {
          return r._out[c.key] ? RZ.t('answers.out_title') : r._na[c.key] ? RZ.t('answers.na_title') : '';
        }
      });
    });
    return cols;
  }

  // autoHide chowa kolumnę pola, którego nie widział nikt z pokazanych wierszy.
  function autoHide(col, rows) {
    if (!col.field) return false;
    var k = col.field.key;
    return rows.every(function (r) { return r._na[k]; });
  }

  function branchFilter(row) {
    if (!branch) return true;
    return (row.picks[branch.key] || []).indexOf(branch.value) >= 0;
  }

  function rowFilter(row) {
    return quickFilter(row) && branchFilter(row);
  }

  function quickFilter(row) {
    switch (quick) {
      case 'payed': return row.status === 'payed';
      case 'unpaid': return row.status !== 'payed';
      case 'nomail': return !row.last_mail;
      default: return true;
    }
  }

  // buildBranches - filtr gałęzi: pola, od których zależy jakiś warunek albo
  // opcje innego pola; jedna wartość naraz.
  function buildBranches() {
    var used = {};
    data.columns.forEach(function (c) {
      if (c.cond) used[c.cond.field] = true;
      if (c.parent) used[c.parent] = true;
    });
    data.sections.forEach(function (sec) { if (sec.cond) used[sec.cond.field] = true; });
    var any = false;
    data.columns.forEach(function (c) {
      if (!used[c.key] || !c.options) return;
      var grp = document.createElement('optgroup');
      grp.label = c.label;
      var seen = {};
      c.options.forEach(function (o) {
        if (seen[o.value]) return;
        seen[o.value] = true;
        var opt = document.createElement('option');
        opt.value = JSON.stringify([c.key, o.value]);
        opt.textContent = c.label + ': ' + o.label;
        grp.appendChild(opt);
      });
      el.branch.appendChild(grp);
      any = true;
    });
    el.branch.hidden = !any;
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
      // pole "ilość × cena" zapisuje się jako "2 * 250 = 500" - liczy się kwota po "="
      var s = String(r.values[data.moneyField] || '');
      if (s.indexOf('=') >= 0) s = s.slice(s.lastIndexOf('=') + 1);
      var v = parseInt(s.replace(/[^\d-]/g, ''), 10);
      if (!isNaN(v)) total += v;
    });
    if (!total) return '';
    return String(total).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
  }

  function updateSum(rows) {
    var money = sumMoney(rows);
    el.sum.textContent = RZ.t('count.answer.many', rows.length) + (money ? ' · ' + money : '');
  }

  // showDetail - panel z tym, co gość wpisał (puste pola pominięte);
  // false = panel wyłączony, kliknięcie wiersza go zaznacza (n_table.js)
  function showDetail(row) {
    if (!paneOn || view !== 'table') return false;
    currentDetailID = row.id;
    el.detail.hidden = false;
    el.detail.innerHTML = '';

    var head = document.createElement('div');
    head.className = 'pane-h';

    var h = document.createElement('h4');
    h.textContent = (row.name + ' ' + row.surname).trim() || row.email;
    head.appendChild(h);

    var x = document.createElement('button');
    x.type = 'button';
    x.className = 'pane-x';
    x.textContent = '×';
    x.title = RZ.t('common.close');
    x.setAttribute('aria-label', RZ.t('common.close'));
    x.addEventListener('click', hideDetail);
    head.appendChild(x);
    el.detail.appendChild(head);

    function line(k, v) {
      var d = document.createElement('div');
      d.className = 'kv-row';
      d.innerHTML = '<span class="muted"></span><span></span>';
      d.children[0].textContent = k;
      d.children[1].textContent = v;
      el.detail.appendChild(d);
    }

    function filled(c) { return String(row.values[c.key] || '').trim() !== ''; }

    if (row.email) line('e-mail', row.email);

    // pola w sekcjach formularza, tylko wypełnione; sekcja bez nich znika
    var bySec = [];
    data.columns.forEach(function (c) {
      if (c.role) return;   // imię, nazwisko i e-mail są w nagłówku panelu
      var last = bySec[bySec.length - 1];
      if (!last || last.section !== c.section) {
        last = {section: c.section, cols: []};
        bySec.push(last);
      }
      last.cols.push(c);
    });
    bySec.forEach(function (g) {
      var label = sectionLabel(g.section);
      var shown = g.cols.filter(filled);
      if (!shown.length) return;
      if (label) {
        var h = document.createElement('h5');
        h.className = 'sec-h';
        h.textContent = label;
        el.detail.appendChild(h);
      }
      shown.forEach(function (c) {
        line(c.label, row.values[c.key]);
        if (row._out[c.key]) {
          var v = el.detail.lastChild.children[1];
          v.className = 'out';
          v.title = RZ.t('answers.out_title');
        }
      });
    });
  }

  function hideDetail() {
    currentDetailID = null;
    el.detail.hidden = true;
  }

  // refreshDetail odtwarza otwarty panel po zmianie danych (np. statusu), żeby
  // nie trzeba było klikać wiersza ponownie.
  function refreshDetail() {
    if (currentDetailID === null || !paneOn) return;
    var row = null;
    data.rows.forEach(function (r) { if (r.id === currentDetailID) row = r; });
    if (row) showDetail(row);
    else hideDetail();
  }

  // --- zestawienie ----------------------------------------------------------

  function num(s) {
    var v = parseFloat(String(s || '').replace(/\s/g, '').replace(',', '.'));
    return isNaN(v) ? 0 : v;
  }

  function fmtNum(v) {
    var s = (Math.round(v * 100) / 100).toFixed(v % 1 ? 2 : 0);
    return s.replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
  }

  function node(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text;
    return n;
  }

  // optionRows liczy wybory opcji pola wśród wierszy, które je widziały.
  // Pole zależne: każda grupa "when" osobno - opcja liczy się w grupie, gdy
  // rodzic ma tę wartość (jak countOptions w form_stats.go).
  function optionRows(c, seen) {
    var dep = !!c.parent;
    var opts = [];
    var known = {};
    (c.options || []).forEach(function (o) {
      known[o.value] = true;
      if (!dep) {
        if (opts.some(function (x) { return x.value === o.value; })) return;
        opts.push({value: o.value, label: o.label, when: '', n: 0});
      } else {
        opts.push({value: o.value, label: o.label, when: o.when || '', n: 0});
      }
    });
    var other = {}, otherOrder = [];
    seen.forEach(function (r) {
      var picks = r.picks[c.key] || [];
      var parent = dep ? (r.picks[c.parent] || []) : [];
      picks.forEach(function (v) {
        if (!known[v]) {
          if (!other[v]) { other[v] = 0; otherOrder.push(v); }
          other[v]++;
          return;
        }
        opts.forEach(function (o) {
          if (o.value === v && (!o.when || parent.indexOf(o.when) >= 0)) o.n++;
        });
      });
    });
    otherOrder.forEach(function (v) { opts.push({value: v, label: v, when: '', n: other[v], other: true}); });
    return opts;
  }

  // optionList - opcje z liczbą i procentem od total; przy długiej liście
  // (np. klasy 1A-9B) opcje bez wyborów zwijają się do jednej linii.
  function optionList(opts, total) {
    var ul = node('ul');
    var zero = opts.filter(function (o) { return !o.n; });
    if (opts.length > 8 && zero.length) {
      opts = opts.filter(function (o) { return o.n; });
    } else {
      zero = [];
    }
    opts.forEach(function (o) {
      var li = node('li', o.other ? 'other' : '');
      var p = total ? Math.round(o.n * 100 / total) : 0;
      li.appendChild(node('span', '', o.label));
      li.appendChild(node('b', 'num', String(o.n)));
      li.appendChild(node('small', 'num', p + '%'));
      var bar = node('i', 'bar');
      var fill = node('i');
      fill.style.width = Math.min(p, 100) + '%';
      bar.appendChild(fill);
      li.appendChild(bar);
      ul.appendChild(li);
    });
    if (zero.length) {
      var rest = node('li', 'zero');
      rest.appendChild(node('span', 'muted', zero.map(function (o) { return o.label; }).join(', ')));
      rest.appendChild(node('b', 'num muted', '0'));
      ul.appendChild(rest);
    }
    return ul;
  }

  function summaryCard(c, rows) {
    if (c.role) return null;
    var seen = rows.filter(function (r) { return !r._na[c.key]; });
    if (!seen.length) return null;
    var card = node('div', 'card');
    card.appendChild(node('div', 'lbl', c.label));
    var seenNote = seen.length < rows.length ? RZ.t('answers.seen', [seen.length, rows.length]) : '';
    var filled = seen.filter(function (r) { return String(r.values[c.key] || '').trim() !== ''; });

    if (c.options) {
      if (seenNote) card.appendChild(node('div', 'seen', seenNote));
      var opts = optionRows(c, seen);
      if (!c.parent) {
        card.appendChild(optionList(opts, seen.length));
        return card;
      }
      // grupy wg wyboru rodzica, etykiety z opcji rodzica
      var parent = colByKey[c.parent];
      var plabel = {};
      ((parent && parent.options) || []).forEach(function (o) { if (!plabel[o.value]) plabel[o.value] = o.label; });
      var groups = [], byWhen = {}, common = [];
      opts.forEach(function (o) {
        if (!o.when) { common.push(o); return; }
        if (!byWhen[o.when]) {
          byWhen[o.when] = {when: o.when, opts: []};
          groups.push(byWhen[o.when]);
        }
        byWhen[o.when].opts.push(o);
      });
      groups.forEach(function (g) {
        // podstawa procentów: ci, którzy widzieli pole i wybrali tę wartość rodzica
        var base = seen.filter(function (r) { return (r.picks[c.parent] || []).indexOf(g.when) >= 0; }).length;
        if (!base) return;
        card.appendChild(node('div', 'opt-grp', (plabel[g.when] || g.when) + ' (' + base + ')'));
        card.appendChild(optionList(g.opts, base));
      });
      if (common.some(function (o) { return o.n; })) {
        if (groups.length) card.appendChild(node('div', 'opt-grp', RZ.t('answers.common_opts')));
        card.appendChild(optionList(common, seen.length));
      }
      return card;
    }

    var filledNote = RZ.t('answers.filled', [filled.length, seen.length]);
    var notes = function () {
      card.appendChild(node('div', 'seen', filledNote + (seenNote ? ' · ' + seenNote : '')));
    };
    if (c.type === 'number' || c.type === 'multiplierField') {
      var sum = 0, money = 0;
      filled.forEach(function (r) {
        var v = String(r.values[c.key]);
        if (c.type === 'multiplierField') {
          // "2 * 250 = 500": ilość przed "*", kwota po "="
          sum += num(v.split('*')[0]);
          if (v.indexOf('=') >= 0) money += num(v.slice(v.lastIndexOf('=') + 1));
        } else {
          sum += num(v);
        }
      });
      card.appendChild(node('div', 'big num', RZ.t('answers.sum') + ': ' + fmtNum(sum)));
      if (money) card.appendChild(node('div', 'seen', RZ.t('answers.money') + ': ' + fmtNum(money)));
      notes();
      return card;
    }
    card.appendChild(node('div', 'big num', String(filled.length)));
    notes();
    return card;
  }

  function renderSummary(rows) {
    el.summary.innerHTML = '';
    if (!rows.length) {
      el.summary.appendChild(node('p', 'muted', RZ.t('answers.empty_view')));
      return;
    }
    var lastSec = null;
    var pendingHead = null;
    data.columns.forEach(function (c) {
      if (c.section !== lastSec) {
        lastSec = c.section;
        var label = sectionLabel(c.section);
        pendingHead = label ? node('h4', '', label) : null;
      }
      var card = summaryCard(c, rows);
      if (!card) return;
      if (pendingHead) {
        el.summary.appendChild(pendingHead);
        pendingHead = null;
      }
      el.summary.appendChild(card);
    });
  }

  function setView(v) {
    view = v;
    try { localStorage.setItem(viewKey, v); } catch (e) { /* ignorujemy */ }
    el.view.querySelectorAll('button').forEach(function (b) { b.classList.toggle('on', b.dataset.view === v); });
    el.table.hidden = v !== 'table';
    el.summary.hidden = v !== 'summary';
    if (v === 'summary') {
      hideDetail();
      renderSummary(table.visibleRows());
    }
  }

  function onRender(rows) {
    updateSum(rows);
    if (view === 'summary') renderSummary(rows);
  }

  // csvViewURL - CSV z tym, co widać: wiersze po filtrach, kolumny pól
  // narysowane w tabeli (bez ukrytych i schowanych), ukryte pola jako "n/d".
  function csvViewURL() {
    var rowIDs = table.visibleRows().map(function (r) { return r.id; });
    var keys = table.shownColumns().filter(function (c) { return c.field; }).map(function (c) { return c.field.key; });
    return '/api/forms/' + formID + '/answers.csv?na=1' +
      '&ids=' + encodeURIComponent(rowIDs.join(',') || '0') +
      '&cols=' + encodeURIComponent(keys.join(',') || '-');
  }

  function ids() {
    return table.selected().map(function (r) { return r.id; });
  }

  function changeStatus(status) {
    if (!ids().length) return;
    RZ.postJSON(api + '/status', {ids: ids(), status: status})
      .then(function (resp) {
        data.rows = prep(resp.rows || []);
        table.setRows(data.rows);
        table.clearSelection();
        refreshDetail();
        RZ.toast(RZ.t('common.saved'));
      })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function removeSelected() {
    var list = ids();
    if (!list.length) return;
    if (!confirm(RZ.t('answers.delete_confirm', list.length))) return;
    RZ.postJSON(api, {ids: list}, 'DELETE')
      .then(function (resp) {
        data.rows = prep(resp.rows || []);
        table.setRows(data.rows);
        table.clearSelection();
        refreshDetail();
        RZ.toast(RZ.t('common.saved'));
      })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function sendMail() {
    var list = ids();
    if (!list.length) return;
    var notifID = Number(el.notif.value);
    if (!notifID) { RZ.toast(RZ.t('res.mail_no_template')); return; }
    if (!confirm(RZ.t('res.mail_confirm').replace('%d', list.length))) return;
    RZ.postJSON(api + '/mail', {ids: list, notification_id: notifID})
      .then(function (resp) {
        data.rows = prep(resp.rows || []);
        table.setRows(data.rows);
        table.clearSelection();
        refreshDetail();
        RZ.toast(resp.msg);
      })
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function wire() {
    el.search.addEventListener('input', function () { table.setSearch(el.search.value); });
    el.quick.addEventListener('click', function (e) {
      var b = e.target.closest('button[data-quick]');
      if (!b) return;
      quick = b.dataset.quick;
      el.quick.querySelectorAll('button').forEach(function (x) { x.classList.toggle('on', x === b); });
      table.setRowFilter(rowFilter);
    });
    el.branch.addEventListener('change', function () {
      var v = el.branch.value;
      branch = null;
      if (v) {
        var kv = JSON.parse(v);
        branch = {key: kv[0], value: kv[1]};
      }
      table.setRowFilter(rowFilter);
    });
    el.view.addEventListener('click', function (e) {
      var b = e.target.closest('button[data-view]');
      if (b) setView(b.dataset.view);
    });
    el.csvView.addEventListener('click', function (e) {
      e.preventDefault();
      location.href = csvViewURL();
    });
    document.getElementById('ans-payed').addEventListener('click', function () { changeStatus('payed'); });
    document.getElementById('ans-ordered').addEventListener('click', function () { changeStatus('ordered'); });
    document.getElementById('ans-mail').addEventListener('click', sendMail);
    document.getElementById('ans-delete').addEventListener('click', removeSelected);
    document.getElementById('ans-none').addEventListener('click', function () { table.clearSelection(); });
    if (el.pane) {
      el.pane.checked = paneOn;
      el.pane.addEventListener('change', function () {
        paneOn = el.pane.checked;
        try { localStorage.setItem(paneKey, paneOn ? '1' : '0'); } catch (e) { /* ignorujemy */ }
        if (!paneOn) hideDetail();
        else refreshDetail();
      });
    }
    table.chooser(el.cols);
  }

  RZ.loadLocale(document.documentElement.lang || 'pl')
    .then(function () { return fetch(api).then(function (r) { return r.json(); }); })
    .then(function (d) {
      data.rows = prep(d.rows || []);
      data.columns = d.columns || [];
      data.sections = d.sections || [];
      data.columns.forEach(function (c) { colByKey[c.key] = c; });
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
        rowFilter: rowFilter,
        autoHide: autoHide,
        onSelect: updateBar,
        onRowClick: showDetail,
        onRender: onRender
      });
      buildBranches();
      wire();
      setView(view);
    })
    .catch(function (e) { el.table.textContent = RZ.t('common.save_error') + ': ' + e.message; });
})();
