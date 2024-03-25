package validation

import (
	"reflect"
	"strings"

	"github.com/absmach/mproxy/pkg/tls/verifier/crl"
	"github.com/absmach/mproxy/pkg/tls/verifier/ocsp"
	"github.com/absmach/mproxy/pkg/tls/verifier/types"
	"github.com/caarlos0/env/v10"
)

type config struct {
	Validations []types.Validation `env:"CLIENT_CERT_VALIDATION_METHODS"             envDefault:""`
}

func NewValidationMethods(opts env.Options) ([]types.ValidationMethod, error) {
	var c config
	if opts.FuncMap == nil {
		opts.FuncMap = make(map[reflect.Type]env.ParserFunc)
	}
	opts.FuncMap[reflect.TypeOf(make([]types.Validation, 0))] = envParseSliceValidate
	opts.FuncMap[reflect.TypeOf(new(types.Validation))] = envParseValidation
	if err := env.ParseWithOptions(&c, opts); err != nil {
		return nil, err
	}
	if len(c.Validations) == 0 {
		return nil, nil
	}
	return c.newValidationMethods(opts)
}

func (c *config) newValidationMethods(opts env.Options) ([]types.ValidationMethod, error) {
	var vms []types.ValidationMethod
	for _, v := range c.Validations {
		switch v {
		case types.OCSP:
			vm, err := ocsp.NewValidationMethod(opts)
			if err != nil {
				return nil, err
			}
			vms = append(vms, vm)
		case types.CRL:
			vm, err := crl.NewValidationMethod(opts)
			if err != nil {
				return nil, err
			}
			vms = append(vms, vm)
		default:
			return nil, types.ErrInvalidClientValidation
		}
	}
	return vms, nil
}

func envParseSliceValidate(v string) (interface{}, error) {
	var vms []types.Validation
	v = strings.TrimSpace(v)
	vmss := strings.Split(v, ",")
	for _, vm := range vmss {
		v, err := types.ParseValidation(vm)
		if err != nil {
			return nil, err
		}
		vms = append(vms, v)
	}
	return vms, nil
}

func envParseValidation(v string) (interface{}, error) {
	return types.ParseValidation(v)
}
