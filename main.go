package main

import (
	"./bucket"
	"./config"
	"./logger"
	"./router"
	"./sysexits"
	"errors"
	"flag"
	"io/ioutil"
	"os"
)

var Conf config.Config
var address string

func init() {
	// get the configuration file, set region, flags and mail routes
	Conf = config.Load()

	usage := "email address for the receiving mailbox"
	flag.StringVar(&address, "address", "CatchAll", usage)
	flag.StringVar(&address, "a", "CatchAll", usage+" (shorthand)")
}

func main() {

	// retrieve the flags
	flag.Parse()

	logger.Log.Printf("[INFO] processing message for: %s", address)

	// find matching mailbox
	// if matching mailbox found read body and pass to put object
	if m, ok := router.MatchMailbox(Conf.Mailboxes, address); ok {

		// retrieve message body passed as argument to mailtos3
		msgBody, err := getBody()
		if err != nil {
			logger.Log.Printf("[ERROR] %s", err)
			// let mta know that there was I/O error
			os.Exit(sysexits.EX_NOINPUT)
		}

		bucket.PutObject(&Conf.RequestConfig, &address, &msgBody, &m.Bucket, &m.CmkKeyArn)

	} else {
		logger.Log.Printf("[WARNING] mailbox not found for: %s", address)
		os.Exit(sysexits.EX_NOUSER)
	}
}

func getBody() (string, error) {

	// read from stdin in first if there is no data check args
	// check if there is anything on stdin
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}

	// if we have something on stdin read until EOF
	if info.Mode()&os.ModeNamedPipe != 0 {

		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return "", errors.New("mailtos3 error reading from stdin.")
		}
		return string(bytes), nil

	} else {

		// nothing passed from pipe check args instead
		args := flag.Args()
		if len(args) != 1 {
			return "", errors.New("mailtos3 expects message body to be passed as the last argument or from stdin.")
		}
		return args[0], nil
	}
}
