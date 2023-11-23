# boot.py -- run on boot-up
import network, utime

# Replace the following with your WIFI Credentials
SSID = "BUNKERNET"
SSID_PASSWORD = "whatchadoin"


def do_connect():
    sta_if = network.WLAN(network.STA_IF)
    if not sta_if.isconnected():
        print('connecting to network...')
        sta_if.active(True)
        sta_if.connect(SSID, SSID_PASSWORD)
        while not sta_if.isconnected():
            print("Attempting to connect....")
            utime.sleep(1)
    print('Connected! Network config:', sta_if.ifconfig())
    
print("Connecting to your wifi...")
do_connect()

# Give time to connect to chip for reprogramming
time.sleep(2)
