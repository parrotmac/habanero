
-- name: HourlyMoistureAverageForSensor :many
SELECT *
FROM hourly_moisture_avg_per_sensor
WHERE sensor_id = pggen.arg('sensor_id');

-- name: HourlyMoistureAverageForSensorAndDate :many
SELECT *
FROM hourly_moisture_avg_per_sensor
WHERE sensor_id = pggen.arg('sensor_id')
  AND one_hour_bucket >= pggen.arg('date')
  AND one_hour_bucket < (pggen.arg('date') + ('1 day'::interval))::timestamp;


-- name: T24HourlyMoistureAverageForSensor :many
SELECT *
FROM hourly_moisture_avg_per_sensor
WHERE sensor_id = pggen.arg('sensor_id')
  AND one_hour_bucket >= (pggen.arg('date')::timestamp - '24 hours'::interval)::timestamp
  AND one_hour_bucket < pggen.arg('date');


-- name: T24FromNowHourlyMoistureAverageForSensor :many
SELECT *
FROM hourly_moisture_avg_per_sensor
WHERE sensor_id = pggen.arg('sensor_id')
  AND one_hour_bucket >= (now() - '24 hours'::interval)::timestamp
  AND one_hour_bucket < now();


-- name: IndividualSensorReadingsInTimeRange :many
SELECT *
FROM sensor_data
WHERE sensor_id = pggen.arg('sensor_id')
  AND time >= pggen.arg('start_time')
  AND time < pggen.arg('end_time');

-- name: ListSensors :many
SELECT *
FROM sensors;

-- name: GetOrCreateSensor :one
INSERT INTO sensors (identifier, type, location)
VALUES (pggen.arg('identifier'), pggen.arg('type'), pggen.arg('location'))
ON CONFLICT (identifier) DO UPDATE SET
                                       type = EXCLUDED.type,
                                       location = EXCLUDED.location
RETURNING *;

-- name: GetSensor :one
SELECT *
FROM sensors
WHERE id = pggen.arg('id');

-- name: InsertReading :one
INSERT INTO sensor_data (sensor_id, time, moisture)
VALUES (pggen.arg('sensor_id'), pggen.arg('time'), pggen.arg('moisture'))
RETURNING *;
