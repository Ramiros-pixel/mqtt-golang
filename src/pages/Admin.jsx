import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { api } from '../services/api';

const FILTERS = [
  { key: '', label: 'Semua' },
  { key: 'pending', label: 'Menunggu' },
  { key: 'approved', label: 'Disetujui' },
  { key: 'rejected', label: 'Ditolak' },
];

const STATUS_LABEL = {
  pending: { text: 'Menunggu', cls: 'status-pill--pending' },
  approved: { text: 'Disetujui', cls: 'status-pill--approved' },
  rejected: { text: 'Ditolak', cls: 'status-pill--rejected' },
};

export default function Admin() {
  const { user } = useAuth();
  const [users, setUsers] = useState([]);
  const [filter, setFilter] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busyId, setBusyId] = useState(null);

  const load = useCallback(async () => {
    try {
      const data = await api.listUsers(filter);
      setUsers(data.users || []);
      setError('');
    } catch (err) {
      setError(err.message || 'Gagal memuat daftar user');
    }
  }, [filter]);

  useEffect(() => {
    load();
    const t = setInterval(load, 10000); // auto-refresh
    return () => clearInterval(t);
  }, [load]);

  const act = async (fn, id, msg) => {
    setBusyId(id);
    setNotice('');
    try {
      await fn();
      setNotice(msg);
      await load();
    } catch (err) {
      setError(err.message || 'Aksi gagal');
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div className="app">
      <header className="app__header">
        <div>
          <h1>⚙️ Panel Admin</h1>
          <p className="app__subtitle">
            Kelola akun &amp; permission · Login sebagai <strong>{user?.email}</strong> (super
            admin)
          </p>
        </div>
        <div className="app__header-right">
          <Link to="/dashboard" className="btn btn--ghost btn--sm">
            ← Dashboard
          </Link>
        </div>
      </header>

      {error && (
        <div className="alert alert--error">
          {error}{' '}
          <button className="alert__dismiss" onClick={() => setError('')}>
            ✕
          </button>
        </div>
      )}
      {notice && (
        <div className="alert alert--success">
          {notice}{' '}
          <button className="alert__dismiss" onClick={() => setNotice('')}>
            ✕
          </button>
        </div>
      )}

      <main className="app__admin">
        <div className="admin-toolbar">
          <div className="admin-toolbar__filters">
            {FILTERS.map((f) => (
              <button
                key={f.key}
                className={`chip ${filter === f.key ? 'chip--active' : ''}`}
                onClick={() => setFilter(f.key)}
              >
                {f.label}
              </button>
            ))}
          </div>
          <button className="btn btn--ghost btn--sm" onClick={load}>
            ⟳ Muat Ulang
          </button>
        </div>

        <div className="panel">
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Nama</th>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Status</th>
                  <th>Terdaftar</th>
                  <th>Aksi</th>
                </tr>
              </thead>
              <tbody>
                {users.length === 0 && (
                  <tr>
                    <td colSpan={7} className="admin-table__empty">
                      Tidak ada user pada filter ini.
                    </td>
                  </tr>
                )}
                {users.map((u) => {
                  const st = STATUS_LABEL[u.status] || STATUS_LABEL.pending;
                  const isSelf = u.id === user?.id;
                  return (
                    <tr key={u.id}>
                      <td>{u.id}</td>
                      <td>
                        {u.name}
                        {isSelf && <span className="admin-self"> (kamu)</span>}
                      </td>
                      <td>{u.email}</td>
                      <td>
                        {u.role === 'super_admin' ? (
                          <span className="role-pill role-pill--admin">Super Admin</span>
                        ) : (
                          <span className="role-pill">User</span>
                        )}
                      </td>
                      <td>
                        <span className={`status-pill ${st.cls}`}>{st.text}</span>
                      </td>
                      <td>
                        {new Date(u.created_at).toLocaleString('id-ID', {
                          dateStyle: 'medium',
                          timeStyle: 'short',
                        })}
                      </td>
                      <td className="admin-table__actions">
                        {isSelf ? (
                          <span className="admin-table__muted">—</span>
                        ) : (
                          <>
                            {u.status !== 'approved' && (
                              <button
                                className="btn btn--approve btn--sm"
                                disabled={busyId === u.id}
                                onClick={() =>
                                  act(() => api.approveUser(u.id), u.id, `Akun ${u.email} disetujui — sekarang bisa login.`)
                                }
                              >
                                ✓ Setujui
                              </button>
                            )}
                            {u.status !== 'rejected' && (
                              <button
                                className="btn btn--reject btn--sm"
                                disabled={busyId === u.id}
                                onClick={() =>
                                  act(() => api.rejectUser(u.id), u.id, `Akun ${u.email} ditolak.`)
                                }
                              >
                                ✕ Tolak
                              </button>
                            )}
                            <button
                              className="btn btn--danger btn--sm"
                              disabled={busyId === u.id}
                              onClick={() => {
                                if (!window.confirm(`Hapus akun ${u.email}?`)) return;
                                act(() => api.deleteUser(u.id), u.id, `Akun ${u.email} dihapus.`);
                              }}
                            >
                              🗑 Hapus
                            </button>
                          </>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p className="panel__hint">
            Alur permission: registrasi → <strong>pending</strong> → super admin menyetujui →{' '}
            <strong>approved</strong> (bisa login). Akun pending/rejected ditolak saat login.
          </p>
        </div>
      </main>
    </div>
  );
}