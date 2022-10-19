Compile for the AutoPi with
```
GOARCH=arm64 GOOS=linux go build .
```
This will produce a binary named `edge-network`. For your convenience, a binary has been placed in the `bin` directory.

Copy to the AutoPi with, e.g.,
```
scp bin/edge-network pi@192.168.4.1:~
```
This should place the executable in the home directory. Then you can run it.
