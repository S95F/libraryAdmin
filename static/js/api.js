// Fetch wrapper — all API calls return { success, data } or { success, error }
async function api(method, path, body) {
  const opts = {
    method,
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
  };
  if (body !== undefined) opts.body = JSON.stringify(body);
  try {
    const res = await fetch(path, opts);
    const json = await res.json();
    if (!json.success) throw new Error(json.error || 'Request failed');
    return json.data;
  } catch (e) {
    throw e;
  }
}

const API = {
  get:    (path)        => api('GET',    path),
  post:   (path, body)  => api('POST',   path, body),
  put:    (path, body)  => api('PUT',    path, body),
  delete: (path)        => api('DELETE', path),
};

// ─── Toast notifications ───────────────────────────────────────────────────
function toast(msg, type = 'success') {
  let container = document.getElementById('toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    document.body.appendChild(container);
  }
  const t = document.createElement('div');
  t.className = `toast toast-${type}`;
  t.textContent = msg;
  container.appendChild(t);
  setTimeout(() => {
    t.style.animation = 'fadeOut .3s ease forwards';
    setTimeout(() => t.remove(), 300);
  }, 3500);
}

// ─── Auth helpers ──────────────────────────────────────────────────────────
async function requireAuth(allowedRoles) {
  try {
    const user = await API.get('/api/auth/me');
    if (allowedRoles && !allowedRoles.includes(user.role)) {
      window.location = '/login.html';
      return null;
    }
    return user;
  } catch {
    window.location = '/login.html';
    return null;
  }
}

async function logout() {
  await API.post('/api/auth/logout');
  window.location = '/login.html';
}

// ─── Nav helpers ───────────────────────────────────────────────────────────
function buildTopNav(user) {
  const roleLinks = {
    user: [
      { href: '/dashboard.html', label: '🏠 Dashboard' },
      { href: '/books.html',     label: '📚 Books' },
      { href: '/requests.html',  label: '📋 My Requests' },
      { href: '/profile.html',   label: '🪪 My ID Card' },
    ],
    clerk: [
      { href: '/dashboard.html', label: '🏠 Dashboard' },
      { href: '/books.html',     label: '📚 Books' },
      { href: '/clerk.html',     label: '🖥 Checkout Desk' },
      { href: '/profile.html',   label: '🪪 My ID Card' },
    ],
    admin: [
      { href: '/dashboard.html',        label: '🏠 Dashboard' },
      { href: '/books.html',            label: '📚 Books' },
      { href: '/clerk.html',            label: '🖥 Checkout Desk' },
      { href: '/admin/index.html',      label: '⚙️ Admin' },
    ],
  };

  const links = roleLinks[user.role] || roleLinks.user;
  const cur = window.location.pathname;

  const nav = document.getElementById('main-nav');
  if (!nav) return;
  nav.innerHTML = links.map(l =>
    `<a href="${l.href}" class="${cur === l.href || cur.startsWith(l.href.replace('.html','')) ? 'active' : ''}">${l.label}</a>`
  ).join('');

  const userSpan = document.getElementById('nav-user');
  if (userSpan) {
    userSpan.innerHTML = `
      <span class="user-badge">
        <strong>${user.username}</strong>
        <span class="badge role-${user.role}">${user.role}</span>
      </span>
      <button class="btn btn-outline btn-sm" onclick="logout()">Sign out</button>
    `;
  }
}

// ─── Utility ───────────────────────────────────────────────────────────────
function formatDate(str) {
  if (!str) return '—';
  return new Date(str).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

function statusBadge(status) {
  return `<span class="badge badge-${status}">${status}</span>`;
}

function overdueBadge(checkout) {
  if (checkout.returned_at) return statusBadge('returned');
  if (checkout.is_overdue)  return `<span class="badge badge-overdue">overdue</span>`;
  return statusBadge('active');
}
