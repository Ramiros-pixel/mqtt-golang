export default function LampControl({ name, isOn, onToggle, disabled }) {
  return (
    <div className={`lamp-control ${isOn ? 'lamp-control--on' : ''}`}>
      <div className="lamp-control__info">
        <span className="lamp-control__icon">{isOn ? '💡' : '🌑'}</span>
        <div>
          <h3>{name}</h3>
          <span className={`lamp-control__state ${isOn ? 'on' : 'off'}`}>
            {isOn ? 'MENYALA' : 'MATI'}
          </span>
        </div>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={isOn}
        className="lamp-control__switch"
        onClick={onToggle}
        disabled={disabled}
      >
        <span className="lamp-control__knob" />
      </button>
    </div>
  );
}