package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mailtos3/bucket"
	"mailtos3/config"
	"mailtos3/logger"
	"mailtos3/router"
	"mailtos3/sysexits"
)

// Conf hold the local configuration for mailtos3
var Conf config.Config
var address string
var from string

func init() {
	// get the configuration file, set region, flags and mail routes
	Conf = config.Load()

	usageA := "email address for the receiving mailbox"
	flag.StringVar(&address, "address", "CatchAll", usageA)
	flag.StringVar(&address, "a", "CatchAll", usageA+" (shorthand)")

	usageF := "sender address, pass postfix ${sasl_sender} or ${sender}"
	flag.StringVar(&from, "from", "", usageF)
	flag.StringVar(&from, "f", "", usageF+" (shorthand)")
}

func main() {

	// retrieve the flags
	flag.Parse()

	objectKey := generateNameHash()

	logger.Log.Printf("[INFO] processing message from=%s, to=%s, object=%s", from, address, objectKey)

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

		prefix := formatPrefix(m.Prefix)

		bucket.PutObject(&Conf.RequestConfig, &address, &msgBody, &m.Bucket, objectKey, &m.CmkKeyArn, prefix)

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

		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", errors.New("mailtos3 error reading from stdin")
		}
		return string(bytes), nil

	}

	// nothing passed from pipe check args instead
	args := flag.Args()
	if len(args) != 1 {
		return "", errors.New("mailtos3 expects message body to be passed as the last argument or from stdin")
	}
	return args[0], nil

}

func formatPrefix(prefix string) string {
	re, err := regexp.Compile(`^dateTimeFormat\(.+\)$`)
	if err != nil {
		logger.Log.Printf("[WARNING] Unable to compile regex, prefix will be dropped, %s", fmt.Sprint(err))
		return ""
	}
	if re.MatchString(prefix) {
		dateLayout := strings.TrimLeft(strings.TrimRight(prefix, ")"), "dateTimeFormat(")
		return time.Now().Format(dateLayout)
	}
	return prefix
}

func generateNameHash() string {
	// get the sha1 string from current unix time
	h := sha1.New()
	s := strconv.FormatInt(time.Now().UnixNano(), 10)
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
