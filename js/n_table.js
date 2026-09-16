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
//     onRowClick: function (row) {}
//   });
//   t.setRows(rows); t.selected(); t.visibleRows();

(function (global) {
  'use strict';

  function create(root, opts) {
    var state = {
      columns: opts.columns,
      rows: opts.rows || [],
      sort: {key: null, dir: 1},
      search: '',
      filters: {},           // klucz kolumny -> tekst
      hidden: {},            // klucz kolumny -> true
      selected: {},          // id wiersza -> true
      idKey: opts.idKey || 'id'
    };

    // zapamiętany widok: kolumny, sortowanie, filtry
    if (opts.storageKey) {
      try {
        var saved = JSON.parse(localStorage.getItem(opts.storageKey) || '{}');
        if (saved.sort) state.sort = saved.sort;
        if (saved.hidden) state.hidden = saved.hidden;
        if (saved.filters) state.filters = saved.filters;
      } catch (e) { /* prywatne okno */ }
    }

    function storeView() {
      if (!opts.storageKey) return;
      try {
        localStorage.setItem(opts.storageKey, JSON.stringify({
          sort: state.sort, hidden: state.hidden, filters: state.filters
        }));
      } catch (e) { /* ignorujemy */ }
    }

    function cellText(row, col) {
      var v = row[col.key];
      if (v === null || v === undefined) return '';
      if (col.text) return col.text(row);
      return String(v);
    }

    function matches(row) {
      if (state.search) {
        var hay = state.columns.map(function (c) { return cellText(row, c); }).join(' ').toLowerCase();
        if (hay.indexOf(state.search.toLowerCase()) === -1) return false;
      }
      for (var key in state.filters) {
        var f = state.filters[key];
        if (!f) continue;
        var col = state.columns.filter(function (c) { return c.key === key; })[0];
        if (!col) continue;
        if (cellText(row, col).toLowerCase().indexOf(f.toLowerCase()) === -1) return false;
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
          if (col && (col.type === 'num' || col.type === 'date')) {
            return (Number(av) - Number(bv)) * state.sort.dir;
          }
          return String(av === null || av === undefined ? '' : av)
            .localeCompare(String(bv === null || bv === undefined ? '' : bv), 'pl') * state.sort.dir;
        });
      }
      return rows;
    }

    function selectedRows() {
      return state.rows.filter(function (r) { return state.selected[r[state.idKey]]; });
    }

    function notifySelect() {
      if (opts.onSelect) opts.onSelect(selectedRows());
    }

    function render() {
      var rows = visibleRows();
      var cols = state.columns.filter(function (c) { return !state.hidden[c.key]; });

      var table = document.createElement('table');
      table.className = 'tbl';

      var thead = document.createElement('thead');
      var htr = document.createElement('tr');

      var thSel = document.createElement('th');
      var allBox = document.createElement('input');
      allBox.type = 'checkbox';
      allBox.checked = rows.length > 0 && rows.every(function (r) { return state.selected[r[state.idKey]]; });
      allBox.addEventListener('change', function () {
        rows.forEach(function (r) {
          if (allBox.checked) state.selected[r[state.idKey]] = true;
          else delete state.selected[r[state.idKey]];
        });
        render();
        notifySelect();
      });
      thSel.appendChild(allBox);
      htr.appendChild(thSel);

      cols.forEach(function (col) {
        var th = document.createElement('th');
        var btn = document.createElement('button');
        btn.className = 'link';
        btn.style.textDecoration = 'none';
        btn.textContent = col.label + (state.sort.key === col.key ? (state.sort.dir > 0 ? ' ↑' : ' ↓') : '');
        btn.addEventListener('click', function () {
          if (state.sort.key === col.key) state.sort.dir = -state.sort.dir;
          else state.sort = {key: col.key, dir: 1};
          storeView();
          render();
        });
        th.appendChild(btn);
        if (col.align === 'right') th.className = 'r';
        htr.appendChild(th);
      });
      thead.appendChild(htr);
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
        box.addEventListener('change', function (e) {
          e.stopPropagation();
          if (box.checked) state.selected[row[state.idKey]] = true;
          else delete state.selected[row[state.idKey]];
          tr.classList.toggle('sel', box.checked);
          notifySelect();
        });
        tdSel.appendChild(box);
        tr.appendChild(tdSel);

        cols.forEach(function (col) {
          var td = document.createElement('td');
          if (col.align === 'right') td.className = 'r';
          if (col.type === 'num' || col.type === 'date') td.classList.add('num');
          if (col.render) {
            var out = col.render(row);
            if (out instanceof Node) td.appendChild(out);
            else td.innerHTML = out;
          } else {
            td.textContent = cellText(row, col);
          }
          tr.appendChild(td);
        });

        if (opts.onRowClick) {
          tr.style.cursor = 'pointer';
          tr.addEventListener('click', function (e) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'BUTTON') return;
            opts.onRowClick(row);
          });
        }
        tbody.appendChild(tr);
      });
      table.appendChild(tbody);

      root.innerHTML = '';
      root.appendChild(table);

      if (opts.onRender) opts.onRender(rows);
    }

    render();

    return {
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
      setFilter: function (key, val) { state.filters[key] = val; storeView(); render(); },
      setRowFilter: function (fn) { opts.rowFilter = fn; render(); },
      toggleColumn: function (key, hide) { state.hidden[key] = hide; storeView(); render(); },
      hiddenColumns: function () { return state.hidden; },
      clearSelection: function () { state.selected = {}; render(); notifySelect(); },
      selected: selectedRows,
      visibleRows: visibleRows,
      rows: function () { return state.rows; },
      render: render
    };
  }

  global.NTable = {create: create};
})(window);
