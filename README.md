Compile for the AutoPi with
```
GOARCH=arm GOOS=linux go build -ldflags="-s -w" .
```
This will produce a binary named `edge-network`. For your convenience, a binary has been placed in the `bin` directory. We should switch to GitHub releases.

Copy to the AutoPi with, e.g.,
```
scp bin/edge-network pi@192.168.4.1:~
```
This should place the executable in the home directory. Then you can run it. We need to make it into a systemd service. The device should be discoverable under thr usual `autopi`-prefixed name.

* Service `463e3f16-f894-44aa-92a2-0d7338075d74`
  * Get VIN `463ede95-f894-44aa-92a2-0d7338075d74`
    * _Read._ Return the ASCII-encoded VIN
  * Sign hash `463e6fe3-f894-44aa-92a2-0d7338075d74`
    * _Write._ Send in the 32 bytes of a hash to be signed
    * _Read._ Return the 65 bytes of the signature for the last submitted hash. If something went wrong with the signing this will error.

We should do notifications but I assumed it would be too much of a change. Note that the UUIDs here only differ in bytes 3 and 4.

Missing:

* Get Ethereum address
* Get cell signal strength
