package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/uuid"
	_ "github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/cors"

	"github.com/parrotmac/nicemqtt"

	pb "github.com/parrotmac/habanero/gen/proto/habanero/v1"
	"github.com/parrotmac/habanero/gen/proto/habanero/v1/habanerov1connect"
	"github.com/parrotmac/habanero/models"
)

var (
	// Overwritten by ldflags
	version = "dev"
)

type serverConfig struct {
	AuthorizedOrigins []string
	BindAddress       string
	DatabaseURL       string
	Debug             bool
	MQTTHost          string
}

func buildAuthorizedOriginsList(input string) []string {
	origins := []string{}
	for _, u := range strings.Split(input, ",") {
		parsedURL, err := url.Parse(strings.TrimSpace(u))
		if err != nil {
			log.Printf("failed to parse url: %s\n", err)
			continue
		}
		// we could sort out why the URL forms documented don't seem to work, or we could just add all the permutations
		origins = append(origins, parsedURL.Hostname())
		if scheme := parsedURL.Scheme; scheme != "" {
			origins = append(origins, scheme+"://"+parsedURL.Hostname())
			if port := parsedURL.Port(); port != "" {
				origins = append(origins, scheme+"://"+parsedURL.Hostname()+":"+port)
			}
		}
	}
	return origins
}

func readServerConfig() serverConfig {
	authorizedOrigins := os.Getenv("ALLOWED_ORIGINS")

	bindAddress := os.Getenv("BIND_ADDRESS")
	port := os.Getenv("PORT")
	if bindAddress == "" && port != "" {
		bindAddress = fmt.Sprintf(":%s", port)
	} else {
		bindAddress = "0.0.0.0:5999"
	}

	debug := strings.TrimSpace(strings.ToLower(os.Getenv("DEBUG"))) == "true"

	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/habanero?sslmode=disable")

	mqttHost := getenv("MQTT_HOST", "localhost")

	cfg := serverConfig{
		BindAddress:       bindAddress,
		AuthorizedOrigins: buildAuthorizedOriginsList(authorizedOrigins),
		Debug:             debug,
		DatabaseURL:       databaseURL,
		MQTTHost:          mqttHost,
	}
	fmt.Printf("server config:\n")
	fmt.Printf("\tbind address: %s\n", cfg.BindAddress)
	fmt.Printf("\tauthorized origins: %s\n", cfg.AuthorizedOrigins)
	fmt.Printf("\tdebug: %t\n", cfg.Debug)
	fmt.Printf("\tmqtt host: %s\n", cfg.MQTTHost)
	return cfg
}

func getDatabaseHandle(ctx context.Context, connString string) (models.Querier, *pgxpool.Pool, error) {
	conn, err := pgxpool.Connect(ctx, connString)
	if err != nil {
		return nil, nil, err
	}

	return models.NewQuerier(conn), conn, nil
}

