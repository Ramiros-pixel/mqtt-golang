export default function StatusCard({ icon, label, value, unit, sub }) {
  return (
    <div className="status-card">
      <div className="status-card__icon">{icon}</div>
      <div className="status-card__body">
        <span className="status-card__label">{label}</span>
        <span className="status-card__value">
          {value === null || value === undefined ? '—' : value}
          {value !== null && value !== undefined && <small> {unit}</small>}
        </span>
        {sub && <span className="status-card__sub">{sub}</span>}
      </div>
    </div>
  );
}