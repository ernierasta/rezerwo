// n_form.js - strona formularza w panelu (tmpl/n_form.html): harmonijka,
// status otwarty/zamknięty, kopiowanie linku, generator ✨ panelu
// informacyjnego i kasowanie formularza.

document.addEventListener('DOMContentLoaded', function () {
  'use strict';

  var page = document.getElementById('form-page');
  var formID = page.dataset.formId;
  var t = RZ.t;
  var panel = infoPanel();
  var secs = RZ.sections(page, '/api/forms/' + formID + '/section', {
    draftKey: 'rezerwo.form.' + formID + '.draft',
    editorToolbar: panel.toolbar,
    editorIn: panel.toEditor,
    editorOut: panel.fromEditor,
    onEditor: panel.onEditor
  });

  RZ.loadLocale(document.documentElement.lang || 'pl').then(function () {
    var acc = RZ.accordion(page, 'rezerwo.form.' + formID + '.sections');
    secs.restore(acc); // niezapisane zmiany sprzed wyjścia ze strony
    syncStatus();
    panel.syncMode();
  });

  // --- panel informacyjny ----------------------------------------------------
  //
  // Tryby: brak / automatycznie / wybrane pola / własny tekst (form_stats.go).
  // We własnym tekście zestawienia są pastylkami ("Σ Klasa"), a w bazie
  // zostają akcje szablonu: nowe {{.Summary "pole"}}, {{.OptionCount "pole"
  // "opcja"}}, {{.Submissions}} i stare {{.Sum ...}}, które wczytujemy bez zmian.
  function infoPanel() {
    var box = document.getElementById('panel-set');
    var fields = [];
    try { fields = JSON.parse(box.dataset.fields || '[]'); } catch (e) { fields = []; }
    var modeEl = document.getElementById('f-panel-mode');
    var preview = document.getElementById('panel-preview');
    var textBox = document.getElementById('panel-text');
    var quill = null;

    var CODES = ['Submissions', 'Summary', 'OptionCount', 'Sum', 'SumAlfa', 'SumStrings', 'SumOnly'];
    var ACTION = new RegExp('\\{\\{\\s*\\.(' + CODES.join('|') + ')((?:\\s+"(?:[^"\\\\]|\\\\.)*")*)\\s*\\}\\}', 'g');
    var ARG = /"(?:[^"\\]|\\.)*"/g;

    function field(name) {
      for (var i = 0; i < fields.length; i++) {
        if (fields[i].name === name || fields[i].label === name) return fields[i];
      }
      return null;
    }

    function chipLabel(code, args) {
      if (code === 'Submissions' || (code === 'Sum' && args[0] === 'forms')) return 'Σ ' + t('panel.submissions');
      var f = field(args[0]);
      var label = f ? f.label : args[0];
      if (args[1] !== undefined) label += ': ' + args[1];
      return 'Σ ' + label;
    }

    function actionText(code, args) {
      return '{{.' + code + args.map(function (a) { return ' ' + JSON.stringify(a); }).join('') + '}}';
    }

    var Embed = Quill.import('blots/embed');
    class PanelVar extends Embed {
      static create(value) {
        var node = super.create();
        var args = value.args || [];
        node.dataset.code = value.code;
        node.dataset.args = JSON.stringify(args);
        node.textContent = chipLabel(value.code, args);
        var known = value.code === 'Submissions' || args[0] === 'forms' || field(args[0]);
        if (!known) node.classList.add('bad');
        return node;
      }
      static value(node) {
        var args = [];
        try { args = JSON.parse(node.dataset.args || '[]'); } catch (e) { args = []; }
        return {code: node.dataset.code, args: args};
      }
    }
    PanelVar.blotName = 'panelvar';
    PanelVar.tagName = 'span';
    PanelVar.className = 'pvar';
    Quill.register(PanelVar);

    function toEditor(name, html) {
      if (name !== 'infopanel') return html;
      var tpl = document.createElement('template');
      tpl.innerHTML = html || '';
      var walker = document.createTreeWalker(tpl.content, NodeFilter.SHOW_TEXT);
      var texts = [];
      while (walker.nextNode()) texts.push(walker.currentNode);
      texts.forEach(function (node) {
        var s = node.data;
        ACTION.lastIndex = 0;
        if (!ACTION.test(s)) return;
        ACTION.lastIndex = 0;
        var frag = document.createDocumentFragment();
        var last = 0, m;
        while ((m = ACTION.exec(s)) !== null) {
          var args = (m[2].match(ARG) || []).map(function (a) {
            try { return JSON.parse(a); } catch (e) { return a.slice(1, -1); }
          });
          frag.appendChild(document.createTextNode(s.slice(last, m.index)));
          var span = document.createElement('span');
          span.className = 'pvar';
          span.dataset.code = m[1];
          span.dataset.args = JSON.stringify(args);
          span.textContent = chipLabel(m[1], args);
          frag.appendChild(span);
          last = m.index + m[0].length;
        }
        frag.appendChild(document.createTextNode(s.slice(last)));
        node.parentNode.replaceChild(frag, node);
      });
      return tpl.innerHTML;
    }

    function fromEditor(name, q) {
      if (name !== 'infopanel') return RZ.editorHTML(q);
      var rootEl = RZ.editorDOM(q.root.cloneNode(true));
      var actions = [];
      rootEl.querySelectorAll('.pvar').forEach(function (node) {
        var args = [];
        try { args = JSON.parse(node.dataset.args || '[]'); } catch (e) { args = []; }
        actions.push(actionText(node.dataset.code, args));
        node.replaceWith(document.createTextNode('\u2063PV' + (actions.length - 1) + '\u2063'));
      });
      return rootEl.innerHTML.replace(/\u2063PV(\d+)\u2063/g, function (all, i) { return actions[+i]; });
    }

    function toolbar(name, tb) {
      if (name !== 'infopanel') return tb;
      return {container: tb.concat([['panelvar']]), handlers: {panelvar: function () { openVarMenu(); }}};
    }

    function onEditor(name, q) {
      if (name !== 'infopanel') return;
      quill = q;
      var b = q.getModule('toolbar').container.querySelector('.ql-panelvar');
      if (b) {
        b.textContent = 'Σ ' + t('panel.insert');
        b.classList.add('ql-wide');
        b.title = t('panel.insert_hint');
      }
    }

    function insert(code, args, index) {
      var range = quill.getSelection(true);
      var at = index !== undefined ? index : (range ? range.index : quill.getLength());
      quill.insertEmbed(at, 'panelvar', {code: code, args: args || []}, 'user');
      quill.setSelection(at + 1, 0, 'user');
      return at + 1;
    }

    // --- menu "Wstaw zestawienie" ---
    var menu = null;
    function closeMenu() {
      if (menu) menu.remove();
      menu = null;
      document.removeEventListener('mousedown', outside, true);
    }
    function outside(e) { if (menu && !menu.contains(e.target)) closeMenu(); }

    function openVarMenu() {
      closeMenu();
      var anchor = quill.getModule('toolbar').container.querySelector('.ql-panelvar');
      menu = document.createElement('div');
      menu.className = 'mv-menu';
      function item(label, fn, cls) {
        var b = document.createElement('button');
        b.type = 'button';
        b.textContent = label;
        if (cls) b.className = cls;
        b.addEventListener('mousedown', function (e) { e.preventDefault(); });
        b.addEventListener('click', function () { closeMenu(); fn(); });
        menu.appendChild(b);
      }
      item(t('panel.submissions'), function () { insert('Submissions'); });
      menu.appendChild(document.createElement('hr'));
      fields.forEach(function (f) {
        item(f.label, function () { insert('Summary', [f.name]); });
        (f.options || []).forEach(function (o) {
          item(o, function () { insert('OptionCount', [f.name, o]); }, 'sub');
        });
      });
      if (!fields.length) {
        var p = document.createElement('p');
        p.className = 'muted';
        p.textContent = t('panel.no_fields');
        menu.appendChild(p);
      }
      document.body.appendChild(menu);
      var r = anchor.getBoundingClientRect();
      menu.style.left = Math.max(8, Math.min(r.left + window.scrollX, window.scrollX + document.documentElement.clientWidth - menu.offsetWidth - 8)) + 'px';
      menu.style.top = (r.bottom + window.scrollY + 4) + 'px';
      setTimeout(function () { document.addEventListener('mousedown', outside, true); }, 0);
    }

    // "Wstaw zestawienie wszystkich pól" - zastępuje dawny przycisk ✨
    document.getElementById('gen-info').addEventListener('click', function () {
      if (!quill) return;
      var range = quill.getSelection(true);
      var at = range ? range.index : quill.getLength() - 1;
      quill.insertText(at, t('panel.submissions') + ': ', 'user');
      at = insert('Submissions', [], at + (t('panel.submissions') + ': ').length);
      quill.insertText(at, '\n', 'user');
      at += 1;
      fields.forEach(function (f) {
        quill.insertText(at, f.label + ': ', 'user');
        at = insert('Summary', [f.name], at + (f.label + ': ').length);
        quill.insertText(at, '\n', 'user');
        at += 1;
      });
    });

    // --- tryb panelu ---
    var seg = document.getElementById('panel-seg');
    function syncMode() {
      var mode = modeEl.value || 'text';
      seg.querySelectorAll('button').forEach(function (b) { b.classList.toggle('on', b.dataset.v === mode); });
      box.querySelectorAll('.panel-hint').forEach(function (h) { h.hidden = h.dataset.mode !== mode; });
      textBox.hidden = mode !== 'text';
      preview.hidden = mode !== 'auto' && mode !== 'fields';
      if (!preview.hidden) {
        preview.innerHTML = '<p class="muted">' + t('common.loading') + '</p>';
        fetch('/api/forms/' + formID + '/infopanel?mode=' + mode)
          .then(function (r) { return r.json(); })
          .then(function (d) { preview.innerHTML = d.html || ''; })
          .catch(function () { preview.innerHTML = ''; });
      }
    }
    seg.addEventListener('click', function (e) {
      var b = e.target.closest('button');
      if (!b) return;
      modeEl.value = b.dataset.v;
      syncMode();
      modeEl.dispatchEvent(new Event('input', {bubbles: true}));
    });

    return {toolbar: toolbar, toEditor: toEditor, fromEditor: fromEditor, onEditor: onEditor, syncMode: syncMode};
  }

  // przełącznik Otwarty / Zamknięty zapisuje się w ukrytym polu sekcji
  var status = document.getElementById('f-status');
  var seg = document.getElementById('status-seg');
  function syncStatus() {
    seg.querySelectorAll('button').forEach(function (b) {
      b.classList.toggle('on', b.dataset.v === (status.value || 'open'));
    });
  }
  seg.addEventListener('click', function (e) {
    var b = e.target.closest('button');
    if (!b) return;
    status.value = b.dataset.v;
    syncStatus();
    status.dispatchEvent(new Event('input', {bubbles: true}));
  });

  document.getElementById('copy-link').addEventListener('click', function () {
    var url = location.origin + document.getElementById('form-link').textContent.trim();
    if (navigator.clipboard) navigator.clipboard.writeText(url);
    RZ.toast(t('fields.copied'));
  });

  // kasowanie: trzeba przepisać nazwę formularza
  var dlg = document.getElementById('del-dlg');
  var name = document.getElementById('del-name');
  var ok = document.getElementById('del-ok');
  var formName = document.querySelector('.dh h3').textContent.trim();
  document.getElementById('form-delete').addEventListener('click', function () {
    dlg.hidden = false;
    name.value = '';
    ok.disabled = true;
    name.focus();
  });
  name.addEventListener('input', function () {
    ok.disabled = name.value.trim() !== formName;
  });
  document.getElementById('del-cancel').addEventListener('click', function () { dlg.hidden = true; });
  dlg.addEventListener('click', function (e) { if (e.target === dlg) dlg.hidden = true; });
  ok.addEventListener('click', function () {
    ok.disabled = true;
    RZ.postJSON('/api/forms/' + formID, {}, 'DELETE')
      .then(function (resp) {
        try { localStorage.removeItem('rezerwo.form.' + formID + '.draft'); } catch (e) { /* nic */ }
        location.href = resp.next || '/admin';
      })
      .catch(function (err) {
        dlg.querySelector('.msg').textContent = err.message;
        ok.disabled = false;
      });
  });
});
