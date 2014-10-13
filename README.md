#Ninja Sphere Go SONOS Driver

This driver is currently using [go-sonos](https://github.com/ianr0bkny/go-sonos).

##Building
Run `make` in the directory of the driver

##Running
Run `./bin/driver-go-sonos` from the `bin` directory after building

# Issues

* Logs way to much crap
* Mite need to drop events given the size of these playloads.. Moving the volume slider back and forward in the app triggers a LOT of updates.
