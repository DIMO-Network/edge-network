# Edge-Network app

This application is meant to run on an autopi, to get a variety of information from the device over BTE.

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
Replace the IP address with the IP of the AutoPi on your local network. You'll need to have it [connect to your local wifi.](https://docs.autopi.io/guides/guides-intro/#6-connect-to-wifi) 
This should place the executable in the home directory. Then you can run it. We need to make it into a systemd service. The device should be discoverable under thr usual `autopi`-prefixed name.

### How this works / testing it

Install [BLE Scanner](https://apps.apple.com/us/app/ble-scanner-4-0/id1221763603) on the Mac/iOS.
The when the AutoPi is on, start up that app and find the autopi, connect to it. 
You'll see the advertised services match what is below. Click on the one you want to ge the value.

Instead of building locally, you can also build in Github and then set the release URL for a specific 
device in the AutoPi cloud (ie. your device). Then do above again with BLE scanner to connect and test your changes.

To view logs, from the AP cloud terminal, run the following: `journalctl -u edge-network`

## Commands

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
* Vehicle service `5c30d387-6859-4d6c-a87b-8d2c98c9f6f0`
  * Get VIN characteristic `5c300acc-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded VIN
* Transactions service `5c30aade-6859-4d6c-a87b-8d2c98c9f6f0`
  * Get Ethereum address characteristic `5c301dd2-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the 20 bytes of the Ethereum address for the device.
  * Sign hash characteristic `5c30e60f-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Write._ Send in the 32 bytes of a hash to be signed
    * _Read._ Return the 65 bytes of the signature for the last submitted hash. If something went wrong with the signing this will error.

We should do notifications but I assumed it would be too much of a change. Note that the UUIDs here only differ in bytes 3 and 4.

Missing:

* Get cell signal strength

## Deploying for production

1. Release a build, wait for Github action to complete
2. Go to advanced settings in AutoPi admin - https://dimo.autopi.io/#/advanced-settings
3. Look for Dimo setting and update the URL. Note that the URL is a cloudflare bucket and not pointing directly to
github because we were getting rate limited. So would need to download the tar.gz release from GH and upload to 
Cloudflare. This process could be automated with CF API keys to copy automatically if find we do this often enough.
