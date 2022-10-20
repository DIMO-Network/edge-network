Compile for the AutoPi with
```
GOARCH=arm GOOS=linux go build .
```
This will produce a binary named `edge-network`. For your convenience, a binary has been placed in the `bin` directory. We should switch to GitHub releases.

Copy to the AutoPi with, e.g.,
```
scp bin/edge-network pi@192.168.4.1:~
```
This should place the executable in the home directory. Then you can run it. The device should be discoverable under thr usual `autopi`-prefixed name.

* Service `463e3f16-f894-44aa-92a2-0d7338075d74`
  * Get VIN `463ede95-f894-44aa-92a2-0d7338075d74`
    * Read
      Return the ASCII-encoded VIN
  * Sign hash `463e6fe3-f894-44aa-92a2-0d7338075d74`
    * Write
      Send in the 32 bytes of a hash to be signed
    * Read
      Return the 65 bytes of the signature for the last submitted hash. If something went wrong with the signing this will error.
