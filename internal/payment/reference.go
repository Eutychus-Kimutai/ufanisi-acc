package payment

import (
	"errors"
	"regexp"
)

var (
	ErrInvalidAccountReference = errors.New("invalid account reference format")
	ErrUnknownProductType      = errors.New("unknown product type")
)

const (
	LoanPrefix       = "LN"
	ProductEducation = "Elimu"
	ProductBusiness  = "Biashara"
	ProductPersonal  = "Mali"
)

var productTypeMap = map[string]string{
	ProductEducation: "Education",
	ProductBusiness:  "Business",
	ProductPersonal:  "Personal",
}

type AccountReference struct {
	LoanNumber  string
	ProductType string
}

var pattern = regexp.MustCompile(`^LN(\d+)(Elimu|Biashara|Mali)$`)

func ParseAccountReference(ref string) (AccountReference, error) {

	matches := pattern.FindStringSubmatch(ref)
	if matches == nil {
		return AccountReference{}, ErrInvalidAccountReference
	}
	productType, ok := productTypeMap[matches[2]]
	if !ok {
		return AccountReference{}, ErrUnknownProductType
	}

	return AccountReference{
		LoanNumber:  LoanPrefix + matches[1],
		ProductType: productType,
	}, nil
}

func EncodeAccountReference(loanNumber, productType string) string {
	for suffix, pt := range productTypeMap {
		if pt == productType {
			return loanNumber + suffix
		}
	}
	return ""
}
