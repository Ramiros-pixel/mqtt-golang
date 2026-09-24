import { API_BASE, TOKEN_KEY } from '../config/api';

// Helper fetch JSON dengan Authorization Bearer otomatis.
async function request(path, { method = 'GET', body, token } = {}) {
  const headers = { 'Content-Type': 'application/json' };
  const t = token ?? localStorage.getItem(TOKEN_KEY);
  if (t) headers.Authorization = `Bearer ${t}`;

  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  let data = null;
  try {
    data = await res.json();
  } catch {
    // respons tanpa body
  }

  if (!res.ok) {
    const msg = (data && data.error) || `HTTP ${res.status}`;
    const err = new Error(msg);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

export const api = {
  // ---- auth ----
  register: (payload) => request('/api/auth/register', { method: 'POST', body: payload }),
  login: (payload) => request('/api/auth/login', { method: 'POST', body: payload }),
  me: () => request('/api/auth/me'),

  // ---- data sensor & lampu ----
  readings: (limit = 100, hours = 24) => request(`/api/readings?limit=${limit}&hours=${hours}`),
  latestReading: () => request('/api/readings/latest'),
  lampStatus: () => request('/api/lamp'),
  setLamp: (on) => request('/api/lamp', { method: 'POST', body: { on } }),
  setShading: (mode, degree) => request('/api/shading', { method: 'POST', body: { mode, degree } }),
  systemStatus: () => request('/api/status'),

  // ---- admin ----
  listUsers: (status = '') => request(`/api/admin/users${status ? `?status=${status}` : ''}`),
  approveUser: (id) => request(`/api/admin/users/${id}/approve`, { method: 'POST' }),
  rejectUser: (id) => request(`/api/admin/users/${id}/reject`, { method: 'POST' }),
  deleteUser: (id) => request(`/api/admin/users/${id}`, { method: 'DELETE' }),
};