// n_formfill.js - menedżer dopisuje osoby przy funkcjach formularza
// (/admin/forms/{id}/fill, form_fill.go).
//
// Pola wspólne (np. klasa) to zwykłe pola formularza z warunkami
// (n_formconds.js); pola .ff-people mają listę osób i wiersz "dodaj osobę"
// z podpowiedziami z wcześniej wpisanych osób. Zmiany czekają na "Zapisz".
// Każde dopisanie pamięta wartości pól wspólnych z chwili dodania, bo nowa
// osoba dostaje je do swojego zgłoszenia (np. klasę 1, choć przed zapisem
// wybrano już klasę 2) - zapis wysyła po jednym żądaniu na każdy zestaw.
(function () {
  'use strict';

  var root = document.getElementById('fill');
  var form = document.getElementById('fill-form');
  if (!root || !form) return;

  var t = RZ.t;
  var formID = root.dataset.formId;
  var draftKey = root.dataset.draft;
  var state = JSON.parse(document.getElementById('fill-state').textContent || '{}');
  var people = state.people || {};
  var contacts = state.contacts || [];
  var shared = Array.prototype.slice.call(form.querySelectorAll('.ff[data-name]:not(.ff-people)'));
  var boxes = Array.prototype.slice.call(form.querySelectorAll('.ff-people'));
  var saveBtn = document.getElementById('fill-save');
  var undoBtn = document.getElementById('fill-undo');
  var countEl = document.getElementById('fill-count');
  var msgEl = document.getElementById('fill-msg');

  // pending.add[pole] = [{name, surname, email, data}], pending.remove = {"pole:form": true}
  var pending = {add: {}, remove: {}};
  var conds = RZ.formConds(form);

  function lower(s) { return String(s || '').toLowerCase(); }

  // --- pola wspólne --------------------------------------------------------

  function valueOf(ff) {
    var type = ff.dataset.type;
    if (type === 'checkbox-group' || type === 'radio-group') {
      return Array.prototype.filter.call(ff.querySelectorAll('input:not(.other-text)'), function (i) { return i.checked && i.value !== ''; })
        .map(function (i) { return i.value; });
    }
    var el = ff.querySelector('input:not(.other-text), select, textarea');
    if (!el) return [];
    if (el.multiple) return Array.prototype.map.call(el.selectedOptions, function (o) { return o.value; });
    if (el.hasAttribute('data-date')) {
      var dt = RZ.parseDate(el.value);
      return [dt ? RZ.fmtDate(dt) : el.value.trim()];
    }
    return [el.value.trim()];
  }

  function collect() {
    return shared.map(function (ff) { return {name: ff.dataset.name, userData: ff.hidden ? [] : valueOf(ff)}; });
  }

  function showError(ff, text) {
    var err = ff.querySelector('.ff-err');
    ff.classList.toggle('bad', !!text);
    if (!err) return;
    err.textContent = text;
    err.hidden = !text;
  }

  // sharedError - pierwsze wymagane, widoczne i puste pole wspólne
  function sharedError() {
    for (var i = 0; i < shared.length; i++) {
      var ff = shared[i];
      if (ff.hidden || !ff.hasAttribute('data-required')) continue;
      var v = valueOf(ff).filter(function (x) { return x !== ''; });
      if (!v.length) {
        var sel = ff.querySelector('select');
        showError(ff, ff.dataset.type === 'checkbox-group' || ff.dataset.type === 'radio-group' || sel ? t('gform.err.choose') : t('gform.err.required'));
        ff.scrollIntoView({behavior: 'smooth', block: 'center'});
        return true;
      }
      showError(ff, '');
    }
    return false;
  }

  function saveDraft() {
    var d = {};
    shared.forEach(function (ff) { d[ff.dataset.name] = valueOf(ff); });
    try { localStorage.setItem(draftKey, JSON.stringify(d)); } catch (e) { /* prywatne okno */ }
  }

  function restoreDraft() {
    var d = null;
    try { d = JSON.parse(localStorage.getItem(draftKey) || 'null'); } catch (e) { d = null; }
    if (!d) return;
    shared.forEach(function (ff) {
      var vals = d[ff.dataset.name];
      if (!vals) return;
      var checks = ff.querySelectorAll('input[type="radio"], input[type="checkbox"]');
      if (checks.length) {
        checks.forEach(function (i) { i.checked = vals.indexOf(i.value) >= 0; });
        return;
      }
      var sel = ff.querySelector('select');
      if (sel) {
        Array.prototype.forEach.call(sel.options, function (o) { o.selected = vals.indexOf(o.value) >= 0; });
        return;
      }
      var el = ff.querySelector('input, textarea');
      if (el && vals.length) el.value = vals[0];
    });
  }

  // --- listy osób ----------------------------------------------------------

  function key(field, formId) { return field + ':' + formId; }

  function pendingCount() {
    var n = Object.keys(pending.remove).length;
    Object.keys(pending.add).forEach(function (f) { n += pending.add[f].length; });
    return n;
  }

  function onField(field) {
    var n = 0;
    (people[field] || []).forEach(function (p) { if (!pending.remove[key(field, p.form)]) n++; });
    return n + (pending.add[field] || []).length;
  }

  function hasEmail(field, email) {
    email = lower(email);
    return (people[field] || []).some(function (p) { return lower(p.email) === email && !pending.remove[key(field, p.form)]; }) ||
      (pending.add[field] || []).some(function (p) { return lower(p.email) === email; });
  }

  function li(p, cls, tag, btn) {
    var el = document.createElement('li');
    if (cls) el.className = cls;
    var b = document.createElement('b');
    b.textContent = (p.name + ' ' + p.surname).trim();
    el.appendChild(b);
    if (p.email) {
      var s = document.createElement('small');
      s.textContent = p.email;
      el.appendChild(s);
    }
    if (tag) {
      var g = document.createElement('i');
      g.className = 'pp-tag';
      g.textContent = tag;
      el.appendChild(g);
    }
    if (btn) el.appendChild(btn);
    return el;
  }

  function button(text, attrs) {
    var b = document.createElement('button');
    b.type = 'button';
    b.className = 'link';
    b.textContent = text;
    Object.keys(attrs).forEach(function (k) { b.setAttribute(k, attrs[k]); });
    return b;
  }

  function renderBox(box) {
    var field = box.dataset.name;
    var list = box.querySelector('.pp-list');
    list.textContent = '';
    (people[field] || []).forEach(function (p) {
      var removed = !!pending.remove[key(field, p.form)];
      var tag = p.by === 'guest' ? t('fill.by_guest') : p.by === 'other' ? t('fill.by_other') : '';
      var btn = null;
      if (p.mine) {
        btn = removed ? button(t('fill.restore'), {'data-restore': p.form}) : button('×', {'data-remove': p.form, 'title': t('fill.remove'), 'aria-label': t('fill.remove')});
      }
      list.appendChild(li(p, removed ? 'removed' : (p.mine ? 'mine' : ''), removed ? t('fill.will_remove') : tag, btn));
    });
    (pending.add[field] || []).forEach(function (p, i) {
      list.appendChild(li(p, 'new', t('fill.new'), button('×', {'data-unadd': i, 'title': t('fill.remove'), 'aria-label': t('fill.remove')})));
    });
    if (!list.children.length) {
      var e = document.createElement('li');
      e.className = 'empty muted';
      e.textContent = t('fill.nobody');
      list.appendChild(e);
    }
    var n = onField(field);
    var goal = +box.dataset.goal || 0;
    // puste wiersze widoczne tylko na wydruku - do brakującej liczby osób,
    // co najmniej kilka (ktoś może zebrać więcej chętnych niż trzeba)
    var blanks = Math.max(goal - n, 4);
    for (var b = 0; b < blanks; b++) {
      var bl = document.createElement('li');
      bl.className = 'pp-blank';
      bl.innerHTML = '<span></span><span></span>';
      if (b === 0) {
        bl.children[0].dataset.l = list.dataset.printName;
        bl.children[1].dataset.l = list.dataset.printEmail;
      }
      list.appendChild(bl);
    }
    var c = box.querySelector('.pp-count');
    c.textContent = goal ? t('fill.count_goal', [n, goal]) : RZ.tN('count.person', n);
    c.classList.toggle('done', goal > 0 && n >= goal);
  }

  function renderAll() {
    boxes.forEach(renderBox);
    var n = pendingCount();
    saveBtn.disabled = n === 0;
    undoBtn.hidden = n === 0;
    countEl.textContent = n ? RZ.tN('fill.changes', n) : '';
    if (n) msgEl.hidden = true;
  }

  // --- dodawanie -------------------------------------------------------------

  function inputsOf(box) {
    return {
      name: box.querySelector('[data-p="name"]'),
      surname: box.querySelector('[data-p="surname"]'),
      email: box.querySelector('[data-p="email"]')
    };
  }

  function rememberContact(p) {
    var e = lower(p.email);
    contacts = contacts.filter(function (c) { return lower(c.email) !== e; });
    contacts.unshift({name: p.name, surname: p.surname, email: e});
  }

  function addPerson(box, p) {
    var field = box.dataset.name;
    p = {name: p.name.trim().replace(/\s+/g, ' '), surname: p.surname.trim().replace(/\s+/g, ' '), email: lower(p.email.trim())};
    if (!p.name || !p.surname) return showError(box, t('fill.err.name'));
    var probe = document.createElement('input');
    probe.type = 'email';
    probe.value = p.email;
    if (!p.email || RZ.inputError(probe) || !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(p.email)) return showError(box, t('fill.err.email', p.email));
    if (hasEmail(field, p.email)) return showError(box, t('fill.err.dup'));
    if (sharedError()) return showError(box, t('fill.err.shared'));
    showError(box, '');
    p.data = collect();
    (pending.add[field] = pending.add[field] || []).push(p);
    rememberContact(p);
    var inp = inputsOf(box);
    inp.name.value = inp.surname.value = inp.email.value = '';
    hideSug(box);
    inp.name.focus();
    renderAll();
  }

  // --- podpowiedzi -----------------------------------------------------------

  function matches(box) {
    var inp = inputsOf(box);
    var words = (inp.name.value + ' ' + inp.surname.value + ' ' + inp.email.value).toLowerCase().split(/\s+/).filter(Boolean);
    if (!words.length) return [];
    var field = box.dataset.name;
    return contacts.filter(function (c) {
      if (hasEmail(field, c.email)) return false;
      var hay = lower(c.name + ' ' + c.surname + ' ' + c.email);
      return words.every(function (w) { return hay.indexOf(w) >= 0; });
    }).slice(0, 8);
  }

  function hideSug(box) {
    var ul = box.querySelector('.pp-sug');
    ul.hidden = true;
    ul.textContent = '';
  }

  function showSug(box) {
    var ul = box.querySelector('.pp-sug');
    var mm = matches(box);
    ul.textContent = '';
    if (!mm.length) { ul.hidden = true; return; }
    mm.forEach(function (c, i) {
      var el = document.createElement('li');
      el.setAttribute('role', 'option');
      el.dataset.i = i;
      el.innerHTML = '<b></b> <small></small>';
      el.querySelector('b').textContent = c.name + ' ' + c.surname;
      el.querySelector('small').textContent = c.email;
      el._contact = c;
      ul.appendChild(el);
    });
    ul.hidden = false;
  }

  function moveSug(box, dir) {
    var ul = box.querySelector('.pp-sug');
    if (ul.hidden) return false;
    var items = Array.prototype.slice.call(ul.children);
    var cur = items.findIndex(function (x) { return x.classList.contains('on'); });
    items.forEach(function (x) { x.classList.remove('on'); });
    var next = (cur + dir + items.length + (cur < 0 && dir < 0 ? 1 : 0)) % items.length;
    items[next].classList.add('on');
    return true;
  }

  // --- zdarzenia -------------------------------------------------------------

  form.addEventListener('input', function (e) {
    var box = e.target.closest('.ff-people');
    if (box) {
      if (box.classList.contains('bad')) showError(box, '');
      showSug(box);
      return;
    }
    conds.apply();
    var ff = e.target.closest('.ff');
    if (ff && ff.classList.contains('bad')) showError(ff, '');
    saveDraft();
  });

  form.addEventListener('change', function (e) {
    if (e.target.closest('.ff-people')) return;
    conds.apply();
    saveDraft();
  });

  form.addEventListener('keydown', function (e) {
    var box = e.target.closest('.ff-people');
    if (!box || !e.target.matches('.pp-add input')) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (moveSug(box, e.key === 'ArrowDown' ? 1 : -1)) e.preventDefault();
      return;
    }
    if (e.key === 'Escape') { hideSug(box); return; }
    if (e.key !== 'Enter') return;
    e.preventDefault();
    var on = box.querySelector('.pp-sug li.on');
    if (on) { addPerson(box, on._contact); return; }
    var inp = inputsOf(box);
    addPerson(box, {name: inp.name.value, surname: inp.surname.value, email: inp.email.value});
  });

  // mousedown: wybór podpowiedzi, zanim pole straci fokus i lista zniknie
  form.addEventListener('mousedown', function (e) {
    var item = e.target.closest('.pp-sug li');
    if (!item) return;
    e.preventDefault();
    addPerson(item.closest('.ff-people'), item._contact);
  });

  form.addEventListener('focusout', function (e) {
    var box = e.target.closest('.ff-people');
    if (box && !box.querySelector('.pp-add').contains(e.relatedTarget)) hideSug(box);
  });

  form.addEventListener('click', function (e) {
    var box = e.target.closest('.ff-people');
    if (!box) return;
    var field = box.dataset.name;
    var b = e.target.closest('button');
    if (!b) return;
    if (b.hasAttribute('data-pp-add')) {
      var inp = inputsOf(box);
      addPerson(box, {name: inp.name.value, surname: inp.surname.value, email: inp.email.value});
    } else if (b.hasAttribute('data-remove')) {
      pending.remove[key(field, b.getAttribute('data-remove'))] = true;
      renderAll();
    } else if (b.hasAttribute('data-restore')) {
      delete pending.remove[key(field, b.getAttribute('data-restore'))];
      renderAll();
    } else if (b.hasAttribute('data-unadd')) {
      pending.add[field].splice(+b.getAttribute('data-unadd'), 1);
      renderAll();
    }
  });

  // wydruk: widoczne teraz pola z wybranymi wartościami, zapisane osoby
  // i puste wiersze do wpisania ręcznie (style @media print w ui.scss)
  var printBtn = document.getElementById('fill-print');
  if (printBtn) printBtn.addEventListener('click', function () { window.print(); });

  undoBtn.addEventListener('click', function () {
    pending = {add: {}, remove: {}};
    renderAll();
  });

  // --- zapis -----------------------------------------------------------------

  // batches - żądania: usunięcia w pierwszym, dopisania pogrupowane według
  // wartości pól wspólnych z chwili dodania
  function batches() {
    var out = [];
    var remove = Object.keys(pending.remove).map(function (k) {
      var i = k.lastIndexOf(':');
      return {field: k.slice(0, i), form: +k.slice(i + 1)};
    });
    var groups = {};
    var order = [];
    Object.keys(pending.add).forEach(function (field) {
      pending.add[field].forEach(function (p) {
        var g = JSON.stringify(p.data);
        if (!groups[g]) { groups[g] = {data: p.data, add: [], remove: []}; order.push(g); }
        groups[g].add.push({field: field, name: p.name, surname: p.surname, email: p.email});
      });
    });
    order.forEach(function (g) { out.push(groups[g]); });
    if (remove.length) {
      if (out.length) out[0].remove = remove;
      else out.push({data: collect(), add: [], remove: remove});
    }
    return out;
  }

  saveBtn.addEventListener('click', function () {
    var list = batches();
    if (!list.length) return;
    saveBtn.disabled = true;
    msgEl.hidden = true;
    var sum = {added: 0, removed: 0, mails: 0};
    var step = function (i) {
      if (i >= list.length) {
        pending = {add: {}, remove: {}};
        renderAll();
        var text = t('fill.saved', [sum.added, sum.removed]);
        if (sum.mails) text += ' ' + RZ.tN('fill.mails', sum.mails);
        RZ.toast(text);
        return;
      }
      RZ.postJSON('/api/forms/' + formID + '/fill', list[i]).then(function (resp) {
        sum.added += resp.added || 0;
        sum.removed += resp.removed || 0;
        sum.mails += resp.mails || 0;
        if (resp.people) people = resp.people;
        if (resp.contacts) contacts = resp.contacts;
        // zapisana część znika z oczekujących, reszta zostaje na wypadek błędu
        if (i === 0) pending.remove = {};
        list[i].add.forEach(function (a) {
          var arr = pending.add[a.field] || [];
          for (var j = 0; j < arr.length; j++) {
            if (arr[j].email === a.email) { arr.splice(j, 1); break; }
          }
        });
        step(i + 1);
      }).catch(function (err) {
        renderAll();
        msgEl.textContent = err.message || t('common.save_error');
        msgEl.hidden = false;
        var ff = err.field && form.querySelector('.ff[data-name="' + err.field + '"]');
        if (ff) {
          showError(ff, err.message);
          ff.scrollIntoView({behavior: 'smooth', block: 'center'});
        }
      });
    };
    step(0);
  });

  window.addEventListener('beforeunload', function (e) {
    if (pendingCount()) { e.preventDefault(); e.returnValue = ''; }
  });

  RZ.ready.then(function () {
    restoreDraft();
    conds.apply();
    renderAll();
  });
})();
