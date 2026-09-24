// n_login.js: logowanie admina (POST /api/login, jak dawniej a_login.js).
(function () {
  var form = document.getElementById('login-form');
  var msg = document.getElementById('login-msg');
  if (!form) return;

  form.addEventListener('submit', function (e) {
    e.preventDefault();
    msg.hidden = true;
    if (!form.reportValidity()) return;
    var btn = form.querySelector('button[type="submit"]');
    btn.disabled = true;
    fetch('/api/login', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({
        email: form.email.value.trim(),
        password: form.password.value,
        'remember-me': form.remember.checked ? 'on' : ''
      })
    }).then(function (r) {
      if (!r.ok) throw new Error(r.status);
      var next = new URLSearchParams(location.search).get('next');
      location.href = next && next.charAt(0) === '/' && next.charAt(1) !== '/' ? next : '/admin';
    }).catch(function () {
      msg.hidden = false;
      btn.disabled = false;
      form.password.select();
    });
  });
})();
