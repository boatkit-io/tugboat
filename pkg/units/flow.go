package units

// FlowUnit is an enum for all flow unit types
type FlowUnit int

// The FlowUnits
const (
	// LitersPerHour is L/hr
	LitersPerHour FlowUnit = iota
	// GallonsPerMinute is Gal/min
	GallonsPerMinute
	// GallonsPerHour is Gal/hr
	GallonsPerHour
)

// flowConversions is a helper for doing unit conversions on FlowUnits
var flowConversions = map[FlowUnit]float32{
	LitersPerHour:    1,
	GallonsPerMinute: 0.00440287,
	GallonsPerHour:   0.264172,
}

// Flow is a generic Unit structure that represents flow volumes over time
type Flow struct {
	Value    float32
	Unit     FlowUnit
	UnitType string
}

// NewFlow creates a flow unit of a given type and value
func NewFlow(u FlowUnit, value float32) Flow {
	return Flow{
		Unit:     u,
		Value:    value,
		UnitType: "flow",
	}
}

// Convert converts the unit+value into a new unit type, returning a new unit value of the requested type.
func (p Flow) Convert(newUnit FlowUnit) Flow {
	v2 := convertTableUnit(flowConversions, p.Value, p.Unit, newUnit)
	return NewFlow(newUnit, v2)
}

// Add will add another unit to this one, returning a new unit with the added values
func (p Flow) Add(o Flow) Flow {
	v2, u2 := addTableUnits(flowConversions, p.Value, p.Unit, o.Value, o.Unit)
	return NewFlow(u2, v2)
}

// Sub will subtract another unit from this one, returning a new unit with the subtracted values
func (p Flow) Sub(o Flow) Flow {
	v2, u2 := subTableUnits(flowConversions, p.Value, p.Unit, o.Value, o.Unit)
	return NewFlow(u2, v2)
}
