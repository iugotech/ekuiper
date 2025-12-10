// Copyright 2024-2025 EMQ Technologies Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package geo

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// GeoPoint represents a geographic point with longitude and latitude
type GeoPoint struct {
	Lng float64 `json:"lng"`
	Lat float64 `json:"lat"`
}

// GeoPolygon represents a geographic polygon as a collection of points
type GeoPolygon struct {
	Points []GeoPoint
}

// ewkbPolygon represents the Extended Well-Known Binary format for polygons
type ewkbPolygon struct {
	ByteOrder uint8
	WkbType   uint32
	SRID      uint32
	Rings     uint32
	Count     uint32
}

// IsInsidePolygon checks if a point is inside a polygon using the ray casting algorithm
func IsInsidePolygon(polygon []GeoPoint, p GeoPoint) bool {
	n := len(polygon)
	// There must be at least 3 vertices in polygon[]
	if n < 3 {
		return false
	}
	// Create a point for line segment from p to infinite
	extreme := GeoPoint{Lng: 1e15, Lat: p.Lat}
	// Count intersections of the above line with sides of polygon
	isFirstTime := true
	count := 0
	i := 0
	for isFirstTime || i != 0 {
		isFirstTime = false
		next := (i + 1) % n
		// Check if the line segment from 'p' to 'extreme' intersects
		// with the line segment from 'polygon[i]' to 'polygon[next]'
		if doIntersect(polygon[i], polygon[next], p, extreme) {
			// If the point 'p' is colinear with line segment 'i-next',
			// then check if it lies on segment. If it lies, return true,
			// otherwise false
			if orientation(polygon[i], p, polygon[next]) == 0 {
				return onSegment(polygon[i], p, polygon[next])
			}
			count++
		}
		i = next
	}
	// Return true if count is odd, false otherwise
	return count%2 == 1
}

// onSegment checks if point q lies on line segment 'pr'
func onSegment(p, q, r GeoPoint) bool {
	if q.Lng <= math.Max(p.Lng, r.Lng) && q.Lng >= math.Min(p.Lng, r.Lng) &&
		q.Lat <= math.Max(p.Lat, r.Lat) && q.Lat >= math.Min(p.Lat, r.Lat) {
		return true
	}
	return false
}

// orientation finds orientation of ordered triplet (p, q, r)
// Returns: 0 --> p, q and r are colinear
//
//	1 --> Clockwise
//	2 --> Counterclockwise
func orientation(p, q, r GeoPoint) int {
	val := (q.Lat-p.Lat)*(r.Lng-q.Lng) - (q.Lng-p.Lng)*(r.Lat-q.Lat)
	if val == 0 {
		return 0 // colinear
	}
	if val > 0 { // clock or counterclock wise
		return 1
	}
	return 2
}

// doIntersect checks if line segment 'p1q1' and 'p2q2' intersect
func doIntersect(p1, q1, p2, q2 GeoPoint) bool {
	// Find the four orientations needed for general and special cases
	o1 := orientation(p1, q1, p2)
	o2 := orientation(p1, q1, q2)
	o3 := orientation(p2, q2, p1)
	o4 := orientation(p2, q2, q1)

	// General case
	if o1 != o2 && o3 != o4 {
		return true
	}

	// Special Cases
	// p1, q1 and p2 are colinear and p2 lies on segment p1q1
	if o1 == 0 && onSegment(p1, p2, q1) {
		return true
	}

	// p1, q1 and q2 are colinear and q2 lies on segment p1q1
	if o2 == 0 && onSegment(p1, q2, q1) {
		return true
	}

	// p2, q2 and p1 are colinear and p1 lies on segment p2q2
	if o3 == 0 && onSegment(p2, p1, q2) {
		return true
	}

	// p2, q2 and q1 are colinear and q1 lies on segment p2q2
	if o4 == 0 && onSegment(p2, q1, q2) {
		return true
	}

	return false // Doesn't fall in any of the above cases
}

// Value implements driver.Valuer for database/sql compatibility
func (p GeoPolygon) Value() (driver.Value, error) {
	if len(p.Points) == 0 {
		return "SRID=4326;POLYGON(())", nil
	}
	parts := make([]string, len(p.Points))
	for i, pt := range p.Points {
		parts[i] = fmt.Sprintf("%.8f %.8f", pt.Lng, pt.Lat)
	}
	return fmt.Sprintf("SRID=4326;POLYGON((%s))", strings.Join(parts, ",")), nil
}

