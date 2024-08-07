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
	// Kelvin is a TemperatureUnit of type Kelvin.
	Kelvin TemperatureUnit = iota
	// Fahrenheit is a TemperatureUnit of type Fahrenheit.
	Fahrenheit
	// Celsius is a TemperatureUnit of type Celsius.
	Celsius
)

var ErrInvalidTemperatureUnit = errors.New("not a valid TemperatureUnit")

const _TemperatureUnitName = "KelvinFahrenheitCelsius"

// TemperatureUnitValues returns a list of the values for TemperatureUnit
func TemperatureUnitValues() []TemperatureUnit {
	return []TemperatureUnit{
		Kelvin,
		Fahrenheit,
		Celsius,
	}
}

var _TemperatureUnitMap = map[TemperatureUnit]string{
	Kelvin:     _TemperatureUnitName[0:6],
	Fahrenheit: _TemperatureUnitName[6:16],
	Celsius:    _TemperatureUnitName[16:23],
}

// String implements the Stringer interface.
func (x TemperatureUnit) String() string {
	if str, ok := _TemperatureUnitMap[x]; ok {
		return str
	}
	return fmt.Sprintf("TemperatureUnit(%d)", x)
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x TemperatureUnit) IsValid() bool {
	_, ok := _TemperatureUnitMap[x]
	return ok
}

var _TemperatureUnitValue = map[string]TemperatureUnit{
	_TemperatureUnitName[0:6]:   Kelvin,
	_TemperatureUnitName[6:16]:  Fahrenheit,
	_TemperatureUnitName[16:23]: Celsius,
}

// ParseTemperatureUnit attempts to convert a string to a TemperatureUnit.
func ParseTemperatureUnit(name string) (TemperatureUnit, error) {
	if x, ok := _TemperatureUnitValue[name]; ok {
		return x, nil
	}
	return TemperatureUnit(0), fmt.Errorf("%s is %w", name, ErrInvalidTemperatureUnit)
}
