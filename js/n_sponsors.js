// n_sponsors.js - edytor sekcji "Sponsorzy" na stronie imprezy (sponsors.go).
//
// Logo leżą w wierszach tak, jak zobaczy je gość. Przeciąganie decyduje
// o układzie: upuszczenie w wierszu wstawia logo między sąsiadów, na pasku
// między wierszami (albo pod ostatnim) zaczyna nowy wiersz; puste wiersze
// znikają. Z klawiatury: strzałki ←/→ w wierszu, ↑/↓ do wiersza obok
// (↓ z ostatniego = nowy wiersz), Delete usuwa. Kliknięte logo pokazuje
// pod spodem pole na adres strony sponsora.
//
// Stan to ukryte pole sponsors_rows (JSON [[{file, link}]]); każda zmiana
// wysyła input, więc sekcja wie o niezapisanych zmianach (RZ.sections).

(function () {
  'use strict';

  var box = document.querySelector('.spk');
  if (!box) return;
  var t = RZ.t;
  var sec = box.closest('.sec');
  var inp = box.querySelector('input[data-field="sponsors_rows"]');
  var sizeInp = box.querySelector('input[data-field="sponsors_size"]');
  var board = box.querySelector('.sp-board');
  var selBox = box.querySelector('.sp-sel');
  var linkInp = selBox.querySelector('input');
  var fileInp = box.querySelector('.sp-file');
  var HEIGHTS = {s: 48, '': 72, l: 104};

  var rows = [];
  var sel = null; // {r, i} zaznaczone logo
  var drag = null; // {r, i} przeciągane logo

  function load() {
    try { rows = JSON.parse(inp.value || '[]') || []; } catch (e) { rows = []; }
    rows = rows.filter(function (r) { return Array.isArray(r) && r.length; });
  }

  function src(file) {
    return box.dataset.base + file.split('/').map(encodeURIComponent).join('/');
  }

  // commit zapisuje stan w polu i rysuje edytor od nowa
  function commit() {
    rows = rows.filter(function (r) { return r.length; });
    inp.value = JSON.stringify(rows);
    inp.dispatchEvent(new Event('input', {bubbles: true}));
    render();
  }

  function gap(k, last) {
    var g = document.createElement('div');
    g.className = 'sp-gap' + (last ? ' last' : '');
    g.dataset.at = k;
    if (last) g.textContent = rows.length ? t('sponsors.drop_new_row') : t('sponsors.empty');
    return g;
  }

  function render() {
    board.style.setProperty('--sp-h', (HEIGHTS[sizeInp.value] || 72) + 'px');
    board.innerHTML = '';
    rows.forEach(function (row, r) {
      board.appendChild(gap(r, false));
      var rowEl = document.createElement('div');
      rowEl.className = 'sp-row';
      rowEl.dataset.r = r;
      row.forEach(function (logo, i) {
        var el = document.createElement('div');
        el.className = 'sp-logo';
        el.draggable = true;
        el.tabIndex = 0;
        el.dataset.r = r;
        el.dataset.i = i;
        el.title = logo.file + (logo.link ? '\n' + logo.link : '');
        el.setAttribute('aria-label', logo.file);
        if (sel && sel.r === r && sel.i === i) el.classList.add('on');
        if (logo.link) el.classList.add('linked');
        var img = document.createElement('img');
        img.src = src(logo.file);
        img.alt = '';
        img.draggable = false;
        img.onerror = function () { el.classList.add('missing'); };
        el.appendChild(img);
        rowEl.appendChild(el);
      });
      board.appendChild(rowEl);
    });
    board.appendChild(gap(rows.length, true));
    showSel();
  }

  function showSel() {
    var logo = sel && rows[sel.r] && rows[sel.r][sel.i];
    if (!logo) { sel = null; selBox.hidden = true; return; }
    selBox.hidden = false;
    selBox.querySelector('img').src = src(logo.file);
    selBox.querySelector('.sp-sel-name').textContent = logo.file;
    if (document.activeElement !== linkInp) linkInp.value = logo.link || '';
  }

  function select(r, i, focus) {
    sel = {r: r, i: i};
    render();
    if (focus) {
      var el = board.querySelector('.sp-logo[data-r="' + r + '"][data-i="' + i + '"]');
      if (el) el.focus();
    }
  }

  // move przenosi logo z (r, i) do wiersza toR na miejsce toI; newRow =
  // zacznij nowy wiersz o numerze toR
  function move(r, i, toR, toI, newRow) {
    var logo = rows[r].splice(i, 1)[0];
    if (newRow) {
      rows.splice(toR, 0, [logo]);
      if (toR <= r) r++; // wiersz źródłowy przesunął się w dół
      toI = 0;
    } else {
      if (toR === r && toI > i) toI--;
      rows[toR].splice(toI, 0, logo);
    }
    // indeks wiersza docelowego po usunięciu pustego źródła
    var target = rows[toR];
    rows = rows.filter(function (x) { return x.length; });
    sel = {r: rows.indexOf(target), i: toI};
    commit();
  }

  function add(name) {
    var logo = {file: name};
    if (!rows.length) rows.push([]);
    var last = rows[rows.length - 1];
    last.push(logo);
    sel = {r: rows.length - 1, i: last.length - 1};
    commit();
  }

  function remove(r, i) {
    rows[r].splice(i, 1);
    sel = null;
    commit();
  }

  // --- przeciąganie ---------------------------------------------------------

  var mark = document.createElement('div');
  mark.className = 'sp-mark';

  function clearDrop() {
    mark.remove();
    board.querySelectorAll('.sp-gap.over').forEach(function (g) { g.classList.remove('over'); });
  }

  // insertAt - miejsce w wierszu wg poziomej pozycji kursora
  function insertAt(rowEl, x) {
    var logos = rowEl.querySelectorAll('.sp-logo');
    for (var k = 0; k < logos.length; k++) {
      var b = logos[k].getBoundingClientRect();
      if (x < b.left + b.width / 2) return k;
    }
    return logos.length;
  }

  board.addEventListener('dragstart', function (e) {
    var el = e.target.closest('.sp-logo');
    if (!el) return;
    drag = {r: +el.dataset.r, i: +el.dataset.i};
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', el.getAttribute('aria-label'));
    // klasa po chwili, żeby obrazek przeciągania był jeszcze "normalny"
    setTimeout(function () { if (drag) { el.classList.add('dragging'); board.classList.add('dragging'); } }, 0);
  });

  // po upuszczeniu edytor rysuje się od nowa, więc dragend trafia do
  // odłączonego logo i nie dochodzi do planszy - sprzątamy też w drop
  function endDrag() {
    drag = null;
    board.classList.remove('dragging');
    board.querySelectorAll('.dragging').forEach(function (el) { el.classList.remove('dragging'); });
    clearDrop();
  }
  board.addEventListener('dragend', endDrag);

  board.addEventListener('dragover', function (e) {
    if (!drag) return;
    var g = e.target.closest('.sp-gap');
    var rowEl = e.target.closest('.sp-row');
    if (!g && !rowEl) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    clearDrop();
    if (g) {
      g.classList.add('over');
      return;
    }
    var k = insertAt(rowEl, e.clientX);
    var logos = rowEl.querySelectorAll('.sp-logo');
    rowEl.insertBefore(mark, logos[k] || null);
  });

  board.addEventListener('dragleave', function (e) {
    if (!board.contains(e.relatedTarget)) clearDrop();
  });

  board.addEventListener('drop', function (e) {
    if (!drag) return;
    e.preventDefault();
    var g = e.target.closest('.sp-gap');
    var rowEl = e.target.closest('.sp-row');
    var from = drag;
    endDrag();
    if (g) {
      move(from.r, from.i, +g.dataset.at, 0, true);
    } else if (rowEl) {
      move(from.r, from.i, +rowEl.dataset.r, insertAt(rowEl, e.clientX), false);
    }
  });

  // --- klik i klawiatura -----------------------------------------------------

  board.addEventListener('click', function (e) {
    var el = e.target.closest('.sp-logo');
    if (el) select(+el.dataset.r, +el.dataset.i, false);
  });

  board.addEventListener('keydown', function (e) {
    var el = e.target.closest('.sp-logo');
    if (!el) return;
    var r = +el.dataset.r, i = +el.dataset.i;
    switch (e.key) {
      case 'ArrowLeft':
        if (i > 0) move(r, i, r, i - 1, false);
        break;
      case 'ArrowRight':
        if (i < rows[r].length - 1) move(r, i, r, i + 2, false);
        break;
      case 'ArrowUp':
        if (rows[r].length === 1 && r === 0) return;
        if (r === 0) move(r, i, 0, 0, true); // nowy wiersz na górze
        else move(r, i, r - 1, rows[r - 1].length, false);
        break;
      case 'ArrowDown':
        if (r === rows.length - 1) {
          if (rows[r].length === 1) return;
          move(r, i, rows.length, 0, true);
        } else {
          move(r, i, r + 1, rows[r + 1].length, false);
        }
        break;
      case 'Delete':
      case 'Backspace':
        remove(r, i);
        return;
      case 'Enter':
      case ' ':
        select(r, i, true);
        break;
      default:
        return;
    }
    e.preventDefault();
    if (sel) select(sel.r, sel.i, true);
  });

  linkInp.addEventListener('input', function (e) {
    e.stopPropagation(); // pole bez data-field; zmianę zgłasza commit niżej
    if (!sel) return;
    rows[sel.r][sel.i].link = linkInp.value.trim();
    inp.value = JSON.stringify(rows);
    inp.dispatchEvent(new Event('input', {bubbles: true}));
  });
  linkInp.addEventListener('change', function (e) {
    e.stopPropagation();
    render();
  });

  selBox.querySelector('.sp-del').addEventListener('click', function () {
    if (sel) remove(sel.r, sel.i);
  });

  // --- dodawanie logo --------------------------------------------------------

  box.querySelector('.sp-up').addEventListener('click', function () { fileInp.click(); });
  box.querySelector('.sp-choose').addEventListener('click', function (e) {
    RZ.mediaMenu(e.currentTarget, '', function (f) { add(f.name); });
  });

  fileInp.addEventListener('change', function (e) {
    e.stopPropagation(); // sam wybór plików nie zmienia jeszcze sekcji
    var files = Array.prototype.slice.call(fileInp.files || []);
    fileInp.value = '';
    var btn = box.querySelector('.sp-up');
    btn.disabled = true;
    // po kolei, żeby logo stanęły w kolejności wybranych plików
    files.reduce(function (p, f) {
      return p.then(function () {
        return RZ.uploadMedia(f).then(function (resp) { add(resp.name); })
          .catch(function (err) { RZ.toast(f.name + ': ' + err.message); });
      });
    }, Promise.resolve()).finally(function () { btn.disabled = false; });
  }, true);

  // wielkość logo od razu w podglądzie
  sec.addEventListener('click', function (e) {
    if (e.target.closest('.seg-pick button')) setTimeout(render, 0);
  });

  // szkic przywrócony po powrocie na stronę (applyValues w n_ui.js)
  sec.addEventListener('sec:values', function () { load(); sel = null; render(); });

  RZ.ready.then(function () { load(); render(); });
})();
