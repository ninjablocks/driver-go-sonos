#Ninja Sphere Go SONOS Driver

This driver is currently using [go-sonos](https://github.com/ianr0bkny/go-sonos).

##Building
Run `make` in the directory of the driver

##Running
Run `./bin/driver-go-sonos` from the `bin` directory after building

# Issues

* Some race conditions in the underlying library around how ssdp events are processed.
* Logs way to much crap
