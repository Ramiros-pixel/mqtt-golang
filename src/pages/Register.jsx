import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export default function Register() {
  const { register } = useAuth();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);
  const [loading, setLoading] = useState(false);

  const onSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await register(name, email, password);
      setSuccess(true);
    } catch (err) {
      setError(err.message || 'Registrasi gagal');
    } finally {
      setLoading(false);
    }
  };

  if (success) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <h1>✅ Pendaftaran Berhasil</h1>
          <p className="auth-card__sub">
            Akunmu sudah dibuat dan berstatus <strong>menunggu persetujuan</strong>.
            Super admin akan meninjau dan menyetujui akunmu — setelah itu kamu bisa login.
          </p>
          <Link to="/login" className="btn btn--primary btn--block">Ke Halaman Login</Link>
          <p className="auth-card__home">
            <Link to="/">← Kembali ke beranda</Link>
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={onSubmit}>
        <h1>🌿 Daftar Akun</h1>
        <p className="auth-card__sub">
          Akun baru memerlukan persetujuan super admin sebelum bisa login.
        </p>

        {error && <div className="alert alert--error">{error}</div>}

        <label>
          Nama Lengkap
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Nama kamu"
            required
            autoFocus
          />
        </label>
        <label>
          Email
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="nama@email.com"
            required
          />
        </label>
        <label>
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Minimal 6 karakter"
            minLength={6}
            required
          />
        </label>

        <button type="submit" className="btn btn--primary btn--block" disabled={loading}>
          {loading ? 'Memproses…' : 'Daftar'}
        </button>

        <p className="auth-card__switch">
          Sudah punya akun? <Link to="/login">Masuk di sini</Link>
        </p>
        <p className="auth-card__home">
          <Link to="/">← Kembali ke beranda</Link>
        </p>
      </form>
    </div>
  );
}