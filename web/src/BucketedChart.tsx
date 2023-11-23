import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js';
import { Line } from 'react-chartjs-2';
import {SensorReading} from "./gen/proto/habanero/v1/main_pb.ts";

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend
);

export const options = {
  responsive: true,
  plugins: {
    legend: {
      position: 'top' as const,
    },
    title: {
      display: true,
      text: 'Chart.js Line Chart',
    },
  },
};

interface BucketedChartProps {
  sensorReadings: SensorReading[];
}

export function BucketedChart({ sensorReadings }: BucketedChartProps) {

    const labels = sensorReadings.map((reading) => reading.timestamp?.toDate().toLocaleTimeString() || '');
    const data = {
        labels,
        datasets: [
            {
                label: 'Moisture',
                data: sensorReadings.map((reading) => reading.moisture),
                borderColor: 'rgb(255, 99, 132)',
                backgroundColor: 'rgba(255, 99, 132, 0.5)',
            },
          ],
    }

  return <Line
    options={options} data={data}
  />;
}
