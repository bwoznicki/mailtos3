package bucket

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"mailtos3/config"
	"mailtos3/logger"
	"mailtos3/sysexits"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3crypto"
)

func PutObject(config *config.RequestConfig, address *string, msgBody *string, bucket *string, cmkArn *string) bool {

	var endpoint string
	if config.Endpoint {
		endpoint = "s3." + config.Region + ".amazonaws.com"
	}

	sess := session.New(&aws.Config{
		Region:   aws.String(config.Region),
		Endpoint: aws.String(endpoint),
	})

	genName := generateNameHash()
	input := &s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(strings.NewReader(*msgBody)),
		Bucket: aws.String(*bucket),
		Key:    aws.String(genName),
	}

	// Create a context with a timeout that will abort the upload if it takes
	// more than the passed in timeout.
	ctx := context.Background()
	var cancelFn func()
	if config.Timeout > 0 {
		ctx, cancelFn = context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Second)
	}
	// Ensure the context is canceled to prevent leaking.
	// See context package for more information, https://golang.org/pkg/context/
	defer cancelFn()

	// determine if we need S3 client or EncryptionClient
	client := getClient(sess, cmkArn)

	// get body size
	size := getBodySize(msgBody)

	logger.Log.Printf("[INFO] sending %s to %s", genName, *bucket)

	// time the transfer
	timer := time.Now()

	var err error
	switch c := client.(type) {
	case *s3crypto.EncryptionClient:
		_, err = c.PutObjectWithContext(ctx, input)
	case *s3.S3:
		_, err = c.PutObjectWithContext(ctx, input)
	default:
		logger.Log.Printf("[ERROR] unhandled type when switching between s3 clients")
		// let mta know that there was internal error
		os.Exit(sysexits.EX_SOFTWARE)
	}

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == request.CanceledErrorCode {
			// If the SDK can determine the request or retry delay was canceled
			// by a context the CanceledErrorCode error code will be returned.
			logger.Log.Printf("[WARNING] upload canceled due to timeout (%ds), %s", config.Timeout, err)
			// let mta know that is ok to retry later
			os.Exit(sysexits.EX_TEMPFAIL)
		}
		logger.Log.Printf("[ERROR] message delivery failed for: %s. %s", *address, err)
		// need to send fail code to mta
		// if the api fails bounce messages, do not risk overflowing the msg queue
		os.Exit(sysexits.EX_UNAVAILABLE)

	}
	logger.Log.Printf("[INFO] transfer complete for %s, %s sent in %v", genName, size, time.Since(timer))
	return true
}

func getClient(s *session.Session, cmkArn *string) interface{} {

	var client interface{}
	if len(*cmkArn) > 0 {
		keywrap := s3crypto.NewKMSKeyGenerator(kms.New(s), *cmkArn)
		// This is our content cipher builder, used to instantiate new ciphers
		// that enable us to encrypt or decrypt the payload.
		builder := s3crypto.AESGCMContentCipherBuilder(keywrap)
		// create our crypto client
		client = s3crypto.NewEncryptionClient(s, builder)

	} else {
		// if no cmk use regular s3 client
		client = s3.New(s)
	}
	return client
}

func generateNameHash() string {
	// get the sha1 string from current unix time
	h := sha1.New()
	s := strconv.FormatInt(time.Now().UnixNano(), 10)
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func getBodySize(body *string) string {
	size := len(*body)

	const unit = 1000
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "kMGTPE"[exp])
}