func getenv(key string, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func mustString(val string, err error) string {
	if err != nil {
		log.Fatalln(err)
	}
	return val
}

type service struct {
	db         models.Querier
	mqttClient nicemqtt.Client
}

func (s *service) ActivateWatering(ctx context.Context, c *connect.Request[pb.ActivateWateringRequest]) (*connect.Response[pb.ActivateWateringResponse], error) {
	sensorID := c.Msg.GetSensorId()
	if sensorID == "" {
		return nil, fmt.Errorf("sensor ID is required")
	}
	sensorUUID, err := uuid.Parse(sensorID)
	if err != nil {
		return nil, err
	}

	sensor, err := s.db.GetSensor(ctx, sensorUUID)
	if err != nil {
		return nil, err
	}

	activationDuration := c.Msg.GetDurationMs()
	if activationDuration == 0 {
		return nil, fmt.Errorf("activation duration is required")
	}
	if activationDuration > 60_000 {
		return nil, fmt.Errorf("activation duration cannot be more than 60 seconds")
	}

	topic := fmt.Sprintf("habanero-controls/%s/pump/3", sensor.Identifier)
	payload := []byte(strconv.Itoa(int(activationDuration)))

	err = s.mqttClient.Publish(ctx, topic, nicemqtt.QoSAtLeastOnce, false, payload)
	if err != nil {
		return nil, err
	}

	return &connect.Response[pb.ActivateWateringResponse]{
		Msg: &pb.ActivateWateringResponse{},
	}, nil
}

func (s *service) GetIndividualSensorReadings(ctx context.Context, c *connect.Request[pb.GetIndividualSensorReadingsRequest]) (*connect.Response[pb.GetIndividualSensorReadingsResponse], error) {
	sensorID := c.Msg.GetSensorId()
	if sensorID == "" {
		return nil, fmt.Errorf("sensor ID is required")
	}
	sensorUUID, err := uuid.Parse(sensorID)
	if err != nil {
		return nil, err
	}

	startTime := c.Msg.Start.AsTime()
	endTime := c.Msg.End.AsTime()
	if endTime.IsZero() {
		endTime = time.Now()
	}

	readings, err := s.db.IndividualSensorReadingsInTimeRange(ctx, models.IndividualSensorReadingsInTimeRangeParams{
		SensorID:  sensorUUID,
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		return nil, err
	}

	readingsResp := make([]*pb.SensorReading, len(readings))
	for i, reading := range readings {
		readingsResp[i] = &pb.SensorReading{
			SensorId: reading.SensorID.String(),
			Timestamp: &timestamp.Timestamp{
				Seconds: reading.Time.Unix(),
			},
			Moisture: reading.Moisture,
		}
	}

	return &connect.Response[pb.GetIndividualSensorReadingsResponse]{
		Msg: &pb.GetIndividualSensorReadingsResponse{
			Readings: readingsResp,
		},
	}, nil
}

func (s *service) handleMqttMessage(topic string, payload []byte) {

	// expect something like 'habanero-status/e6614103e70c7137/soil-moisture'
	topicParts := strings.Split(topic, "/")
	if len(topicParts) != 3 {
		log.Printf("Received MQTT message on topic %s, but it was not in the expected format\n", topic)
		return
	}
	if topicParts[0] != "habanero-status" {
		log.Printf("Received MQTT message on topic %s, but it does not start with habanero-status\n", topic)
		return
	}
	machineID := topicParts[1]
	sensorType := topicParts[2]

	if sensorType != "soil-moisture" {
		log.Printf("Received MQTT message on topic %s, but it does not have a supported sensor type\n", topic)
		return
	}

	log.Printf("Processing MQTT message on topic %s as machine %s reporting soil moisture\n", topic, machineID)

	moistureLevel, err := strconv.ParseFloat(string(payload), 64)
	if err != nil {
		log.Printf("Error parsing moisture level: %s\n", err)
		return
	}

	sensor, err := s.db.GetOrCreateSensor(context.Background(), models.GetOrCreateSensorParams{
		Identifier: machineID,
		Type:       sensorType,
		Location:   "unspecified",
	})
	if err != nil {
		log.Printf("Error getting/creating sensor from DB: %s\n", err)
		return
	}

	_, err = s.db.InsertReading(context.Background(), models.InsertReadingParams{
		SensorID: sensor.ID,
		Time:     time.Now(),
		Moisture: moistureLevel,
	})
	if err != nil {
		log.Printf("Error inserting reading to DB: %s\n", err)
		return
	}
}

func (s *service) GetSensors(ctx context.Context, c *connect.Request[pb.GetSensorsRequest]) (*connect.Response[pb.GetSensorsResponse], error) {
	sensors, err := s.db.ListSensors(ctx)
	if err != nil {
		return nil, err
	}

	sensorsResp := make([]*pb.Sensor, len(sensors))
	for i, sensor := range sensors {
		sensorsResp[i] = &pb.Sensor{
			Id:         sensor.ID.String(),
			Identifier: sensor.Identifier,
			Type:       sensor.Type,
			Location:   sensor.Location,
		}
	}

	return &connect.Response[pb.GetSensorsResponse]{
		Msg: &pb.GetSensorsResponse{
			Sensors: sensorsResp,
		},
	}, nil
}

func (s *service) GetSensorReadings(ctx context.Context, c *connect.Request[pb.GetSensorReadingsRequest]) (*connect.Response[pb.GetSensorReadingsResponse], error) {
	sensorID, err := uuid.Parse(c.Msg.SensorId)
	if err != nil {
		return nil, err
	}

	readings, err := s.db.T24FromNowHourlyMoistureAverageForSensor(ctx, sensorID)
	if err != nil {
		return nil, err
	}

	readingsResp := make([]*pb.SensorReading, len(readings))
	for i, reading := range readings {
		readingsResp[i] = &pb.SensorReading{
			SensorId: reading.SensorID.String(),
			Timestamp: &timestamp.Timestamp{
				Seconds: reading.OneHourBucket.Unix(),
			},
			Moisture: reading.AverageMoisture,
		}
	}

	return &connect.Response[pb.GetSensorReadingsResponse]{
		Msg: &pb.GetSensorReadingsResponse{
			Readings: readingsResp,
		},
	}, nil
}

var _ habanerov1connect.SensorServiceHandler = (*service)(nil)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	log.Printf("Starting habanero %s", version)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln("Could not get hostname", err)
	}

	cfg := readServerConfig()

	db, _, err := getDatabaseHandle(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalln(err)
	}

	mqttClient, err := nicemqtt.NewClient(cfg.MQTTHost, 1883, "habanero-server-"+hostname)
	if err != nil {
		log.Fatalln("Could not connect client", err)
	}

	svc := &service{
		db:         db,
		mqttClient: mqttClient,
	}

	connCtx, cancelConn := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancelConn()

	if err := mqttClient.Connect(connCtx); err != nil {
		log.Fatalln("Could not connect client", err)
	}

	log.Println("Connected")

	topic := "habanero-status/+/+"
	err = mqttClient.Subscribe(topic, nicemqtt.QoSAtLeastOnce, svc.handleMqttMessage)
	if err != nil {
		log.Fatalln("Could not subscribe to topics", err)
	}
	log.Printf("Subscribed to MQTT topic %s\n", topic)

	mux := http.NewServeMux()

	{
		baseURL, connectHandler := habanerov1connect.NewSensorServiceHandler(svc)
		mux.Handle(baseURL, connectHandler)
	}

	mux.Handle("/", http.FileServer(http.Dir("./web/dist")))

	withLogging := loggingMiddleware(mux)
	corsConfig := cors.New(cors.Options{
		AllowedOrigins: cfg.AuthorizedOrigins,
		AllowedMethods: []string{
			"GET",
			"PATCH",
			"POST",
			"OPTIONS",
		},
		AllowCredentials: true,
		AllowedHeaders: []string{
			"Origin",
			"Cookie",
			"Authorization",
			"Connect-Protocol-Version",
			"Content-Type",
		},
		Debug: cfg.Debug,
	})
	withCors := corsConfig.Handler(withLogging)

	server := &http.Server{
		Handler:      withCors,
		Addr:         cfg.BindAddress,
		ReadTimeout:  time.Second * 15,
		WriteTimeout: time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
	err = server.ListenAndServe()
	if err != nil {
		log.Fatalln(err)
	}
}
