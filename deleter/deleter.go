package deleter

import (
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/coreos/grafiti/arn"
	"github.com/spf13/viper"
)

const drStr = "(dry-run)"

func setUpAWSSession() *session.Session {
	return session.Must(session.NewSession(
		&aws.Config{
			Region: aws.String(viper.GetString("grafiti.region")),
		},
	))
}

// DeleteConfig holds configuration info for resource deletion
type DeleteConfig struct {
	DryRun       bool
	IgnoreErrors bool
	BackoffTime  time.Duration
	Logger       logrus.FieldLogger
}

// InitRequestLogger creates a logrus.FieldLogger that logs to a file at path,
// or os.Stderr if an error occurs opening the file
func InitRequestLogger(path string) logrus.FieldLogger {
	logger := &logrus.Logger{
		Out:       os.Stderr,
		Formatter: &logrus.JSONFormatter{},
		Level:     logrus.InfoLevel,
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0664)
	if err == nil {
		logger.Out = f
	} else {
		logger.Infof("Failed to open file %s for logging, using stderr instead", path)
	}

	return logger
}

// LogEntry maps potential log entry fields to a Go struct. Add fields here when
// creating fields in ResourceDeleter.DeleteResources implementations
type LogEntry struct {
	ResourceType       arn.ResourceType `json:"resource_type"`
	ResourceName       arn.ResourceName `json:"resource_name"`
	AWSErrorCode       string           `json:"aws_err_code,omitempty"`
	AWSErrorMsg        string           `json:"aws_err_msg,omitempty"`
	ErrMsg             string           `json:"err_msg,omitempty"`
	ParentResourceType arn.ResourceType `json:"parent_resource_type,omitempty"`
	ParentResourceName arn.ResourceName `json:"parent_resource_name,omitempty"`
}

// Log deletion errors to a DeleteConfig.Logger
func (c *DeleteConfig) logDeleteError(rt arn.ResourceType, rn arn.ResourceName, err error, fs ...logrus.Fields) {
	fields := logrus.Fields{
		"resource_type": rt,
		"resource_name": rn.String(),
	}

	aerr, ok := err.(awserr.Error)
	if ok {
		fields["aws_err_code"] = aerr.Code()
		fields["aws_err_msg"] = aerr.Message()
	} else {
		fields["err_msg"] = err.Error()
	}

	// Allow overwriting old fields or adding extra fields if desired
	if len(fs) > 0 {
		for fk, fv := range fs[0] {
			fields[fk] = fv
		}
	}

	c.Logger.WithFields(fields).Info("Failed to delete resource")
}

// A ResourceDeleter is any type that can delete itself from AWS and describe
// itself using an AWS request
type ResourceDeleter interface {
	// Adds resource names to the ResourceDeleter
	AddResourceNames(...arn.ResourceName)
	// Delete resources using DeleteConfig info
	DeleteResources(*DeleteConfig) error
}

// InitResourceDeleter creates a ResourceDeleter using a type defined in
// the `deleter` package
func InitResourceDeleter(t arn.ResourceType) ResourceDeleter {
	switch t {
	case arn.AutoScalingGroupRType:
		return &AutoScalingGroupDeleter{ResourceType: t}
	case arn.AutoScalingLaunchConfigurationRType:
		return &AutoScalingLaunchConfigurationDeleter{ResourceType: t}
	}
	return nil
}