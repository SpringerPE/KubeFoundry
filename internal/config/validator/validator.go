package validator

import (
	"fmt"

	validator "github.com/asaskevich/govalidator"

	"kubefoundry/internal/config"
)

// Validator is an interface for setting default values
type Validator interface {
	Validate(c *config.Config) error
}

func init() {
	// validation to fail when struct fields do not include validations or are
	// not explicitly marked as exempt (using valid:"-" or
	// valid:"email,optional")
	validator.SetFieldsRequiredByDefault(false)
	for k, v := range config.Validators {
		validator.TagMap[k] = validator.Validator(v)
	}
}

// Validate Config the application's configuration
func Validate(c *config.Config) error {
	if _, err := validator.ValidateStruct(c); err != nil {
		return fmt.Errorf("Configuration is not correct: %s", err)
	}
	return nil
}
