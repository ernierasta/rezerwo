// n_copy.js - okno kopiowania imprezy.
// Korzysta z istniejącego API: GET /api/eventcopyinfo?event_id=..,
// POST /api/eventcopy. Daty proponowane rok później, na ten sam dzień tygodnia
// (52 tygodnie = 364 dni).

(function (global) {
  'use strict';

  function shiftYear(dateStr) {
    if (!dateStr) return '';
    var d = new Date(dateStr + 'T12:00:00');
    if (isNaN(d)) return dateStr;
    d.setDate(d.getDate() + 364);
    return d.toISOString().slice(0, 10);
  }

  function nextName(name) {
    // "Bal 2026" -> "Bal 2027"
    return String(name).replace(/(\d{4})/, function (y) { return String(Number(y) + 1); });
  }

  function field(label, input) {
    var f = document.createElement('div');
    f.className = 'f';
    var l = document.createElement('label');
    l.textContent = label;
    f.appendChild(l);
    f.appendChild(input);
    return f;
  }

  function input(value, type) {
    var i = document.createElement('input');
    i.className = 'inp';
    i.type = type || 'text';
    i.value = value || '';
    return i;
  }

  function open(eventID) {
    fetch('/api/eventcopyinfo?event_id=' + encodeURIComponent(eventID))
      .then(function (r) { return r.json().then(function (d) { if (!r.ok) throw new Error(d.msg || r.status); return d; }); })
      .then(showDialog)
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function showDialog(info) {
    var back = document.createElement('div');
    back.className = 'dlg-back';

    var dlg = document.createElement('div');
    dlg.className = 'dlg';
    back.appendChild(dlg);

    var h = document.createElement('h3');
    h.textContent = RZ.t('copy.title');
    dlg.appendChild(h);

    var sub = document.createElement('p');
    sub.className = 'muted';
    sub.textContent = info.name;
    dlg.appendChild(sub);

    var name = input(nextName(info.name));
    var date = input(shiftYear(info.date), 'date');
    var from = input(shiftYear(info.from_date), 'date');

    var grid = document.createElement('div');
    grid.className = 'grid2';
    grid.appendChild(field(RZ.t('field.name'), name));
    grid.appendChild(document.createElement('div'));
    grid.appendChild(field(RZ.t('field.date'), date));
    grid.appendChild(field(RZ.t('field.from_date'), from));
    dlg.appendChild(grid);

    // sale: dowiązanie tej samej sali albo jej kopia pod nową nazwą
    var roomsWrap = document.createElement('div');
    roomsWrap.style.marginTop = '14px';
    var roomsLbl = document.createElement('label');
    roomsLbl.className = 'muted';
    roomsLbl.textContent = RZ.t('copy.rooms');
    roomsWrap.appendChild(roomsLbl);

    var roomRows = [];
    (info.room_ids || []).forEach(function (id) {
      var room = (info.all_rooms || []).filter(function (r) { return r.id === id; })[0] || {id: id, name: '#' + id};
      var row = document.createElement('div');
      row.className = 'copy-room';

      var nameSpan = document.createElement('span');
      nameSpan.textContent = room.name;
      row.appendChild(nameSpan);

      var sel = document.createElement('select');
      sel.className = 'inp';
      [['link', RZ.t('copy.room_link')], ['copy', RZ.t('copy.room_copy')]].forEach(function (o) {
        var opt = document.createElement('option');
        opt.value = o[0];
        opt.textContent = o[1];
        sel.appendChild(opt);
      });
      row.appendChild(sel);

      var copyName = input(room.name + ' ' + new Date().getFullYear());
      copyName.hidden = true;
      sel.addEventListener('change', function () { copyName.hidden = sel.value !== 'copy'; });
      row.appendChild(copyName);

      roomsWrap.appendChild(row);
      roomRows.push({id: room.id, sel: sel, name: copyName});
    });
    dlg.appendChild(roomsWrap);

    // maile: kopiować czy używać tych samych
    var mailsWrap = document.createElement('div');
    mailsWrap.style.marginTop = '14px';
    var mailBox = document.createElement('input');
    mailBox.type = 'checkbox';
    var mailLbl = document.createElement('label');
    mailLbl.textContent = RZ.t('copy.mails');
    var check = document.createElement('div');
    check.className = 'check';
    check.appendChild(mailBox);
    check.appendChild(mailLbl);
    mailsWrap.appendChild(check);

    var thankName = input(info.thankyou ? nextName(info.thankyou.name) : '');
    var adminName = input(info.admin ? nextName(info.admin.name) : '');
    var mailNames = document.createElement('div');
    mailNames.className = 'grid2';
    mailNames.hidden = true;
    if (info.thankyou) mailNames.appendChild(field(RZ.t('copy.thankyou_name'), thankName));
    if (info.admin) mailNames.appendChild(field(RZ.t('copy.admin_name'), adminName));
    mailBox.addEventListener('change', function () { mailNames.hidden = !mailBox.checked; });
    mailsWrap.appendChild(mailNames);
    dlg.appendChild(mailsWrap);

    var act = document.createElement('div');
    act.className = 'sec-f';
    var ok = document.createElement('button');
    ok.className = 'btn';
    ok.textContent = RZ.t('copy.submit');
    var cancel = document.createElement('button');
    cancel.className = 'btn quiet';
    cancel.textContent = RZ.t('common.cancel');
    var msg = document.createElement('small');
    msg.className = 'msg';
    act.appendChild(ok);
    act.appendChild(cancel);
    act.appendChild(msg);
    dlg.appendChild(act);

    function close() { back.remove(); }
    cancel.addEventListener('click', close);
    back.addEventListener('click', function (e) { if (e.target === back) close(); });
    document.addEventListener('keydown', function esc(e) {
      if (e.key === 'Escape') { close(); document.removeEventListener('keydown', esc); }
    });

    ok.addEventListener('click', function () {
      ok.disabled = true;
      msg.textContent = RZ.t('common.loading');
      RZ.postJSON('/api/eventcopy', {
        event_id: info.event_id,
        name: name.value,
        date: date.value,
        from_date: from.value,
        rooms: roomRows.map(function (r) {
          return {room_id: r.id, copy: r.sel.value === 'copy', name: r.sel.value === 'copy' ? r.name.value : ''};
        }),
        copy_notifications: mailBox.checked,
        thankyou_name: thankName.value,
        admin_name: adminName.value
      }).then(function (resp) {
        close();
        if (resp.event_id) location.href = '/admin/events/' + resp.event_id;
        else location.reload();
      }).catch(function (e) {
        ok.disabled = false;
        msg.textContent = RZ.t('common.save_error') + ': ' + e.message;
        msg.classList.add('bad');
      });
    });

    document.body.appendChild(back);
    name.focus();
  }

  // wybór imprezy do skopiowania, gdy wołamy z listy (strona startowa)
  function pick(events) {
    if (!events.length) return;
    var back = document.createElement('div');
    back.className = 'dlg-back';
    var dlg = document.createElement('div');
    dlg.className = 'dlg';
    back.appendChild(dlg);

    var h = document.createElement('h3');
    h.textContent = RZ.t('copy.pick_event');
    dlg.appendChild(h);

    var sel = document.createElement('select');
    sel.className = 'inp';
    events.forEach(function (e) {
      var o = document.createElement('option');
      o.value = e.id;
      o.textContent = e.name;
      sel.appendChild(o);
    });
    dlg.appendChild(sel);

    var act = document.createElement('div');
    act.className = 'sec-f';
    var ok = document.createElement('button');
    ok.className = 'btn';
    ok.textContent = RZ.t('common.copy');
    var cancel = document.createElement('button');
    cancel.className = 'btn quiet';
    cancel.textContent = RZ.t('common.cancel');
    act.appendChild(ok);
    act.appendChild(cancel);
    dlg.appendChild(act);

    function close() { back.remove(); }
    cancel.addEventListener('click', close);
    back.addEventListener('click', function (e) { if (e.target === back) close(); });
    ok.addEventListener('click', function () { close(); open(sel.value); });

    document.body.appendChild(back);
  }

  global.RZCopy = {open: open, pick: pick};

  document.addEventListener('click', function (e) {
    var btn = e.target.closest('[data-copy-event]');
    if (btn) {
      e.preventDefault();
      open(btn.dataset.copyEvent);
      return;
    }
    if (e.target.closest('[data-copy-pick]')) {
      e.preventDefault();
      var events = Array.prototype.map.call(document.querySelectorAll('a.row.event'), function (a) {
        return {id: a.getAttribute('href').split('/').pop(), name: a.querySelector('.ename').childNodes[0].textContent.trim()};
      });
      pick(events);
    }
  });
})(window);
