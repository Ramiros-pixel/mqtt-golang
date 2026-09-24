import { Link } from 'react-router-dom';

export default function Landing() {
  return (
    <div className="landing">
      <nav className="landing__nav">
        <span className="landing__brand">🌿 SmartHome</span>
        <div className="landing__nav-links">
          <Link to="/login" className="btn btn--ghost">Masuk</Link>
          <Link to="/register" className="btn btn--primary">Daftar</Link>
        </div>
      </nav>

      <section className="landing__hero">
        <div className="landing__hero-text">
          <span className="landing__tag">IoT · MQTT · ESP32</span>
          <h1>
            Pantau &amp; Kendalikan Rumahmu <span className="accent">dari Mana Saja</span>
          </h1>
          <p>
            Dashboard smart home berbasis web untuk memonitor suhu dan kelembaban ruangan
            secara realtime, plus kontrol lampu satu ketukan. Data dikirim ESP32 lewat MQTT,
            diproses backend Go, dan tampil langsung di browser.
          </p>
          <div className="landing__cta">
            <Link to="/register" className="btn btn--primary btn--lg">Mulai Sekarang — Gratis</Link>
            <Link to="/login" className="btn btn--ghost btn--lg">Saya Sudah Punya Akun</Link>
          </div>
        </div>
        <div className="landing__hero-visual" aria-hidden="true">
          <div className="landing__card-preview">
            <div className="preview-row"><span>🌡️ Suhu</span><strong>27.5 °C</strong></div>
            <div className="preview-row"><span>💧 Kelembaban</span><strong>62 %</strong></div>
            <div className="preview-row"><span>💡 Lampu</span><strong className="accent">ON</strong></div>
            <div className="preview-spark">▁▂▄▃▅▇▆▅▃▂▄▆</div>
          </div>
        </div>
      </section>

      <section className="landing__features">
        <div className="feature">
          <div className="feature__icon">📊</div>
          <h3>Monitoring Realtime</h3>
          <p>Grafik suhu &amp; kelembaban diperbarui langsung lewat WebSocket, dengan history tersimpan di database.</p>
        </div>
        <div className="feature">
          <div className="feature__icon">💡</div>
          <h3>Kontrol Lampu</h3>
          <p>Nyalakan atau matikan lampu dengan satu sentuhan — perintah dikirim ke ESP32 melalui topik MQTT controlling.</p>
        </div>
        <div className="feature">
          <div className="feature__icon">🔐</div>
          <h3>Akses Terkendali</h3>
          <p>Setiap akun baru harus disetujui super admin sebelum bisa login, jadi akses sistem tetap aman.</p>
        </div>
      </section>

      <footer className="landing__footer">
        <p>© {new Date().getFullYear()} Smart Home Dashboard — Proyek IoT Kampus</p>
      </footer>
    </div>
  );
}