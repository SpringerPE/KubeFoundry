package config

type ValidatorFn func(string) bool

type Validate map[string]ValidatorFn

// No custom validators here
var Validators = Validate{}
