import { useEffect, useState } from 'react';

// Fully-controlled: state mode & derajat diambil dari props (hook useSmartHome)
// agar selalu mengikuti laporan realtime ESP32 via WebSocket.
// Nilai drag slider lokal hanya dipakai sementara selama user menyeret.
export default function ShadingControl({
  mode,
  degree,
  known,
  rain,
  rainKnown,
  onUpdate,
  disabled,
}) {
  const [dragValue, setDragValue] = useState(null);
  const [isUpdating, setIsUpdating] = useState(false);

  const isManual = mode === 'manual';
  // Tampilkan nilai drag saat menyeret, selain itu tampilkan posisi dari ESP32
  const displayDegree = dragValue !== null ? dragValue : degree;

  // Lepaskan nilai drag kalau mode berganti (mis. WS mengubah mode)
  useEffect(() => {
    setDragValue(null);
  }, [mode]);

  const send = async (m, d) => {
    setIsUpdating(true);
    try {
      await onUpdate(m, d);
    } finally {
      setIsUpdating(false);
    }
  };

  // Auto -> manual: kirim posisi terakhir yang dilaporkan ESP32 agar servo diam di tempat
  const handleModeToggle = () => {
    const newMode = isManual ? 'auto' : 'manual';
    const targetDeg = newMode === 'manual' && known ? degree : 90;
    send(newMode, targetDeg);
  };

  const handleDrag = (e) => setDragValue(parseInt(e.target.value, 10));

  const commitDrag = () => {
    if (dragValue === null) return;
    const v = dragValue;
    setDragValue(null);
    // Kirim hanya kalau nilai berubah, supaya tidak spam topic
    if (v !== degree) send('manual', v);
  };

  return (
    <div className="shading-control">
      <div className="shading-control__header">
        <div className="shading-control__icon">🪟</div>
        <div>
          <h3>Shading</h3>
          <span className="shading-control__mode">{isManual ? 'MANUAL' : 'OTOMATIS'}</span>
        </div>
        <div className="shading-control__position" title="Posisi servo saat ini dari ESP32">
          <span className="shading-control__position-label">Posisi</span>
          <span className="shading-control__position-value">
            {known ? `${degree}°` : '—'}
          </span>
        </div>
      </div>

      <div className="shading-control__body">
        <div className="shading-control__switch-row">
          <span className="shading-control__label">Mode</span>
          <button
            type="button"
            role="switch"
            aria-checked={!isManual}
            className={`mode-switch ${isManual ? '' : 'mode-switch--auto'}`}
            onClick={handleModeToggle}
            disabled={disabled || isUpdating}
          >
            <span className="mode-switch__label mode-switch__label--manual">Manual</span>
            <span className="mode-switch__knob" />
            <span className="mode-switch__label mode-switch__label--auto">Auto</span>
          </button>
        </div>

        {isManual && (
          <div className="shading-control__slider-row">
            <label htmlFor="shading-degree" className="shading-control__label">
              Derajat
            </label>
            <div className="shading-control__slider-wrapper">
              <input
                id="shading-degree"
                type="range"
                min="0"
                max="180"
                step="1"
                value={displayDegree}
                onChange={handleDrag}
                onMouseUp={commitDrag}
                onTouchEnd={commitDrag}
                disabled={disabled || isUpdating}
                className="shading-slider"
              />
              <div className="shading-control__degree-display">{displayDegree}°</div>
            </div>
          </div>
        )}

        {!isManual && (
          <p className="shading-control__hint">
            Mode otomatis aktif: servo mengikuti sensor hujan dari ESP32
            {known ? ` — posisi saat ini ${degree}°` : ''}.
          </p>
        )}

        <div className="shading-control__rain">
          <span className="shading-control__rain-label">Sensor hujan</span>
          <span className={`shading-control__rain-state ${rain ? 'detected' : ''}`}>
            {!rainKnown
              ? '—'
              : rain
                ? '🌧️ TERDETEKSI'
                : '✳️ Tidak hujan'}
          </span>
        </div>
      </div>
    </div>
  );
}
