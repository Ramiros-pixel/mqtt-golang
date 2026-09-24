import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import useSmartHome from '../hooks/useSmartHome';
import { useAuth } from '../context/AuthContext';
import { api } from '../services/api';
import StatusCard from '../components/StatusCard';
import LampControl from '../components/LampControl';
import ShadingControl from '../components/ShadingControl';
import HistoryChart from '../components/HistoryChart';

export default function Dashboard() {
  const { user, isAdmin, logout } = useAuth();
  const navigate = useNavigate();
  const { status, reading, history, lampOn, lampKnown, shadingMode, shadingDegree, shadingKnown, rain, rainKnown, brokerConnected, lastUpdate, toggleLamp, updateShading } =
    useSmartHome();
  const [toast, setToast] = useState('');

  // Validasi token saat masuk halaman: kalau akun di-reject/dihapus admin,
  // API /me akan menolak dan kita keluar otomatis.
  useEffect(() => {
    api
      .me()
      .then((d) => {
        if (d.user && d.user.status !== 'approved') {
          logout();
          navigate('/login');
        }
      })
      .catch(() => {
        logout();
        navigate('/login');
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleToggle = async () => {
    try {
      await toggleLamp();
    } catch (err) {
      setToast(err.message || 'Perintah lampu gagal terkirim');
      setTimeout(() => setToast(''), 4000);
    }
  };

  const handleShadingUpdate = async (mode, degree) => {
    try {
      await updateShading(mode, degree);
    } catch (err) {
      setToast(err.message || 'Perintah shading gagal terkirim');
      setTimeout(() => setToast(''), 4000);
    }
  };

  const wsBadge =
    status === 'online'
      ? { text: 'Realtime: terhubung', cls: 'badge--ok' }
      : status === 'connecting'
        ? { text: 'Menghubungkan…', cls: 'badge--warn' }
        : { text: 'Realtime: terputus', cls: 'badge--err' };

  const brokerBadge = brokerConnected
    ? { text: 'Broker MQTT: online', cls: 'badge--ok' }
    : { text: 'Broker MQTT: offline', cls: 'badge--err' };

  return (
    <div className="app">
      <header className="app__header">
        <div>
          <h1>🌿 Smart Home Dashboard</h1>
          <p className="app__subtitle">
            Halo, <strong>{user?.name || user?.email}</strong> · Ruangan: Ruang Tamu · 1 Lampu
          </p>
        </div>
        <div className="app__header-right">
          <span className={`badge ${wsBadge.cls}`}>{wsBadge.text}</span>
          <span className={`badge ${brokerBadge.cls}`}>{brokerBadge.text}</span>
          <div className="app__user">
            {isAdmin && (
              <Link to="/admin" className="btn btn--ghost btn--sm">
                ⚙️ Admin
              </Link>
            )}
            <button
              className="btn btn--ghost btn--sm"
              onClick={() => {
                logout();
                navigate('/');
              }}
            >
              Keluar
            </button>
          </div>
        </div>
      </header>

      {toast && <div className="alert alert--error alert--floating">{toast}</div>}

      <main className="app__grid">
        <section className="panel panel--stats">
          <h2>Status Ruangan</h2>
          <div className="stats-grid">
            <StatusCard icon="🌡️" label="Suhu" value={reading.temperature} unit="°C" />
            <StatusCard icon="💧" label="Kelembaban" value={reading.humidity} unit="%" />
            <StatusCard
              icon="💡"
              label="Lampu"
              value={lampKnown ? (lampOn ? 'ON' : 'OFF') : '—'}
              unit=""
              sub={
                lampKnown
                  ? lastUpdate
                    ? `Update terakhir: ${new Date(lastUpdate).toLocaleTimeString('id-ID')}`
                    : 'Status terakhir dari device'
                  : 'Belum ada laporan dari device'
              }
            />
            <StatusCard
              icon="🌧️"
              label="Hujan"
              value={rainKnown ? (rain ? 'Ya' : 'Tidak') : '—'}
              unit=""
              sub={rainKnown ? 'Sensor hujan dari device' : 'Belum ada laporan dari device'}
            />
          </div>
        </section>

        <section className="panel panel--lights">
          <h2>Kontrol Lampu</h2>
          <LampControl name="Lampu Ruang Tamu" isOn={lampOn} onToggle={handleToggle} />
          <p className="panel__hint">
            Perintah dikirim backend ke topik <code>esp32/monitor/controlling</code> (payload
            ON/OFF) via MQTT.
          </p>
        </section>

        <section className="panel panel--shading">
          <h2>Kontrol Shading</h2>
          <ShadingControl
            mode={shadingMode}
            degree={shadingDegree}
            known={shadingKnown}
            rain={rain}
            rainKnown={rainKnown}
            onUpdate={handleShadingUpdate}
            disabled={!brokerConnected}
          />
          <p className="panel__hint">
            Mode manual: atur derajat servo 0-180°. Mode auto: servo mengikuti sensor hujan (topik{' '}
            <code>esp32/monitor/shading</code>).
          </p>
        </section>

        <section className="panel panel--history">
          <h2>History Suhu &amp; Kelembaban</h2>
          <HistoryChart data={history} />
        </section>
      </main>

      <footer className="app__footer">
        Topik sensor: <code>esp32/monitor/monitoring</code> · Backend: <code>Go :8080</code> · Data
        tersimpan di MySQL
      </footer>
    </div>
  );
}