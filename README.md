Compile for the AutoPi with
```
brew install --build-from-source upx
GOARCH=arm GOOS=linux go build -ldflags="-s -w" -o edge-network && upx edge-network

```
Binaries will build [for releases](https://github.com/DIMO-Network/edge-network/releases) from the [workflow](.github/workflows/release.yaml).

Copy to the AutoPi with, e.g.,
```
scp bin/edge-network pi@192.168.4.1:~
```
This should place the executable in the home directory. Then you can run it. We need to make it into a systemd service. The device should be discoverable under thr usual `autopi`-prefixed name.

For the management calls, the process needs to have the `CAP_NET_BIND_SERVICE` capability.

* Device service `5c307fa4-6859-4d6c-a87b-8d2c98c9f6f0`
  * Get Serial Number characteristic `5c305a11-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded Serial Number of the Unit
  * Get Secondary Serial Number characteristic `5c305a12-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded Secondary Serial Number of the Unit
   * Get Hardware Revision Number characteristic `5c305a13-6859-4d6c-a87b-8d2c98c9f6f0`
    * _Read._ Return the ASCII-encoded HW Revision of the Unit
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