// Scan implements sql.Scanner for database/sql compatibility
func (p *GeoPolygon) Scan(value any) error {
	if value == nil {
		*p = GeoPolygon{}
		return nil
	}

	strVal, ok := value.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into GeoPolygon", value)
	}

	// Try to decode as hex (EWKB format)
	ewkb, err := hex.DecodeString(strVal)
	if err != nil {
		// If hex decode fails, try parsing as string format
		return StringToGeoPolygon(strVal, p)
	}

	// Parse EWKB format
	r := bytes.NewReader(ewkb)
	var ewkbP ewkbPolygon
	err = binary.Read(r, binary.LittleEndian, &ewkbP)
	if err != nil {
		return err
	}

	if ewkbP.ByteOrder != 1 || ewkbP.WkbType != 0x20000003 || ewkbP.SRID != 4326 || ewkbP.Rings != 1 {
		return fmt.Errorf("GeoPolygon.Scan: unexpected ewkb %#v", ewkbP)
	}

	p.Points = make([]GeoPoint, ewkbP.Count)
	err = binary.Read(r, binary.LittleEndian, p.Points)
	if err != nil {
		return err
	}

	return nil
}

// StringToGeoPolygon parses a PostGIS string format into GeoPolygon
func StringToGeoPolygon(val string, p *GeoPolygon) error {
	if p == nil {
		p = &GeoPolygon{}
	}

	if val == "SRID=4326;POLYGON(())" {
		return nil
	}

	if strings.Contains(val, "SRID=4326;POLYGON(") {
		re := regexp.MustCompile(`\(\((.*?)\)\)`)
		matches := re.FindStringSubmatch(val)
		if len(matches) < 2 {
			return fmt.Errorf("invalid polygon format: %s", val)
		}
		coordinates := matches[1]
		coordinatesArray := strings.Split(coordinates, ",")

		p.Points = make([]GeoPoint, 0, len(coordinatesArray))
		for i := range coordinatesArray {
			singleCoordinate := strings.TrimSpace(coordinatesArray[i])
			coords := strings.Fields(singleCoordinate)
			if len(coords) < 2 {
				return fmt.Errorf("invalid coordinate format: %s", singleCoordinate)
			}

			lng, errP := strconv.ParseFloat(coords[0], 64)
			if errP != nil {
				return fmt.Errorf("invalid longitude: %w", errP)
			}

			lat, errP := strconv.ParseFloat(coords[1], 64)
			if errP != nil {
				return fmt.Errorf("invalid latitude: %w", errP)
			}

			p.Points = append(p.Points, GeoPoint{Lng: lng, Lat: lat})
		}
		return nil
	}

	return fmt.Errorf("unsupported polygon format: %s", val)
}

// StringToGeoPoint parses a PostGIS string format into GeoPoint
func StringToGeoPoint(val string, p *GeoPoint) error {
	if strings.Contains(val, "SRID=4326;POINT(") {
		re := regexp.MustCompile(`\((.*?)\)`)
		matches := re.FindStringSubmatch(val)
		if len(matches) < 2 {
			return fmt.Errorf("invalid point format: %s", val)
		}
		coordinates := matches[1]
		coordinatesArray := strings.Fields(coordinates)
		if len(coordinatesArray) < 2 {
			return fmt.Errorf("invalid coordinate format: %s", coordinates)
		}

		lng, errP := strconv.ParseFloat(coordinatesArray[0], 64)
		if errP != nil {
			return fmt.Errorf("invalid longitude: %w", errP)
		}

		lat, errP := strconv.ParseFloat(coordinatesArray[1], 64)
		if errP != nil {
			return fmt.Errorf("invalid latitude: %w", errP)
		}

		if p == nil {
			p = &GeoPoint{}
		}
		p.Lng = lng
		p.Lat = lat
		return nil
	}

	return fmt.Errorf("unsupported point format: %s", val)
}

// String returns the PostGIS string representation of the point
func (p *GeoPoint) String() string {
	return fmt.Sprintf("SRID=4326;POINT(%v %v)", p.Lng, p.Lat)
}

// ToSimpleString returns a simple string representation
func (p *GeoPoint) ToSimpleString() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%.6f, %.6f", p.Lng, p.Lat)
}

// Scan implements sql.Scanner for database/sql compatibility
func (p *GeoPoint) Scan(val any) error {
	strVal, ok := val.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into GeoPoint", val)
	}

	// Try to decode as hex (WKB format)
	b, err := hex.DecodeString(strVal)
	if err != nil {
		// If hex decode fails, try parsing as string format
		return StringToGeoPoint(strVal, p)
	}

	// Parse WKB format
	r := bytes.NewReader(b)
	var wkbByteOrder uint8
	if err := binary.Read(r, binary.LittleEndian, &wkbByteOrder); err != nil {
		return err
	}

	var byteOrder binary.ByteOrder
	switch wkbByteOrder {
	case 0:
		byteOrder = binary.BigEndian
	case 1:
		byteOrder = binary.LittleEndian
	default:
		return fmt.Errorf("invalid byte order %d", wkbByteOrder)
	}

	var wkbGeometryType uint64
	if err := binary.Read(r, byteOrder, &wkbGeometryType); err != nil {
		return err
	}

	if err := binary.Read(r, byteOrder, p); err != nil {
		return err
	}

	return nil
}

