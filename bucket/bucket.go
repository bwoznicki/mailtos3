package bucket

import (
	"context"
	"errors"
	"fmt"
	"mailtos3/config"
	"mailtos3/logger"
	"mailtos3/sysexits"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConf "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	// s3 encryption
	// Import the materials and client package
	"github.com/aws/amazon-s3-encryption-client-go/v3/client"
	"github.com/aws/amazon-s3-encryption-client-go/v3/materials"
)

func PutObject(config *config.RequestConfig, address *string, msgBody *string, bucket *string, object string, cmkArn *string, prefix string) bool {

	// initiate aws config for s3 client with correct region
	cfg, err := awsConf.LoadDefaultConfig(context.TODO(),
		awsConf.WithRegion(config.Region),
	)
	if err != nil {
		logger.Log.Printf("[ERROR] unable to create client config, %v", err)
		// let mta know that there was internal error
		os.Exit(sysexits.EX_SOFTWARE)
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

	// get body size
	size := getBodySize(msgBody)

	// time the transfer
	timer := time.Now()

	objectKey := filepath.Join(prefix, object)
	input := &s3.PutObjectInput{
		Body:   strings.NewReader(*msgBody),
		Bucket: aws.String(*bucket),
		Key:    aws.String(objectKey),
	}

	logger.Log.Printf("[INFO] sending %s to %s", objectKey, *bucket)

	s3Client := newS3Client(cfg, cmkArn)
	switch c := s3Client.(type) {
	case *client.S3EncryptionClientV3:
		_, err = c.PutObject(ctx, input)
	case *s3.Client:
		_, err = c.PutObject(ctx, input)
	default:
		logger.Log.Printf("[ERROR] unhandled type when switching between s3 clients")
		// let mta know that there was internal error
		os.Exit(sysexits.EX_SOFTWARE)
	}
	if err != nil {
		var oe *smithy.OperationError
		if errors.As(err, &oe) && errors.Is(err, os.ErrDeadlineExceeded) {
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
	logger.Log.Printf("[INFO] transfer complete for %s, %s sent in %v", objectKey, size, time.Since(timer))
	return true

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

// newS3Client will create s3Client or s3EncryptionClient depending on
// whether we have a valid kms key
func newS3Client(cfg aws.Config, cmkArn *string) interface{} {

	s3Client := s3.NewFromConfig(cfg)

	if cmkArn != nil && len(*cmkArn) > 0 {

		kmsClient := kms.NewFromConfig(cfg)
		cmm, err := materials.NewCryptographicMaterialsManager(materials.NewKmsKeyring(kmsClient, *cmkArn, func(options *materials.KeyringOptions) {
			options.EnableLegacyWrappingAlgorithms = false
		}))
		if err != nil {
			logger.Log.Printf("[ERROR] creating new CMM, %v", err)
			// let mta know that there was internal error
			os.Exit(sysexits.EX_SOFTWARE)
		}

		s3EncryptionClient, err := client.New(s3Client, cmm)
		if err != nil {
			logger.Log.Printf("[ERROR] failed to crate encryption client, %v", err)
			// let mta know that there was internal error
			os.Exit(sysexits.EX_SOFTWARE)
		}
		return s3EncryptionClient
	}

	return s3Client
}
