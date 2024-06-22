package units

import (
	"encoding/json"
	"fmt"
)

// TemperatureUnit is an enum for all temperature unit types
type TemperatureUnit int

// The different TemperatureUnits
const (
	// Kelvin is Kelvin
	Kelvin TemperatureUnit = iota
	// Farenheit is Farenheit
	Farenheit TemperatureUnit = iota
	// Celsius is Celsius
	Celsius TemperatureUnit = iota
)

// Temperature is a generic Unit structure that represents temperatures
type Temperature Unit[TemperatureUnit]

// MarshalJSON is a custom marshaler for the unit type to add the UnitType string
func (u Temperature) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Value    float32 `json:"value"`
		Unit     int     `json:"unit"`
		UnitType string  `json:"unitType"`
	}{
		Value:    u.Value,
		Unit:     int(u.Unit),
		UnitType: "temp",
	})
}

// NewTemperature creates a temperature unit of a given type and value
func NewTemperature(u TemperatureUnit, value float32) Temperature {
	return Temperature{
		Unit:  u,
		Value: value,
	}
}

// Convert converts the unit+value into a new unit type, returning a new unit value of the requested type.
func (p Temperature) Convert(newUnit TemperatureUnit) Temperature {
	// Shortcut (ez optimization)
	if p.Unit == newUnit {
		return p
	}

	var inKelvin float32
	switch p.Unit {
	case Kelvin:
		inKelvin = p.Value
	case Farenheit:
		inKelvin = (p.Value + 459.67) * (5.0 / 9.0)
	case Celsius:
		inKelvin = p.Value + 273.15
	default:
		panic(fmt.Sprintf("Unknown old temperature unit %+v", p.Unit))
	}
	switch newUnit {
	case Kelvin:
		return NewTemperature(newUnit, inKelvin)
	case Farenheit:
		return NewTemperature(newUnit, inKelvin*(9.0/5.0)-459.67)
	case Celsius:
		return NewTemperature(newUnit, inKelvin-273.15)
	default:
		panic(fmt.Sprintf("Unknown new temperature unit %+v", newUnit))
	}
}
