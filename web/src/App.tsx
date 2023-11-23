import './App.css'
import {createConnectTransport} from "@bufbuild/connect-web";
import {createPromiseClient} from "@bufbuild/connect";
import {SensorService} from "./gen/proto/habanero/v1/main_connect.ts";
import {BucketedChart} from "./BucketedChart.tsx";
import {useEffect, useState} from "react";
import {Sensor, SensorReading} from "./gen/proto/habanero/v1/main_pb.ts";
import {Timestamp} from "@bufbuild/protobuf";


export const anonymousTransport = () => createConnectTransport({
    baseUrl: window.location.origin,
    useBinaryFormat: false,
    credentials: "include",
});

const getSensors = async () => {
    return (await createPromiseClient(SensorService, anonymousTransport()).getSensors({})).sensors;
}

interface SensorChooserProps {
    onSensorSelected: (sensor: Sensor) => void;
}

function SensorChooser({onSensorSelected}: SensorChooserProps) {
    const [sensors, setSensors] = useState<Sensor[]>([]);
    useEffect(() => {
        getSensors().then(setSensors);
    }, []);

    return (
        <div className={'flex flex-row'}>
            <select onChange={(e) => {
                const sensorID = e.target.value;
                const sensor = sensors.find((sensor) => sensor.id === sensorID);
                if (!sensor) return;
                onSensorSelected(sensor);
            }}>
                <option value={''}>Select a sensor</option>
                {sensors.map((sensor: Sensor, index) => {
                    return <option key={index} value={sensor.id}>{sensor.identifier}</option>
                })}
            </select>
        </div>
    )
}

function AggregateToggle({onAggregateToggled}: { onAggregateToggled: (aggregate: boolean) => void }) {
    return (
        <div className={'flex flex-row'}>
            <input defaultChecked={true} type={'checkbox'} onChange={(e) => onAggregateToggled(e.target.checked)}/>
            <label>Aggregate</label>
        </div>
    )
}

function ControlsSection({sensorId}: { sensorId: string }) {
    const [waterActivationSeconds, setWaterActivationSeconds] = useState<number>(10);

    const activateWater = async () => {
        await createPromiseClient(SensorService, anonymousTransport()).activateWatering({
            sensorId,
            durationMs: BigInt(waterActivationSeconds * 1000),
        });
    }

    return (
        <div className={'flex flex-row'}>
            <div className={'flex flex-col'}>
                <div className={'text-xl'}>Controls</div>
                <div className={'flex flex-row'}>
                    <div className={'flex flex-col'}>
                        <div className={'text-lg'}>Water</div>
                        <input type={'range'} min={0} max={60} value={waterActivationSeconds} onChange={(e) => {
                            setWaterActivationSeconds(parseInt(e.target.value));
                        }}/>
                        <button onClick={activateWater}
                            className={'bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded'}
                        >Activate for {waterActivationSeconds} second{waterActivationSeconds === 1 ? '' : 's'}</button>
                    </div>
                </div>
            </div>
        </div>
    )
}

function App() {
    const [sensor, setSensor] = useState<Sensor | null>(null);
    const [aggregateSensorReadings, setAggregateSensorReadings] = useState<SensorReading[]>([]);
    const [aggregate, setAggregate] = useState<boolean>(true);

    useEffect(() => {
        if (!sensor) return;
        console.log('setting up interval for sensor', sensor);
        if (aggregate) {
            const interval = setInterval(async () => {
                const readings = (await createPromiseClient(SensorService, anonymousTransport()).getSensorReadings({
                    sensorId: sensor.id,
                })).readings;
                setAggregateSensorReadings(readings);
            }, 2000);
            return () => clearInterval(interval);
        } else {
            const interval = setInterval(async () => {
                const readings = (await createPromiseClient(SensorService, anonymousTransport()).getIndividualSensorReadings({
                    sensorId: sensor.id,
                    start: Timestamp.fromDate(new Date(Date.now() - 1000 * 60 * 60 * 24)),
                    end: Timestamp.fromDate(new Date())
                })).readings;
                setAggregateSensorReadings(readings.reverse());
            }, 2000);
            return () => clearInterval(interval);
        }
    }, [sensor]);

    return (
        <div className={'bg-gray-300'}>
            <div className={'text-2xl'}>habanero</div>
            <div className={'text-xl'}>Sensor: {sensor?.identifier}</div>
            {sensor
                && <ControlsSection sensorId={sensor?.id} />
            }
            <AggregateToggle onAggregateToggled={setAggregate}/>
            <SensorChooser onSensorSelected={setSensor}/>
            <BucketedChart sensorReadings={aggregateSensorReadings}/>
        </div>
    )
}

export default App
