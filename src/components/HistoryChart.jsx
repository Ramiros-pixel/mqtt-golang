import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from 'recharts';

const formatTime = (ts) =>
  new Date(ts).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

export default function HistoryChart({ data }) {
  if (!data || data.length === 0) {
    return <p className="history-chart__empty">Belum ada data history…</p>;
  }

  return (
    <div className="history-chart">
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={data} margin={{ top: 8, right: 16, bottom: 0, left: -8 }}>
          <defs>
            <linearGradient id="gradTemp" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#22c55e" stopOpacity={0.45} />
              <stop offset="95%" stopColor="#22c55e" stopOpacity={0.02} />
            </linearGradient>
            <linearGradient id="gradHum" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#86efac" stopOpacity={0.45} />
              <stop offset="95%" stopColor="#86efac" stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="rgba(34,197,94,0.15)" />
          <XAxis
            dataKey="time"
            tickFormatter={formatTime}
            stroke="#4ade80"
            fontSize={11}
            minTickGap={40}
          />
          <YAxis yAxisId="temp" stroke="#22c55e" fontSize={11} unit="°" width={48} />
          <YAxis
            yAxisId="hum"
            orientation="right"
            stroke="#86efac"
            fontSize={11}
            unit="%"
            width={48}
          />
          <Tooltip
            labelFormatter={(ts) => new Date(ts).toLocaleString('id-ID')}
            formatter={(value, name) => [
              name === 'temperature' ? `${value} °C` : `${value} %`,
              name === 'temperature' ? 'Suhu' : 'Kelembaban',
            ]}
            contentStyle={{
              background: '#052e16',
              border: '1px solid #166534',
              borderRadius: 10,
              color: '#dcfce7',
              fontSize: 12,
            }}
          />
          <Legend
            formatter={(name) => (name === 'temperature' ? 'Suhu (°C)' : 'Kelembaban (%)')}
            wrapperStyle={{ fontSize: 12 }}
          />
          <Area
            yAxisId="temp"
            type="monotone"
            dataKey="temperature"
            stroke="#22c55e"
            strokeWidth={2}
            fill="url(#gradTemp)"
            isAnimationActive={false}
            dot={false}
          />
          <Area
            yAxisId="hum"
            type="monotone"
            dataKey="humidity"
            stroke="#86efac"
            strokeWidth={2}
            fill="url(#gradHum)"
            isAnimationActive={false}
            dot={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}