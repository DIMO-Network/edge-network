Compile for the AutoPi with
```
GOARCH=arm64 GOOS=linux go build .
```
This will produce a binary named `edge-network`. Copy to the AutoPi with, e.g.,
```
scp edge-network pi@192.168.4.1:~
```
This should place the executable in the home directory. Then you can run it.

The name is a bit of a joke.