// Value implements driver.Valuer for database/sql compatibility
func (p GeoPoint) Value() (driver.Value, error) {
	return p.String(), nil
}

// ParsePolygonFromAny attempts to parse a polygon from various input formats
// Supports:
//   - string: PostGIS format "SRID=4326;POLYGON((...))"
//   - map[string]any: {"Points": [{"Lng": x, "Lat": y}, ...]}
//   - []any: [[lng1, lat1], [lng2, lat2], ...] or [{"Lng": x, "Lat": y}, ...]
func ParsePolygonFromAny(polygon any) (*GeoPolygon, error) {
	if polygon == nil {
		return nil, fmt.Errorf("polygon cannot be nil")
	}

	switch v := polygon.(type) {
	case string:
		var gp GeoPolygon
		if err := StringToGeoPolygon(v, &gp); err != nil {
			return nil, err
		}
		return &gp, nil

	case map[string]any:
		pointsVal, ok := v["Points"]
		if !ok {
			return nil, fmt.Errorf("map must contain 'Points' key")
		}

		pointsArray, ok := pointsVal.([]any)
		if !ok {
			return nil, fmt.Errorf("'Points' must be an array")
		}

		gp := GeoPolygon{
			Points: make([]GeoPoint, 0, len(pointsArray)),
		}

		for i, pt := range pointsArray {
			switch ptVal := pt.(type) {
			case map[string]any:
				lng, ok1 := ptVal["Lng"].(float64)
				lat, ok2 := ptVal["Lat"].(float64)
				if !ok1 || !ok2 {
					// Try with lowercase keys
					lng, ok1 = ptVal["lng"].(float64)
					lat, ok2 = ptVal["lat"].(float64)
					if !ok1 || !ok2 {
						return nil, fmt.Errorf("point %d must have 'Lng'/'lng' and 'Lat'/'lat' as float64", i)
					}
				}
				gp.Points = append(gp.Points, GeoPoint{Lng: lng, Lat: lat})

			case []any:
				if len(ptVal) < 2 {
					return nil, fmt.Errorf("point %d array must have at least 2 elements [lng, lat]", i)
				}
				lng, err := parseFloat64(ptVal[0])
				if err != nil {
					return nil, fmt.Errorf("point %d invalid longitude: %w", i, err)
				}
				lat, err := parseFloat64(ptVal[1])
				if err != nil {
					return nil, fmt.Errorf("point %d invalid latitude: %w", i, err)
				}
				gp.Points = append(gp.Points, GeoPoint{Lng: lng, Lat: lat})

			default:
				return nil, fmt.Errorf("point %d must be a map or array, got %T", i, pt)
			}
		}

		return &gp, nil

	case []any:
		gp := GeoPolygon{
			Points: make([]GeoPoint, 0, len(v)),
		}

		for i, pt := range v {
			switch ptVal := pt.(type) {
			case map[string]any:
				lng, ok1 := ptVal["Lng"].(float64)
				lat, ok2 := ptVal["Lat"].(float64)
				if !ok1 || !ok2 {
					lng, ok1 = ptVal["lng"].(float64)
					lat, ok2 = ptVal["lat"].(float64)
					if !ok1 || !ok2 {
						return nil, fmt.Errorf("point %d must have 'Lng'/'lng' and 'Lat'/'lat' as float64", i)
					}
				}
				gp.Points = append(gp.Points, GeoPoint{Lng: lng, Lat: lat})

			case []any:
				if len(ptVal) < 2 {
					return nil, fmt.Errorf("point %d array must have at least 2 elements [lng, lat]", i)
				}
				lng, err := parseFloat64(ptVal[0])
				if err != nil {
					return nil, fmt.Errorf("point %d invalid longitude: %w", i, err)
				}
				lat, err := parseFloat64(ptVal[1])
				if err != nil {
					return nil, fmt.Errorf("point %d invalid latitude: %w", i, err)
				}
				gp.Points = append(gp.Points, GeoPoint{Lng: lng, Lat: lat})

			default:
				return nil, fmt.Errorf("point %d must be a map or array, got %T", i, pt)
			}
		}

		return &gp, nil

	case *GeoPolygon:
		if v == nil {
			return nil, fmt.Errorf("GeoPolygon pointer cannot be nil")
		}
		return v, nil

	case GeoPolygon:
		return &v, nil

	default:
		return nil, fmt.Errorf("unsupported polygon type: %T", polygon)
	}
}

// parseFloat64 attempts to parse various numeric types to float64
func parseFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}
