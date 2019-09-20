package logger

import (
	"../sysexits"
	"log"
	"os"
)

var Log *log.Logger

func init() {
	file, err := os.OpenFile("/var/log/mailtos3.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Println(err)
		// let mta know that there is a critical file missing
		os.Exit(sysexits.EX_OSFILE)
	}

	Log = log.New(file, "", log.LstdFlags|log.Lshortfile)
}
