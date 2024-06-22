package units

// VolumeUnit is an enum for all volume unit types
type VolumeUnit int

// The VolumeUnits
const (
	// MetersCubed is m^3
	MetersCubed VolumeUnit = iota
	// Liter is Liter
	Liter VolumeUnit = iota
	// Gallon is Gallon
	Gallon VolumeUnit = iota
)

// volumeConversions is a helper for doing unit conversions on VolumeUnits
var volumeConversions = map[VolumeUnit]float32{
	MetersCubed: 1,
	Liter:       1000,
	Gallon:      264.172,
}

// Volume is a generic Unit structure that represents volumes/capacities
type Volume Unit[VolumeUnit]

// NewVolume creates a volume unit of a given type and value
func NewVolume(u VolumeUnit, value float32) Volume {
	return Volume{
		Unit:     u,
		Value:    value,
		UnitType: "vol",
	}
}

// Convert converts the unit+value into a new unit type, returning a new unit value of the requested type.
func (p Volume) Convert(newUnit VolumeUnit) Volume {
	v2 := convertTableUnit(volumeConversions, p.Value, p.Unit, newUnit)
	return NewVolume(newUnit, v2)
}

// Add will add another unit to this one, returning a new unit with the added values
func (p Volume) Add(o Volume) Volume {
	v2, u2 := addTableUnits(volumeConversions, p.Value, p.Unit, o.Value, o.Unit)
	return NewVolume(u2, v2)
}

// Sub will subtract another unit from this one, returning a new unit with the subtracted values
func (p Volume) Sub(o Volume) Volume {
	v2, u2 := subTableUnits(volumeConversions, p.Value, p.Unit, o.Value, o.Unit)
	return NewVolume(u2, v2)
}
