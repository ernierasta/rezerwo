// n_formconds.js - pola warunkowe i zależne opcje (formconds.go) na stronie
// gościa (n_formpage.js) i w trybie próby designera pól (n_fields.js).
//
// Pola to .ff z data-name i data-type. Warunek pokazania (własny albo
// sekcji) jest w data-show='{"field": "klasa", "values": ["1A"]}', pole
// z opcjami zależnymi ma data-dep="nazwa rodzica", a jego opcje (label.opt
// albo <option>) data-when="wartość rodzica" (pusty = przy każdym wyborze).
// Ukryte pole dostaje hidden; wartość zostaje, ale gość jej nie wysyła.

(function () {
  'use strict';

  window.RZ = window.RZ || {};

  // fieldValues - zaznaczone/wpisane wartości pola (bez pustych)
  function fieldValues(ff) {
    var checks = ff.querySelectorAll('input[type="radio"], input[type="checkbox"]');
    if (checks.length) {
      return Array.prototype.filter.call(checks, function (i) { return i.checked; })
        .map(function (i) { return i.value; });
    }
    var sel = ff.querySelector('select');
    if (sel) return Array.prototype.map.call(sel.selectedOptions, function (o) { return o.value; });
    var el = ff.querySelector('input, textarea');
    return el ? [el.value] : [];
  }

  function hit(vals, set) {
    return vals.some(function (v) { return set.indexOf(v) >= 0; });
  }

  function allowed(el, parent) {
    var w = el.getAttribute('data-when');
    return !w || parent.indexOf(w) >= 0;
  }

  // RZ.formConds(form, valueOf) - valueOf(ff) zwraca listę wartości pola
  // (domyślnie fieldValues). Zwraca {apply}; apply() przelicza widoczność
  // i oddaje true, jeśli coś się zmieniło.
  RZ.formConds = function (form, valueOf) {
    valueOf = valueOf || fieldValues;
    var byName = {};
    Array.prototype.forEach.call(form.querySelectorAll('.ff[data-name]'), function (ff) {
      byName[ff.dataset.name] = ff;
    });
    var conds = Array.prototype.map.call(form.querySelectorAll('.ff[data-show], .ff[data-dep]'), function (ff) {
      var c = null;
      try { c = ff.dataset.show ? JSON.parse(ff.dataset.show) : null; } catch (e) { c = null; }
      var sel = ff.dataset.dep ? ff.querySelector('select') : null;
      return {
        ff: ff, c: c, dep: ff.dataset.dep || '', sel: sel,
        // opcje listy zależnej: zdejmujemy je z <select>, bo ukryte <option>
        // część przeglądarek i tak pokazuje
        opts: sel ? Array.prototype.slice.call(sel.querySelectorAll('option[data-when]')) : null
      };
    });
    if (!conds.length) return {apply: function () { return false; }};

    function vals(name) {
      var ff = byName[name];
      if (!ff || ff.hidden) return [];
      return valueOf(ff).filter(function (v) { return v !== ''; });
    }

    // filterOpts zostawia opcje pasujące do wyboru rodzica; oddaje
    // [czy coś się zmieniło, czy jest co pokazać]
    function filterOpts(x, parent) {
      var changed = false, any = false;
      if (x.sel) {
        var want = x.opts.filter(function (o) { return allowed(o, parent); });
        var cur = Array.prototype.slice.call(x.sel.querySelectorAll('option[data-when]'));
        if (cur.length !== want.length || cur.some(function (o, i) { return o !== want[i]; })) {
          x.opts.forEach(function (o) {
            if (want.indexOf(o) < 0) {
              o.selected = false;
              if (o.parentNode) o.remove();
            }
          });
          want.forEach(function (o) { x.sel.appendChild(o); });
          changed = true;
        }
        any = want.length > 0;
      } else {
        Array.prototype.forEach.call(x.ff.querySelectorAll('.opt[data-when]'), function (l) {
          var ok = allowed(l, parent);
          if (l.hidden === ok) {
            l.hidden = !ok;
            var inp = l.querySelector('input');
            if (!ok && inp && inp.checked) inp.checked = false;
            changed = true;
          }
          if (ok) any = true;
        });
        if (x.ff.querySelector('.opt.other')) any = true;
      }
      return [changed, any];
    }

    function apply() {
      var changedAny = false;
      // warunki wskazują pola wyżej, ale wiersze z polami obok siebie mogą
      // mieć inną kolejność niż definicja - liczymy do skutku
      for (var pass = 0; pass < 20; pass++) {
        var changed = false;
        conds.forEach(function (x) {
          var show = !x.c || hit(vals(x.c.field), x.c.values || []);
          if (show && x.dep) {
            var parent = vals(x.dep);
            var r = filterOpts(x, parent);
            if (r[0]) changed = true;
            show = parent.length > 0 && r[1];
          }
          if (x.ff.hidden === show) {
            x.ff.hidden = !show;
            changed = true;
          }
        });
        if (!changed) break;
        changedAny = true;
      }
      Array.prototype.forEach.call(form.querySelectorAll('.ff-row'), function (row) {
        var ff = row.querySelectorAll('.ff');
        row.hidden = ff.length > 0 && Array.prototype.every.call(ff, function (f) { return f.hidden; });
      });
      return changedAny;
    }

    apply();
    return {apply: apply};
  };
})();
