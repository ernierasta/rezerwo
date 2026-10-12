// n_table.js - własny komponent tabeli (bez jQuery i DataTables).
// Jeden komponent dla rezerwacji i odpowiedzi formularzy: sortowanie,
// wyszukiwanie, zaznaczanie, wybór kolumn i zapamiętany widok.
//
// użycie:
//   var t = NTable.create(el, {
//     columns: [{key:'name', label:'Imię', type:'text'|'num'|'date'|'status', render:fn}],
//     rows: [...],
//     storageKey: 'rezerwo.res.22',
//     onSelect: function (rows) {},
//     onRowClick: function (row) {},     // false = kliknięcie zaznacza wiersz
//     autoHide: function (col, rows) {}  // true = schowaj kolumnę przy tych wierszach
//   });
// kolumna może mieć też group (nagłówek nad kolumnami - dwa poziomy głowicy)
// i cellClass(row) / cellTitle(row) dla komórki.
//   t.setRows(rows); t.selected(); t.visibleRows(); t.chooser(el);
// Pod nagłówkami jest wiersz filtrów: tekst w polu kolumny zawęża wiersze do
// tych, których komórka go zawiera (bez wielkości liter). Filtry nie są
// zapamiętywane - otwarty raport zawsze pokazuje wszystkie wiersze.
// Nagłówek przeciągnięty na inny zmienia kolejność kolumn, prawa krawędź
// nagłówka - szerokość (dwuklik: szerokość z treści). Kolejność, szerokości,
// schowane kolumny i sortowanie zapamiętuje przeglądarka pod storageKey.
// Zaznaczanie jak w liście plików: Ctrl+klik przełącza wiersz, Shift+klik
// (też na polu wyboru) zaznacza zakres od ostatnio klikniętego, Ctrl+A
// wszystkie pokazane wiersze. Zwykły klik otwiera szczegóły (onRowClick),
// a gdy ich nie ma - zaznacza wiersz.

