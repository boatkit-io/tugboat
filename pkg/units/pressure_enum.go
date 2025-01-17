// Code generated by go-enum DO NOT EDIT.
// Version:
// Revision:
// Build Date:
// Built By:

package units

import (
	"errors"
	"fmt"
)

const (
	// Pa is a PressureUnit of type Pa.
	Pa PressureUnit = iota
	// Psi is a PressureUnit of type Psi.
	Psi
	// Hpa is a PressureUnit of type Hpa.
	Hpa
)

var ErrInvalidPressureUnit = errors.New("not a valid PressureUnit")

const _PressureUnitName = "PaPsiHpa"

// PressureUnitValues returns a list of the values for PressureUnit
func PressureUnitValues() []PressureUnit {
	return []PressureUnit{
		Pa,
		Psi,
		Hpa,
	}
}

var _PressureUnitMap = map[PressureUnit]string{
	Pa:  _PressureUnitName[0:2],
	Psi: _PressureUnitName[2:5],
	Hpa: _PressureUnitName[5:8],
}

// String implements the Stringer interface.
func (x PressureUnit) String() string {
	if str, ok := _PressureUnitMap[x]; ok {
		return str
	}
	return fmt.Sprintf("PressureUnit(%d)", x)
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x PressureUnit) IsValid() bool {
	_, ok := _PressureUnitMap[x]
	return ok
}

var _PressureUnitValue = map[string]PressureUnit{
	_PressureUnitName[0:2]: Pa,
	_PressureUnitName[2:5]: Psi,
	_PressureUnitName[5:8]: Hpa,
}

// ParsePressureUnit attempts to convert a string to a PressureUnit.
func ParsePressureUnit(name string) (PressureUnit, error) {
	if x, ok := _PressureUnitValue[name]; ok {
		return x, nil
	}
	return PressureUnit(0), fmt.Errorf("%s is %w", name, ErrInvalidPressureUnit)
}
