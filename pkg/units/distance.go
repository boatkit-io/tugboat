package units

import "encoding/json"

// DistanceUnit is an enum for all distance unit types
type DistanceUnit int

// DistanceUnits
const (
	// Meter is a meter
	Meter DistanceUnit = 0
	// Foot is a foot
	Foot DistanceUnit = 1
	// Mile is a mile
	Mile DistanceUnit = 2
	// NauticalMile is a nautical mile
	NauticalMile DistanceUnit = 3
	// Fathom is a fathom
	Fathom DistanceUnit = 4
)

// distanceConversions is a helper for doing unit conversions on distance units
var distanceConversions = map[DistanceUnit]float32{
	Meter:        1609.34,
	Foot:         5280,
	Mile:         1,
	NauticalMile: 1.15078,
	Fathom:       880,
}

// Distance is a generic Unit structure that represents distances/lengths
type Distance Unit[DistanceUnit]

// MarshalJSON is a custom marshaler for the unit type to add the UnitType string
func (u Distance) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Value    float32 `json:"value"`
		Unit     int     `json:"unit"`
		UnitType string  `json:"unitType"`
	}{
		Value:    u.Value,
		Unit:     int(u.Unit),
		UnitType: "dist",
	})
}

// NewDistance creates a distance unit of a given type and value
func NewDistance(u DistanceUnit, value float32) Distance {
	return Distance{
		Unit:  u,
		Value: value,
	}
}

// Convert converts the unit+value into a new unit type, returning a new unit value of the requested type.
func (p Distance) Convert(newUnit DistanceUnit) Distance {
	v2 := convertTableUnit(distanceConversions, p.Value, p.Unit, newUnit)
	return NewDistance(newUnit, v2)
}

// Add will add another unit to this one, returning a new unit with the added values
func (p Distance) Add(o Distance) Distance {
	v2, u2 := addTableUnits(distanceConversions, p.Value, p.Unit, o.Value, o.Unit)
	return NewDistance(u2, v2)
}

// Sub will subtract another unit from this one, returning a new unit with the subtracted values
func (p Distance) Sub(o Distance) Distance {
	v2, u2 := subTableUnits(distanceConversions, p.Value, p.Unit, o.Value, o.Unit)
	return NewDistance(u2, v2)
}
