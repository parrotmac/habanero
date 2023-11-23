import time
import ubinascii
from umqtt_simple import MQTTClient
import machine
import network

# Default  MQTT_BROKER to connect to
MQTT_BROKER = "mqtt.stag9.com"
CLIENT_ID = ubinascii.hexlify(machine.unique_id())

SUBSCRIBE_TOPIC = b"habanero-controls/" + CLIENT_ID + b"/+/+"
PUBLISH_TOPIC_MOISTURE = b"habanero-status/" + CLIENT_ID + "/soil-moisture"
PUBLICH_TOPIC_PUMP_CONTROL = b"habanero-status/" + CLIENT_ID + "/pump"

# Setup built in PICO LED as Output
led_pin = machine.Pin("LED", machine.Pin.OUT)

pump_pin_1 = machine.Pin(22, machine.Pin.OUT)  # Pin 29 is GP22 on the Pico
pump_pin_2 = machine.Pin(21, machine.Pin.OUT)  # Pin 27 is GP21 on the Pico
pump_pin_3 = machine.Pin(20, machine.Pin.OUT)  # Pin 26 is GP20 on the Pico

moisture_sensor_1 = machine.ADC(26) # Pin 31
moisture_sensor_2 = machine.ADC(27) # Pin 32
moisture_sensor_3 = machine.ADC(28) # Pin 34

last_publish = time.time()
publish_interval = 5

outgoing_pump_messages = []

def led_control(on):
    led_pin.value(1 if on else 0)


def pump_control(pump_id, on):
    if pump_id == "1":
        pump_pin_1.value(1 if on else 0)
    if pump_id == "2":
        pump_pin_2.value(1 if on else 0)
    if pump_id == "3":
        pump_pin_3.value(1 if on else 0)

def pump(pump_id, milliseconds):
    print(f"Activating pumping {pump_id} for {milliseconds} milliseconds")
    outgoing_pump_messages.append(f"ON::{pump_id}::{milliseconds}::{time.ticks_ms()}")
    shutoff_time = time.ticks_ms() + milliseconds
    pump_control(pump_id, True)
    while time.ticks_ms() < shutoff_time:
        # blink led rapidly
        led_control(True)
        time.sleep_ms(125)
        led_control(False)
        time.sleep_ms(125)
    pump_control(pump_id, False)
    print("Pump shutoff")
    outgoing_pump_messages.append(f"OFF::{pump_id}::{time.ticks_ms()}")

# Received messages from subscriptions will be delivered to this callback
def sub_cb(topic_b, msg_b):
    topic = topic_b.decode()
    msg = msg_b.decode()
    print((topic, msg))

    topic_segments = topic.split("/")
    if len(topic_segments) != 4:
        print("Invalid topic: " + topic)
        return

    if topic.split("/")[2] == "led":
        led_control(msg == "ON")

    if topic.split("/")[2] == "pump":
        pump_id = topic.split("/")[3]
        milliseconds = int(msg)
        if milliseconds < 50:
            return
        if milliseconds > 60_000:  # Maximum of 1 minute
            milliseconds = 60_000
        pump(pump_id, milliseconds)


def reset():
    print("Resetting...")
    time.sleep(5)
    machine.reset()
    
def get_soil_moisture():
    return moisture_sensor_1.read_u16() / 1000
    
def main():
    sta_if = network.WLAN(network.STA_IF)

    print("Client ID: " + str(CLIENT_ID))
    print("Subscribe Topic: " + str(SUBSCRIBE_TOPIC))
    print("Publish Topic: " + str(PUBLISH_TOPIC_MOISTURE))
    print("Publish Topic: " + str(PUBLICH_TOPIC_PUMP_CONTROL))

    print(f"Begin connection with MQTT Broker :: {MQTT_BROKER}")
    mqttClient = MQTTClient(CLIENT_ID, MQTT_BROKER, keepalive=60)
    mqttClient.set_callback(sub_cb)
    mqttClient.connect()
    mqttClient.subscribe(SUBSCRIBE_TOPIC)
    print(f"Connected to MQTT  Broker :: {MQTT_BROKER}, and waiting for callback function to be called!")
    while True:
            # Non-blocking wait for message
            mqttClient.check_msg()
            global last_publish
            if (time.time() - last_publish) >= publish_interval:
                soil_moisture = get_soil_moisture()
                mqttClient.publish(PUBLISH_TOPIC_MOISTURE, str(soil_moisture).encode())
                last_publish = time.time()

            status_messages = outgoing_pump_messages[-10:]
            outgoing_pump_messages.clear()
            for message in status_messages:
                mqttClient.publish(PUBLISH_TOPIC_MOISTURE, message.encode())

            time.sleep(1)

            if not sta_if.isconnected():
                print("Wifi connection lost, resetting...")
                reset()

def start():
    while True:
        try:
            for i in range(5):
                led_control(bool(i % 2 == 0))
                time.sleep(0.25)
            main()
        except OSError as e:
            print("Error: " + str(e))
            reset()


if __name__ == "__main__":
    start()
