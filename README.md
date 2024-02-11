# Edge-Network app

This application is meant to run on an autopi, to get a variety of information from the device over BTE (leveraging devices that have it, not 5.2 hw). 

## Building and deploying locally

Compile for the AutoPi with
```
brew install --build-from-source upx
GOARCH=arm GOOS=linux go build -ldflags="-s -w -X 'main.Version=v1.0.0'" -o edge-network && upx edge-network

```
Binaries will build [for releases](https://github.com/DIMO-Network/edge-network/releases) from the [workflow](.github/workflows/release.yaml).

Copy to the AutoPi with, e.g.,
```
scp bin/edge-network pi@192.168.4.1:~
```

Once edge-network is copied into the autopi, run this command to ssh into local autopi:

```
ssh pi@local.autopi.io
```

For newer devices [7.0 and newer] password should be the first 13 digits INCLUDING dashes of your device id. Note that this is different that the unit id, although they are the same length. 

To get a CAN dump after getting into the autopi network, run the following command to get a CAN dump from your car:

```
./edge-network - candump [-cycles <qty>] [-send <chunk_size>]
e.g.
cmd.run "edge-network candump -cycles 1000 -send 100"
....would capture 1000 frames and send them in 100 frame chunks

```
This should create a file called testcandump.txt. Permissions may need to be altered if the file should be viewed. To do this, run the following command:

```
chmod 644 testcandump.txt
cat testcandump.txt

```
This should print testcandump.txt directly to the terminal.


Note that IP is the default IP address of the AutoPi when you connect to it's wifi. You'll need to [connect to the AP local wifi.](https://docs.autopi.io/guides/guides-intro/#6-connect-to-wifi) 
to be able to run the above command successfully. `scp` will also prompt for a password, ask internal dev for it. 
This should place the executable in the home directory. Then you can replace the existing systemctl edge-network that is running by:

- `which edge-control`
- `sudo systemctl stop edge-network`
- replace the binary (you'll need to decompres `tar -xzvf` the binary if needed)
- `sudo systemctl start edge-network`

The device should be discoverable under thr usual `autopi`-prefixed name via Bluetooth.

### How this works / testing it

This runs as a systemd service. To see it's status from ap cloud: 
`cmd.run 'systemctl status edge-network'`

To check it out, Install [BLE Scanner](https://apps.apple.com/us/app/ble-scanner-4-0/id1221763603) on the Mac/iOS.
The when the AutoPi is on, with your vehicle ON, start up that app and find the autopi, connect to it - prefixed by `autopi-`.
You'll see the advertised services match what is below in Commands. Click on the one you want to get the value.

Instead of building locally, you can also build in Github and then set the release URL for a specific 
device in the AutoPi cloud (ie. your device). Then do above again with BLE scanner to connect and test your changes.
Note that you will only be able to pull data from the vehicle if it is on. 
Also, if the AutoPi is unable to connect to the network for whatever reason edge-network won't return data eg. the VIN. For example
if you connec the AP to wifi, but it can't get an internet connection, this stuff won't work. 

To view logs, from the AP cloud terminal, run the following: `journalctl -u edge-network`

## Loggers / DIMO Client

Change the template on the device to "no loggers" id 117.

### Development Cycle
- set no loggers on device from ap cloud
- have it connect to local wifi.
- enable allow ssh connections on local wifi: https://docs.autopi.io/guides/how-to-ssh-to-your-device/
- turn on with simulator. 
- send command so AP doesn't turn off (can also use AP cloud to do this): `power.hibernate delay 3600`
- find IP address of AP with your wifi router.
- ssh pi@192.168.181.129
- see above for ssh password, default pwd is autopi2018
- Voltage from simulator is 11.6, change critical voltage for hibernation to 11.5, same for Safety Cutout
- If you have pending changes/ updates, connect to car to get higher voltage so it stays on & applies them.

3d020f1f-7a5c-463f-a57a-785bae2ea760

## Can Dump Commands from terminal

        edge-network candump -cycles <cycle_count> -send <chunk_size> -save
        
    To generate local can dump to single file:
       ./edge-network candump -cycles <cycle_count> -save
       
          example: ./edge-network candump -cycles 100 -save

    To generate can dump and send over mqtt:
       ./edge-network candump -cycles <cycle_count> -send <chunk_size>
       
          example: ./edge-network candump -cycles 100 -send 50

    To generate local can dump and send over mqtt, AND save local copies of chunked messages to file:
       ./edge-network candump -cycles <cycle_count> -send <chunk_size> -save
       
          example: ./edge-network candump -cycles 100 -send 50 -save

## BLE Commands

For the management calls, the process needs to have the `CAP_NET_BIND_SERVICE` capability.

* Device service `5c307fa4-6859-4d6c-a87b-8d2c98c9f6f0`
  * Get Serial Number characteristic `5c305a11-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded Serial Number of the Unit
  * Get Secondary Serial Number characteristic `5c305a12-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded Secondary Serial Number of the Unit
  * Get Hardware Revision Number characteristic `5c305a13-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded HW Revision of the Unit
  * Get Software Version characteristic `5c305a18-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded Software Version Number of the Unit
  * Get Bluetooth Version Number characteristic `5c305a19-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded Bluetooth Version Number of the Unit
  * Get Signal Strength characteristic `5c305a14-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded Signal Strength of the Unit in dBm
  * Get Wifi connection status characteristic `5c305a15-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded SSID of current wifi connected to the unit
  * Set Wifi Connection characteristic `5c305a16-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Write._ Send in wifi SSID and Password in the json string form e.g. "{\"network\":\"Wifi-4A91D0\",\"password\":\"somePassword\"}" response on success will be the network name
  * Get IMSI characteristic `5c305a21-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded IMSI of the SIM card
* Vehicle service `5c30d387-6859-4d6c-a87b-8d2c98c9f6f0`
  * Get VIN characteristic `5c300acc-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded VIN
  * Get Protocol characteristic `5c300adc-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded Protocol supported, either "06" or "07"
  * Get Diagnostics Error Codes characteristic `5c300add-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded, comma delimited list of DTC error codes, if any
* Transactions service `5c30aade-6859-4d6c-a87b-8d2c98c9f6f0`
  * Get Ethereum address characteristic `5c301dd2-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the 20 bytes of the Ethereum address for the device.
  * Sign hash characteristic `5c30e60f-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Write._ Send in the 32 bytes of a hash to be signed
    * _Read._ Return the 65 bytes of the signature for the last submitted hash. If something went wrong with the signing this will error.

We should do notifications but I assumed it would be too much of a change. Note that the UUIDs here only differ in bytes 3 and 4.

Missing:

* Get cell signal strength

## Deploying for production (single vehicle)

1. Release a build, wait for Github action to complete
2. Select the desired vehicle in top right selector.
3. Go to advanced settings in AutoPi admin - https://dimo.autopi.io/#/advanced-settings
4. Look for Dimo setting and update the URL, basically just update the version. Note that the URL is a cloudflare bucket and not pointing directly to
github because we were getting rate limited. The proxy code [is here](https://github.com/DIMO-Network/assets-proxy/blob/main/src/index.js) that automatically pulls binaries.dimo.zone from GH. 

### Deploying to all vehicles or specific templates

1. Release a build, wait for Github action to complete
2. Templates https://dimo.autopi.io/#/template
3. Select the Base template, or other specific templates
4. Configuration tab
5. Edit the `Dimo > Edge-network > Url` property updating the version portion of it.

## Re-installing edge-network on an autopi

Remove the current install:
`cmd.run 'rm /opt/autopi/edge-network_release.tar.gz'` 

Install the newest version of the edge-network, uses the URL configured in autopi cloud to download tar.gz
`state.sls dimo.install`

Check to see if it is running:
`cmd.run 'systemctl status edge-network'`

If you see this result:
  edge-network.service - DIMO Bluetooth Service
  Loaded: loaded (/lib/systemd/system/edge-network.service; disabled; vendor preset: enabled)
  Active: inactive (dead)

Start the service manually if didn't start after install:
`cmd.run 'systemctl start edge-network'` 

# Gotchas / Notes

The `-v` command is important for the salt stack on the autopi to work correctly for managing the correct version to download.
If not present, this will likely cause devices being unable to update and/or apply pending syncs, 
since the devices are using the edge-network -v call to confirm if the binary installation is working properly. 
It expects to see the version number of the binary.

Note that the `Version` is set via a build flag in the `/.github/workflows/release.yaml` via the `ldflags`. 

# Research

Other CAN libraries: https://github.com/go-daq/canbus 
supports sending data on the bus
eg PID fuel tank level: `can-send vcan0 7DF#02012F5555555555`