(function (global) {
  'use strict';

  var MINW = 40;   // najwęższa kolumna przy zmianie szerokości

  function create(root, opts) {
    var state = {
      columns: opts.columns,
      rows: opts.rows || [],
      sort: {key: null, dir: 1},
      search: '',
      filters: {},           // klucz kolumny -> tekst
      hidden: {},            // klucz kolumny -> true
      order: null,           // klucze kolumn w kolejności użytkownika (null = domyślna)
      widths: {},            // klucz kolumny -> szerokość w px
      selected: {},          // id wiersza -> true
      idKey: opts.idKey || 'id',
      shown: [],             // kolumny narysowane ostatnio (bez ukrytych i schowanych)
      anchor: null           // id ostatnio klikniętego wiersza (początek zakresu dla Shift)
    };
    var chooserEl = null;

    // zapamiętany widok: kolumny, sortowanie
    if (opts.storageKey) {
      try {
        var saved = JSON.parse(localStorage.getItem(opts.storageKey) || '{}');
        if (saved.sort) state.sort = saved.sort;
        if (saved.hidden) state.hidden = saved.hidden;
        if (Array.isArray(saved.order)) state.order = saved.order;
        if (saved.widths) state.widths = saved.widths;
      } catch (e) { /* prywatne okno */ }
    }

    function storeView() {
      if (!opts.storageKey) return;
      try {
        localStorage.setItem(opts.storageKey, JSON.stringify({
          sort: state.sort, hidden: state.hidden, order: state.order, widths: state.widths
        }));
      } catch (e) { /* ignorujemy */ }
    }

    function cellText(row, col) {
      // kolumny wyliczane (pola formularza: klucz v_...) nie mają wartości
      // pod swoim kluczem - ich tekst daje col.text
      if (col.text) return String(col.text(row) || '');
      var v = row[col.key];
      if (v === null || v === undefined) return '';
      return String(v);
    }

    // squash - filtr kolumny pomija wielkość liter i spacje ("4500" znajdzie "4 500 Kč")
    function squash(s) { return s.toLowerCase().replace(/\s+/g, ''); }

    function matches(row) {
      if (state.search) {
        var hay = state.columns.map(function (c) { return cellText(row, c); }).join(' ').toLowerCase();
        if (hay.indexOf(state.search.toLowerCase()) === -1) return false;
      }
      for (var key in state.filters) {
        var f = state.filters[key];
        if (!f || state.hidden[key]) continue;   // filtr schowanej kolumny nie działa
        var col = state.columns.filter(function (c) { return c.key === key; })[0];
        if (!col) continue;
        if (squash(cellText(row, col)).indexOf(squash(f)) === -1) return false;
      }
      if (opts.rowFilter && !opts.rowFilter(row)) return false;
      return true;
    }

    function visibleRows() {
      var rows = state.rows.filter(matches);
      if (state.sort.key) {
        var col = state.columns.filter(function (c) { return c.key === state.sort.key; })[0];
        rows.sort(function (a, b) {
          var av = a[state.sort.key], bv = b[state.sort.key];
          // kolumny wyliczane (np. pola formularza) sortują się po swoim tekście
          if (col && col.text && col.type !== 'num' && col.type !== 'date') { av = col.text(a); bv = col.text(b); }
          if (col && (col.type === 'num' || col.type === 'date')) {
            return (Number(av) - Number(bv)) * state.sort.dir;
          }
          return String(av === null || av === undefined ? '' : av)
            .localeCompare(String(bv === null || bv === undefined ? '' : bv), 'pl') * state.sort.dir;
        });
      }
      return rows;
    }

    // orderedColumns - kolumny w kolejności użytkownika; kolumna, której nie
    // ma w zapisanej kolejności (np. nowe pole formularza), staje za swoją
    // poprzedniczką z kolejności domyślnej
    function orderedColumns() {
      if (!state.order) return state.columns;
      var byKey = {};
      state.columns.forEach(function (c) { byKey[c.key] = c; });
      var keys = state.order.filter(function (k) { return byKey[k]; });
      state.columns.forEach(function (c, i) {
        if (keys.indexOf(c.key) >= 0) return;
        var at = i ? keys.indexOf(state.columns[i - 1].key) + 1 : 0;
        keys.splice(at, 0, c.key);
      });
      return keys.map(function (k) { return byKey[k]; });
    }

    function moveColumn(key, target, after) {
      if (key === target) return;
      var keys = orderedColumns().map(function (c) { return c.key; });
      keys.splice(keys.indexOf(key), 1);
      keys.splice(keys.indexOf(target) + (after ? 1 : 0), 0, key);
      state.order = keys;
      storeView();
      render();
    }

    function selectedRows() {
      return state.rows.filter(function (r) { return state.selected[r[state.idKey]]; });
    }

    function notifySelect() {
      if (opts.onSelect) opts.onSelect(selectedRows());
    }

    // syncSelection przenosi zaznaczenie na narysowaną tabelę bez rysowania od nowa
    function syncSelection() {
      var all = root.querySelectorAll('tbody tr');
      var on = 0;
      [].forEach.call(all, function (tr) {
        var sel = !!state.selected[tr.dataset.id];
        if (sel) on++;
        tr.classList.toggle('sel', sel);
        tr.querySelector('input[type="checkbox"]').checked = sel;
      });
      var allBox = root.querySelector('tr.hdr input[type="checkbox"]');
      if (allBox) allBox.checked = all.length > 0 && on === all.length;
      notifySelect();
    }

    function setSelected(id, on) {
      if (on) state.selected[id] = true;
      else delete state.selected[id];
    }

    // pick - kliknięcie wiersza albo jego pola wyboru:
    // toggle = przełącz ten wiersz, range = zakres od kotwicy (stan kotwicy)
    function pick(row, how) {
      var id = row[state.idKey];
      if (how === 'range' && state.anchor !== null) {
        var rows = visibleRows();
        var ids = rows.map(function (r) { return String(r[state.idKey]); });
        var a = ids.indexOf(String(state.anchor)), b = ids.indexOf(String(id));
        if (a >= 0 && b >= 0) {
          var on = !!state.selected[state.anchor];
          for (var i = Math.min(a, b); i <= Math.max(a, b); i++) setSelected(rows[i][state.idKey], on);
          syncSelection();
          return;
        }
      }
      setSelected(id, !state.selected[id]);
      state.anchor = id;
      syncSelection();
    }

    function selectAll() {
      visibleRows().forEach(function (r) { state.selected[r[state.idKey]] = true; });
      syncSelection();
    }

    var dragKey = null;   // klucz przeciąganej kolumny

    // headCell - nagłówek kolumny: sortowanie kliknięciem, przeciąganie
    // (kolejność) i uchwyt szerokości przy prawej krawędzi
    function headCell(col, cols, table) {
      var th = document.createElement('th');
      th.draggable = true;
      th.title = col.label + '\n' + RZ.t('res.col_hint');
      if (col.align === 'right') th.className = 'r';

      // span zamiast button: z przycisku przeglądarki nie zaczynają przeciągania
      var lbl = document.createElement('span');
      lbl.className = 'sort';
      lbl.tabIndex = 0;
      lbl.setAttribute('role', 'button');
      lbl.textContent = col.label + (state.sort.key === col.key ? (state.sort.dir > 0 ? ' ↑' : ' ↓') : '');
      function sort() {
        if (state.sort.key === col.key) state.sort.dir = -state.sort.dir;
        else state.sort = {key: col.key, dir: 1};
        storeView();
        render();
      }
      lbl.addEventListener('click', sort);
      lbl.addEventListener('keydown', function (e) {
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); sort(); }
      });
      th.appendChild(lbl);

      th.addEventListener('dragstart', function (e) {
        dragKey = col.key;
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', col.key);
        th.classList.add('dragging');
      });
      th.addEventListener('dragend', function () {
        dragKey = null;
        th.classList.remove('dragging');
      });
      function side(e) {
        var b = th.getBoundingClientRect();
        return e.clientX > b.left + b.width / 2;
      }
      th.addEventListener('dragover', function (e) {
        if (!dragKey || dragKey === col.key) return;
        e.preventDefault();
        var after = side(e);
        th.classList.toggle('drop-r', after);
        th.classList.toggle('drop-l', !after);
      });
      th.addEventListener('dragleave', function () { th.classList.remove('drop-l', 'drop-r'); });
      th.addEventListener('drop', function (e) {
        e.preventDefault();
        th.classList.remove('drop-l', 'drop-r');
        if (dragKey) moveColumn(dragKey, col.key, side(e));
      });

      var rsz = document.createElement('span');
      rsz.className = 'rsz';
      rsz.addEventListener('mousedown', function (e) {
        e.preventDefault();   // bez przeciągania kolumny i zaznaczania tekstu
        e.stopPropagation();
        startResize(e, col, cols, table);
      });
      rsz.addEventListener('click', function (e) { e.stopPropagation(); });
      rsz.addEventListener('dblclick', function (e) {
        e.stopPropagation();
        delete state.widths[col.key];
        storeView();
        render();
      });
      th.appendChild(rsz);
      return th;
    }

    // fixLayout - przy zapamiętanych szerokościach tabela ma stały układ
    // (colgroup); kolumny bez szerokości dostają szerokość z treści.
    // force: układ stały także bez zapamiętanych szerokości (start zmiany).
    function fixLayout(table, cols, force) {
      if (!force && !cols.some(function (c) { return state.widths[c.key]; })) return null;
      var ths = table.querySelector('tr.hdr').children;
      var ws = [ths[0].getBoundingClientRect().width];
      cols.forEach(function (c, i) {
        ws.push(state.widths[c.key] || ths[i + 1].getBoundingClientRect().width);
      });
      var old = table.querySelector('colgroup');
      if (old) old.remove();
      var cg = document.createElement('colgroup');
      ws.forEach(function (w) {
        var c = document.createElement('col');
        c.style.width = w + 'px';
        cg.appendChild(c);
      });
      table.insertBefore(cg, table.firstChild);
      table.classList.add('fixed');
      table.style.width = ws.reduce(function (a, b) { return a + b; }, 0) + 'px';
      return {cg: cg, ws: ws};
    }

    function startResize(e, col, cols, table) {
      var lay = fixLayout(table, cols, true);
      var i = cols.indexOf(col) + 1;
      var x0 = e.clientX, w0 = lay.ws[i];
      document.body.classList.add('col-resizing');
      function move(ev) {
        lay.ws[i] = Math.max(MINW, Math.round(w0 + ev.clientX - x0));
        lay.cg.children[i].style.width = lay.ws[i] + 'px';
        table.style.width = lay.ws.reduce(function (a, b) { return a + b; }, 0) + 'px';
      }
      function up() {
        document.removeEventListener('mousemove', move);
        document.removeEventListener('mouseup', up);
        document.body.classList.remove('col-resizing');
        // zapamiętujemy wszystkie pokazane kolumny - inaczej sąsiednie
        // zmieniałyby się razem z treścią przy następnym rysowaniu
        cols.forEach(function (c, j) { state.widths[c.key] = Math.round(lay.ws[j + 1]); });
        storeView();
      }
      document.addEventListener('mousemove', move);
      document.addEventListener('mouseup', up);
    }

    function render() {
      var rows = visibleRows();
      var cols = orderedColumns().filter(function (c) {
        if (state.hidden[c.key]) return false;
        return !(opts.autoHide && rows.length && opts.autoHide(c, rows));
      });
      state.shown = cols;

      var table = document.createElement('table');
      table.className = 'tbl';

      var thead = document.createElement('thead');
      var htr = document.createElement('tr');
      htr.className = 'hdr';

      var thSel = document.createElement('th');
      var allBox = document.createElement('input');
      allBox.type = 'checkbox';
      allBox.checked = rows.length > 0 && rows.every(function (r) { return state.selected[r[state.idKey]]; });
      allBox.addEventListener('change', function () {
        rows.forEach(function (r) { setSelected(r[state.idKey], allBox.checked); });
        syncSelection();
      });
      thSel.appendChild(allBox);
      htr.appendChild(thSel);

      cols.forEach(function (col) { htr.appendChild(headCell(col, cols, table)); });
      if (cols.some(function (c) { return c.group; })) {
        // górny wiersz głowicy: grupy kolejnych kolumn (sekcje formularza)
        var gtr = document.createElement('tr');
        gtr.className = 'grp';
        gtr.appendChild(document.createElement('th'));
        var last = null;
        cols.forEach(function (col) {
          var g = col.group || '';
          if (last && last.dataset.g === g) {
            last.colSpan += 1;
            return;
          }
          last = document.createElement('th');
          last.dataset.g = g;
          last.textContent = g;
          if (g) last.className = 'g';
          gtr.appendChild(last);
        });
        thead.appendChild(gtr);
      }
      thead.appendChild(htr);
      thead.appendChild(filterRow(cols));
      table.appendChild(thead);

      var tbody = document.createElement('tbody');
      rows.forEach(function (row) {
        var tr = document.createElement('tr');
        tr.dataset.id = row[state.idKey];
        if (state.selected[row[state.idKey]]) tr.classList.add('sel');

        var tdSel = document.createElement('td');
        var box = document.createElement('input');
        box.type = 'checkbox';
        box.checked = !!state.selected[row[state.idKey]];
        box.addEventListener('click', function (e) {
          e.stopPropagation();
          e.preventDefault();   // stan pola ustawia pick
          var how = e.shiftKey ? 'range' : 'toggle';
          setTimeout(function () { pick(row, how); });   // po preventDefault przeglądarka cofa pole
        });
        tdSel.appendChild(box);
        tr.appendChild(tdSel);

        cols.forEach(function (col) {
          var td = document.createElement('td');
          if (col.align === 'right') td.className = 'r';
          if (col.type === 'num' || col.type === 'date') td.classList.add('num');
          if (col.cellClass) {
            var cc = col.cellClass(row);
            if (cc) td.classList.add(cc);
          }
          if (col.cellTitle) {
            var ct = col.cellTitle(row);
            if (ct) td.title = ct;
          }
          if (col.render) {
            var out = col.render(row);
            if (out instanceof Node) td.appendChild(out);
            else td.innerHTML = out;
          } else {
            td.textContent = cellText(row, col);
          }
          tr.appendChild(td);
        });

        tr.addEventListener('mousedown', function (e) {
          if (e.shiftKey) e.preventDefault();   // Shift+klik nie zaznacza tekstu
        });
        tr.addEventListener('click', function (e) {
          var tag = e.target.tagName;
          if (tag === 'INPUT' || tag === 'BUTTON' || tag === 'A') return;
          if (e.shiftKey) { pick(row, 'range'); return; }
          if (e.ctrlKey || e.metaKey) { pick(row, 'toggle'); return; }
          if (opts.onRowClick && opts.onRowClick(row) !== false) {
            state.anchor = row[state.idKey];
            return;
          }
          pick(row, 'toggle');
        });
        tbody.appendChild(tr);
      });
      table.appendChild(tbody);

      // tabela rysuje się od nowa przy każdej literze filtra - pole, w którym
      // się pisze, musi zachować fokus i kursor
      var act = document.activeElement, focus = null;
      if (act && act.dataset && act.dataset.filter && root.contains(act)) {
        focus = {key: act.dataset.filter, start: act.selectionStart, end: act.selectionEnd};
      }
      var scroll = root.scrollLeft;
      root.innerHTML = '';
      root.appendChild(table);
      fixLayout(table, cols, false);
      root.scrollLeft = scroll;
      if (focus) {
        var inp = root.querySelector('input[data-filter="' + focus.key + '"]');
        if (inp) {
          inp.focus({preventScroll: true});
          inp.setSelectionRange(focus.start, focus.end);
        }
      }

      if (opts.onRender) opts.onRender(rows);
    }

    function anyFilter() {
      return Object.keys(state.filters).some(function (k) { return state.filters[k]; });
    }

    // filterRow - pola filtrów pod nagłówkami kolumn; w pierwszej komórce
    // przycisk czyszczący wszystkie filtry (gdy któryś jest ustawiony)
    function filterRow(cols) {
      var tr = document.createElement('tr');
      tr.className = 'flt';
      var first = document.createElement('th');
      if (anyFilter()) {
        var clr = document.createElement('button');
        clr.type = 'button';
        clr.className = 'flt-x';
        clr.textContent = '×';
        clr.title = RZ.t('res.filter_clear');
        clr.setAttribute('aria-label', clr.title);
        clr.addEventListener('click', function () {
          state.filters = {};
          render();
        });
        first.appendChild(clr);
      }
      tr.appendChild(first);
      cols.forEach(function (col) {
        var th = document.createElement('th');
        var inp = document.createElement('input');
        inp.className = 'inp';
        inp.size = 1;   // szerokość daje kolumna, nie domyślne 20 znaków pola
        inp.dataset.filter = col.key;
        inp.value = state.filters[col.key] || '';
        inp.setAttribute('aria-label', RZ.t('res.filter_col') + ': ' + col.label);
        inp.addEventListener('input', function () {
          state.filters[col.key] = inp.value;
          render();
        });
        th.appendChild(inp);
        tr.appendChild(th);
      });
      return tr;
    }

    // chooser - lista kolumn z polami wyboru w el (w rozwijanym <details>),
    // w grupach (col.group); pole przy nazwie grupy włącza i wyłącza całą
    // grupę. Na końcu "przywróć domyślne": kolejność, szerokości, kolumny.
    function chooser(el) {
      chooserEl = el;
      el.innerHTML = '';
      var groups = [];
      state.columns.forEach(function (c) {
        var g = groups[groups.length - 1];
        var label = c.group || '';
        if (!g || g.label !== label) {
          g = {label: label, cols: []};
          groups.push(g);
        }
        g.cols.push(c);
      });
      var boxes = {};
      function syncGroup(g) {
        if (!g.box) return;
        var on = g.cols.filter(function (c) { return !state.hidden[c.key]; }).length;
        g.box.checked = on === g.cols.length;
        g.box.indeterminate = on > 0 && on < g.cols.length;
      }
      groups.forEach(function (g) {
        if (groups.length > 1 && g.label.trim()) {
          var gl = document.createElement('label');
          gl.className = 'colsec';
          g.box = document.createElement('input');
          g.box.type = 'checkbox';
          g.box.addEventListener('change', function () {
            var hide = !g.box.checked;
            g.cols.forEach(function (c) { state.hidden[c.key] = hide; boxes[c.key].checked = !hide; });
            storeView();
            render();
            syncGroup(g);
          });
          gl.appendChild(g.box);
          gl.appendChild(document.createTextNode(' ' + g.label));
          el.appendChild(gl);
        }
        g.cols.forEach(function (c) {
          var lbl = document.createElement('label');
          lbl.className = 'colopt';
          lbl.title = c.label;
          var box = document.createElement('input');
          box.type = 'checkbox';
          box.checked = !state.hidden[c.key];
          box.addEventListener('change', function () {
            state.hidden[c.key] = !box.checked;
            storeView();
            render();
            syncGroup(g);
          });
          boxes[c.key] = box;
          lbl.appendChild(box);
          lbl.appendChild(document.createTextNode(' ' + c.label));
          el.appendChild(lbl);
        });
        syncGroup(g);
      });
      var reset = document.createElement('button');
      reset.type = 'button';
      reset.className = 'link colreset';
      reset.textContent = RZ.t('res.cols_reset');
      reset.addEventListener('click', function () {
        state.order = null;
        state.widths = {};
        state.hidden = {};
        storeView();
        render();
        chooser(el);
      });
      el.appendChild(reset);

      // rozwijana lista zamyka się kliknięciem obok niej
      var dd = el.closest('details');
      if (dd && !dd.dataset.wired) {
        dd.dataset.wired = '1';
        document.addEventListener('click', function (e) {
          if (dd.open && !dd.contains(e.target)) dd.open = false;
        });
      }
    }

    render();

    var api = {
      setRows: function (rows) {
        state.rows = rows;
        // zaznaczenia, których już nie ma, znikają
        var ids = {};
        rows.forEach(function (r) { ids[r[state.idKey]] = true; });
        Object.keys(state.selected).forEach(function (id) { if (!ids[id]) delete state.selected[id]; });
        render();
        notifySelect();
      },
      setSearch: function (s) { state.search = s; render(); },
      setFilter: function (key, val) { state.filters[key] = val; render(); },
      setRowFilter: function (fn) { opts.rowFilter = fn; render(); },
      toggleColumn: function (key, hide) {
        state.hidden[key] = hide;
        storeView();
        render();
        if (chooserEl) chooser(chooserEl);
      },
      hiddenColumns: function () { return state.hidden; },
      shownColumns: function () { return state.shown; },
      chooser: chooser,
      clearSelection: function () { state.selected = {}; render(); notifySelect(); },
      selected: selectedRows,
      visibleRows: visibleRows,
      rows: function () { return state.rows; },
      selectAll: selectAll,
      render: render
    };
    root._ntable = api;   // Ctrl+A trafia do ostatniej tabeli w tym miejscu
    if (roots.indexOf(root) === -1) roots.push(root);
    return api;
  }

  // Ctrl+A poza polami tekstowymi zaznacza wiersze widocznej tabeli
  var roots = [];
  document.addEventListener('keydown', function (e) {
    if (!(e.ctrlKey || e.metaKey) || e.altKey || e.key.toLowerCase() !== 'a') return;
    var t = e.target;
    if (t.isContentEditable || /^(TEXTAREA|SELECT)$/.test(t.tagName)) return;
    if (t.tagName === 'INPUT' && !/^(checkbox|radio|button|submit)$/.test(t.type)) return;
    var hit = false;
    roots.forEach(function (r) {
      if (r._ntable && r.offsetParent !== null) { r._ntable.selectAll(); hit = true; }
    });
    if (hit) e.preventDefault();
  });

  global.NTable = {create: create};
})(window);
