package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/ninjasphere/go-ninja/logger"
)

var nlog = logger.GetLogger(info.Name)

func main() {

	log.SetFlags(log.Ltime | log.Lshortfile)

	StartSonosDriver()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	s := <-c
	fmt.Println("Got signal:", s)

}
