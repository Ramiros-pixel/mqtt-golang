import { useCallback, useEffect, useRef, useState } from 'react';
import { WS_URL, TOKEN_KEY, MAX_HISTORY_POINTS } from '../config/api';
import { api } from '../services/api';

const pushHistory = (prev, point) => {
  const next = [...prev, point];
  return next.length > MAX_HISTORY_POINTS ? next.slice(-MAX_HISTORY_POINTS) : next;
};

// Hook utama dashboard: ambil data dari backend Go via REST + WebSocket
// (backend yang menjembatani broker MQTT — browser tidak connect MQTT langsung).
export default function useSmartHome() {
  const [status, setStatus] = useState('connecting'); // connecting | online | offline
  const [reading, setReading] = useState({ temperature: null, humidity: null });
  const [history, setHistory] = useState([]);
  const [lampOn, setLampOn] = useState(false);
  const [lampKnown, setLampKnown] = useState(false); // false = belum ada laporan dari device
  const [shadingMode, setShadingMode] = useState('manual');
  const [shadingDegree, setShadingDegree] = useState(90);
  const [shadingKnown, setShadingKnown] = useState(false);
  const [rain, setRain] = useState(false);
  const [rainKnown, setRainKnown] = useState(false);
  const [brokerConnected, setBrokerConnected] = useState(false);
  const [lastUpdate, setLastUpdate] = useState(null);

  const wsRef = useRef(null);

  // Muat data awal: history + status lampu + status sistem
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [hist, lamp, sys] = await Promise.all([
          api.readings(100),
          api.lampStatus(),
          api.systemStatus(),
        ]);
        if (cancelled) return;
        setHistory((hist.readings || []).map((r) => ({ time: r.time, temperature: r.temperature, humidity: r.humidity })));
        if (hist.readings?.length) {
          const last = hist.readings[hist.readings.length - 1];
          setReading({ temperature: last.temperature, humidity: last.humidity });
          setLastUpdate(last.time);
        }
        if (lamp.state === 'on' || lamp.state === 'off') {
          setLampOn(lamp.state === 'on');
          setLampKnown(true);
        }
        if (sys.shading_known) {
          setShadingKnown(true);
          setShadingMode(sys.shading_mode || 'manual');
          setShadingDegree(sys.shading_degree ?? 90);
        }
        setBrokerConnected(!!sys.broker_connected);
      } catch {
        // backend belum siap — WS tetap dicoba
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // WebSocket realtime
  useEffect(() => {
    let disposed = false;
    let retryTimer = null;
    let ws = null;

    const connect = () => {
      const token = localStorage.getItem(TOKEN_KEY);
      if (!token || disposed) return;
      ws = new WebSocket(`${WS_URL}?token=${encodeURIComponent(token)}`);
      wsRef.current = ws;

      ws.onopen = () => {
        if (!disposed) setStatus('online');
      };
      ws.onmessage = (ev) => {
        if (disposed) return;
        try {
          const msg = JSON.parse(ev.data);
          if (msg.type === 'reading') {
            setReading({ temperature: msg.temperature, humidity: msg.humidity });
            setLastUpdate(msg.time ?? Date.now());
            setHistory((prev) => pushHistory(prev, { time: msg.time ?? Date.now(), temperature: msg.temperature, humidity: msg.humidity }));
            if (typeof msg.rain_sensor === 'boolean') {
              setRain(msg.rain_sensor);
              setRainKnown(true);
            }
          } else if (msg.type === 'rain_status') {
            setRain(!!msg.rain_sensor);
            setRainKnown(true);
          } else if (msg.type === 'lamp_status') {
            setLampKnown(true);
            setLampOn(msg.state === 'on');
          } else if (msg.type === 'shading_status') {
            setShadingKnown(true);
            setShadingDegree(msg.degree ?? 90);
            setShadingMode(msg.mode ?? 'manual');
          }
        } catch {
          // abaikan pesan bukan JSON
        }
      };
      ws.onclose = () => {
        if (disposed) return;
        setStatus('offline');
        retryTimer = setTimeout(connect, 3000);
      };
      ws.onerror = () => ws.close();
    };

    connect();

    return () => {
      disposed = true;
      clearTimeout(retryTimer);
      if (ws) ws.close();
      wsRef.current = null;
    };
  }, []);

  const toggleLamp = useCallback(async () => {
    const target = !lampOn;
    setLampOn(target); // optimistic; konfirmasi datang via WS lamp_status
    try {
      await api.setLamp(target);
      setLampKnown(true);
    } catch (err) {
      setLampOn(!target); // kembalikan kalau gagal
      throw err;
    }
  }, [lampOn]);

  const updateShading = useCallback(async (mode, degree) => {
    setShadingMode(mode);
    if (mode === 'manual') setShadingDegree(degree);
    try {
      await api.setShading(mode, degree);
      setShadingKnown(true);
    } catch (err) {
      throw err;
    }
  }, []);

  return {
    status,
    reading,
    history,
    lampOn,
    lampKnown,
    shadingMode,
    shadingDegree,
    shadingKnown,
    rain,
    rainKnown,
    brokerConnected,
    lastUpdate,
    toggleLamp,
    updateShading,
  };
}