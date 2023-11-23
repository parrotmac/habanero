-- Exists solely so editors don't underline every pggen.arg() expression in
-- squiggly red.
CREATE SCHEMA IF NOT EXISTS pggen;

-- pggen.arg defines a named parameter that's eventually compiled into a
-- placeholder for a prepared query: $1, $2, etc.
CREATE FUNCTION pggen.arg(param TEXT) RETURNS text AS $$SELECT null$$ LANGUAGE sql;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

create table public.sensors
(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    identifier varchar(255) unique not null,
    type     varchar(50),
    location varchar(50)
);

alter table public.sensors
    owner to postgres;

create table public.sensor_data
(
    time        timestamp with time zone not null,
    sensor_id   uuid not null
        references public.sensors,
    moisture double precision
);

alter table public.sensor_data
    owner to postgres;

create index sensor_data_time_idx
    on public.sensor_data (time desc);

SELECT create_hypertable('sensor_data', 'time');

CREATE MATERIALIZED VIEW hourly_moisture_avg WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS one_hour_bucket,
    avg(moisture) AS average_moisture
FROM
    sensor_data
GROUP BY
    one_hour_bucket;

CREATE MATERIALIZED VIEW hourly_moisture_avg_per_sensor WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS one_hour_bucket,
    sensor_id,
    avg(moisture) AS average_moisture
FROM
    sensor_data
GROUP BY
    one_hour_bucket, sensor_id;
