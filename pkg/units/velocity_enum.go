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
	// MetersPerSecond is a VelocityUnit of type MetersPerSecond.
	MetersPerSecond VelocityUnit = iota
	// Knots is a VelocityUnit of type Knots.
	Knots
	// Mph is a VelocityUnit of type Mph.
	Mph
	// Kph is a VelocityUnit of type Kph.
	Kph
)

var ErrInvalidVelocityUnit = errors.New("not a valid VelocityUnit")

const _VelocityUnitName = "MetersPerSecondKnotsMphKph"

// VelocityUnitValues returns a list of the values for VelocityUnit
func VelocityUnitValues() []VelocityUnit {
	return []VelocityUnit{
		MetersPerSecond,
		Knots,
		Mph,
		Kph,
	}
}

var _VelocityUnitMap = map[VelocityUnit]string{
	MetersPerSecond: _VelocityUnitName[0:15],
	Knots:           _VelocityUnitName[15:20],
	Mph:             _VelocityUnitName[20:23],
	Kph:             _VelocityUnitName[23:26],
}

// String implements the Stringer interface.
func (x VelocityUnit) String() string {
	if str, ok := _VelocityUnitMap[x]; ok {
		return str
	}
	return fmt.Sprintf("VelocityUnit(%d)", x)
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x VelocityUnit) IsValid() bool {
	_, ok := _VelocityUnitMap[x]
	return ok
}

var _VelocityUnitValue = map[string]VelocityUnit{
	_VelocityUnitName[0:15]:  MetersPerSecond,
	_VelocityUnitName[15:20]: Knots,
	_VelocityUnitName[20:23]: Mph,
	_VelocityUnitName[23:26]: Kph,
}

// ParseVelocityUnit attempts to convert a string to a VelocityUnit.
func ParseVelocityUnit(name string) (VelocityUnit, error) {
	if x, ok := _VelocityUnitValue[name]; ok {
		return x, nil
	}
	return VelocityUnit(0), fmt.Errorf("%s is %w", name, ErrInvalidVelocityUnit)
}
