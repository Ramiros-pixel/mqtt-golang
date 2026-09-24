// ============================================================
// Konfigurasi koneksi ke backend Go — ubah di sini saja
// ============================================================

export const API_BASE = '/api';

// WebSocket realtime (sama dengan API_BASE tapi protokol ws)
export const WS_URL = 
`${window.location.protocol === 'https:' ? 'wss':'ws'}://${window.location.host}/ws`;

export const TOKEN_KEY = 'smarthome_token';
export const USER_KEY = 'smarthome_user';
// Jumlah maksimum titik history di memori
export const MAX_HISTORY_POINTS = 500;