// n_formfill.js - menedżer dopisuje osoby przy funkcjach formularza
// (/admin/forms/{id}/fill, form_fill.go).
//
// Pola wspólne (np. klasa) to zwykłe pola formularza z warunkami
// (n_formconds.js); pola .ff-people mają listę osób i wiersz "dodaj osobę"
// z podpowiedziami z wcześniej wpisanych osób. Każda zmiana od razu trafia
// do bazy: „Dodaj” (albo wybór podpowiedzi) zapisuje osobę z bieżącymi
// wartościami pól wspólnych, × (kliknięte drugi raz, dla pewności) ją zdejmuje.
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
  // confirm - "pole:form" osoby, przy której × kliknięto raz (drugie
  // kliknięcie usuwa); busy - trwa zapis
  var confirm = null;
  var confirmTimer = 0;
  var busy = false;
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

  function onField(field) { return (people[field] || []).length; }

  function hasEmail(field, email) {
    email = lower(email);
    return (people[field] || []).some(function (p) { return lower(p.email) === email; });
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
      var k = key(field, p.form);
      var tag = p.by === 'guest' ? t('fill.by_guest') : p.by === 'other' ? t('fill.by_other') : '';
      var btn = null;
      if (p.mine && confirm === k) {
        btn = button(t('fill.remove_sure'), {'data-remove': p.form});
        btn.classList.add('sure');
      } else if (p.mine) {
        btn = button('×', {'data-remove': p.form, 'title': t('fill.remove'), 'aria-label': t('fill.remove')});
      }
      var el = li(p, p.mine ? 'mine' : '', tag, btn);
      el.dataset.k = k;
      list.appendChild(el);
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
    // i jeden zapasowy (ktoś może zebrać więcej chętnych), co najmniej dwa
    var blanks = Math.max(goal - n + 1, 2);
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

  function renderAll() { boxes.forEach(renderBox); }

  // setPeople - nowy stan list z serwera; świeżo dopisane wiersze na chwilę
  // się podświetlają
  function setPeople(next) {
    var known = {};
    Object.keys(people).forEach(function (f) {
      (people[f] || []).forEach(function (p) { known[key(f, p.form)] = true; });
    });
    people = next;
    renderAll();
    Array.prototype.forEach.call(form.querySelectorAll('.pp-list li[data-k]'), function (el) {
      if (!known[el.dataset.k]) RZ.hit(el);
    });
  }

  function setBusy(box, on) {
    busy = on;
    box.classList.toggle('busy', on);
    Array.prototype.forEach.call(box.querySelectorAll('.pp-add input, .pp-add button'), function (el) { el.disabled = on; });
  }

  // send - jedna zmiana od razu do bazy; błąd pokazuje przy polu, którego
  // dotyczy (wspólnym albo tej funkcji)
  function send(box, msg, done) {
    if (busy) return;
    setBusy(box, true);
    RZ.postJSON('/api/forms/' + formID + '/fill', msg).then(function (resp) {
      setBusy(box, false);
      if (resp.contacts) contacts = resp.contacts;
      if (resp.people) setPeople(resp.people);
      done(resp);
    }).catch(function (err) {
      setBusy(box, false);
      var ff = err.field && form.querySelector('.ff[data-name="' + err.field + '"]');
      showError(ff || box, err.message || t('common.save_error'));
      if (ff && ff !== box) ff.scrollIntoView({behavior: 'smooth', block: 'center'});
    });
  }

  function mailsNote(resp) { return resp.mails ? ' ' + RZ.tN('fill.mails', resp.mails) : ''; }

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
    hideSug(box);
    send(box, {data: collect(), add: [{field: field, name: p.name, surname: p.surname, email: p.email}], remove: []}, function (resp) {
      rememberContact(p);
      var inp = inputsOf(box);
      inp.name.value = inp.surname.value = inp.email.value = '';
      inp.name.focus();
      RZ.toast(t('fill.added', p.name + ' ' + p.surname) + mailsNote(resp));
    });
  }

  function removePerson(box, formId) {
    var field = box.dataset.name;
    var k = key(field, formId);
    clearTimeout(confirmTimer);
    if (confirm !== k) {
      // pierwsze kliknięcie: × zmienia się w „Na pewno usunąć?”
      confirm = k;
      renderAll();
      confirmTimer = setTimeout(function () { confirm = null; renderAll(); }, 5000);
      return;
    }
    confirm = null;
    var p = (people[field] || []).filter(function (x) { return String(x.form) === String(formId); })[0];
    send(box, {data: collect(), add: [], remove: [{field: field, form: +formId}]}, function () {
      if (p) RZ.toast(t('fill.removed', (p.name + ' ' + p.surname).trim()));
    });
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
      removePerson(box, b.getAttribute('data-remove'));
    }
  });

  // wydruk: widoczne teraz pola z wybranymi wartościami, zapisane osoby
  // i puste wiersze do wpisania ręcznie (style @media print w ui.scss)
  var printBtn = document.getElementById('fill-print');
  if (printBtn) printBtn.addEventListener('click', function () { window.print(); });

  // osoby na żywo: zgłoszenia gości i innych menedżerów co 30 s; nowe
  // wiersze na chwilę się podświetlają
  function applyLive(d) {
    if (busy || !d.people) return; // odpowiedź zapisu i tak przyniesie świeży stan
    setPeople(d.people);
  }

  RZ.ready.then(function () {
    restoreDraft();
    conds.apply();
    renderAll();
    RZ.live('/api/forms/' + formID + '/fill/people', 30000, applyLive);
  });
})();
