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

package function

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lf-edge/ekuiper/v2/internal/conf"
	"github.com/lf-edge/ekuiper/v2/internal/pkg/def"
	"github.com/lf-edge/ekuiper/v2/internal/testx"
	kctx "github.com/lf-edge/ekuiper/v2/internal/topo/context"
	"github.com/lf-edge/ekuiper/v2/internal/topo/state"
	"github.com/lf-edge/ekuiper/v2/pkg/ast"
)

func init() {
	testx.InitEnv("function")
}

func TestGeoInPoly(t *testing.T) {
	f, ok := builtins["geo_in_poly"]
	if !ok {
		t.Fatal("builtin not found")
	}
	contextLogger := conf.Log.WithField("rule", "testExec")
	ctx := kctx.WithValue(kctx.Background(), kctx.LoggerKey, contextLogger)
	tempStore, _ := state.CreateStore("mockRule0", def.AtMostOnce)
	fctx := kctx.NewDefaultFuncContext(ctx.WithMeta("mockRule0", "test", tempStore), 2)

	tests := []struct {
		name   string
		args   []any
		result any
		ok     bool
	}{
		{
			name: "point inside polygon - PostGIS string format",
			args: []any{
				40.5,  // lat
				-74.0, // lon
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))", // polygon
			},
			result: true,
			ok:     true,
		},
		{
			name: "point outside polygon - PostGIS string format",
			args: []any{
				40.3,  // lat (outside)
				-74.0, // lon
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))", // polygon
			},
			result: false,
			ok:     true,
		},
		{
			name: "point inside polygon - map format with Points key",
			args: []any{
				40.5,  // lat
				-74.0, // lon
				map[string]any{
					"Points": []any{
						map[string]any{"Lng": -74.1, "Lat": 40.4},
						map[string]any{"Lng": -73.9, "Lat": 40.4},
						map[string]any{"Lng": -73.9, "Lat": 40.6},
						map[string]any{"Lng": -74.1, "Lat": 40.6},
						map[string]any{"Lng": -74.1, "Lat": 40.4},
					},
				},
			},
			result: true,
			ok:     true,
		},
		{
			name: "point inside polygon - map format with lowercase keys",
			args: []any{
				40.5,  // lat
				-74.0, // lon
				map[string]any{
					"Points": []any{
						map[string]any{"lng": -74.1, "lat": 40.4},
						map[string]any{"lng": -73.9, "lat": 40.4},
						map[string]any{"lng": -73.9, "lat": 40.6},
						map[string]any{"lng": -74.1, "lat": 40.6},
						map[string]any{"lng": -74.1, "lat": 40.4},
					},
				},
			},
			result: true,
			ok:     true,
		},
		{
			name: "point inside polygon - array of coordinate arrays",
			args: []any{
				40.5,  // lat
				-74.0, // lon
				[]any{
					[]any{-74.1, 40.4},
					[]any{-73.9, 40.4},
					[]any{-73.9, 40.6},
					[]any{-74.1, 40.6},
					[]any{-74.1, 40.4},
				},
			},
			result: true,
			ok:     true,
		},
		{
			name: "point inside polygon - array of point maps",
			args: []any{
				40.5,  // lat
				-74.0, // lon
				[]any{
					map[string]any{"Lng": -74.1, "Lat": 40.4},
					map[string]any{"Lng": -73.9, "Lat": 40.4},
					map[string]any{"Lng": -73.9, "Lat": 40.6},
					map[string]any{"Lng": -74.1, "Lat": 40.6},
					map[string]any{"Lng": -74.1, "Lat": 40.4},
				},
			},
			result: true,
			ok:     true,
		},
		{
			name: "point on polygon edge",
			args: []any{
				40.4,  // lat (on edge)
				-74.0, // lon
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))", // polygon
			},
			result: true, // Point on edge is considered inside
			ok:     true,
		},
		{
			name: "complex polygon - point inside",
			args: []any{
				0.0, // lat
				0.0, // lon
				"SRID=4326;POLYGON((-1.0 -1.0, 1.0 -1.0, 1.0 1.0, -1.0 1.0, -1.0 -1.0))",
			},
			result: true,
			ok:     true,
		},
		{
			name: "complex polygon - point outside",
			args: []any{
				2.0, // lat (outside)
				2.0, // lon (outside)
				"SRID=4326;POLYGON((-1.0 -1.0, 1.0 -1.0, 1.0 1.0, -1.0 1.0, -1.0 -1.0))",
			},
			result: false,
			ok:     true,
		},
		{
			name: "invalid argument count - too few",
			args: []any{
				40.5,
				-74.0,
			},
			result: fmt.Errorf("geo_in_poly function requires 3 arguments (lat, lon, polygon), but got 2"),
			ok:     false,
		},
		{
			name: "invalid argument count - too many",
			args: []any{
				40.5,
				-74.0,
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
				"extra",
			},
			result: fmt.Errorf("geo_in_poly function requires 3 arguments (lat, lon, polygon), but got 4"),
			ok:     false,
		},
		{
			name: "invalid latitude - string",
			args: []any{
				"invalid",
				-74.0,
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
			},
			result: fmt.Errorf("invalid latitude argument: cannot convert string(invalid) to float64"),
			ok:     false,
		},
		{
			name: "invalid longitude - string",
			args: []any{
				40.5,
				"invalid",
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
			},
			result: fmt.Errorf("invalid longitude argument: cannot convert string(invalid) to float64"),
			ok:     false,
		},
		{
			name: "invalid polygon - invalid string format",
			args: []any{
				40.5,
				-74.0,
				"invalid polygon string",
			},
			result: fmt.Errorf("invalid polygon argument: %w", fmt.Errorf("unsupported polygon format: invalid polygon string")),
			ok:     false,
		},
		{
			name: "invalid polygon - empty map",
			args: []any{
				40.5,
				-74.0,
				map[string]any{},
			},
			result: fmt.Errorf("invalid polygon argument: %w", fmt.Errorf("map must contain 'Points' key")),
			ok:     false,
		},
		{
			name: "invalid polygon - map without Points",
			args: []any{
				40.5,
				-74.0,
				map[string]any{
					"OtherKey": "value",
				},
			},
			result: fmt.Errorf("invalid polygon argument: %w", fmt.Errorf("map must contain 'Points' key")),
			ok:     false,
		},
		{
			name: "invalid polygon - map with invalid Points type",
			args: []any{
				40.5,
				-74.0,
				map[string]any{
					"Points": "not an array",
				},
			},
			result: fmt.Errorf("invalid polygon argument: %w", fmt.Errorf("'Points' must be an array")),
			ok:     false,
		},
		{
			name: "invalid polygon - array with insufficient points",
			args: []any{
				40.5,
				-74.0,
				[]any{
					[]any{-74.1, 40.4},
					[]any{-73.9, 40.4},
					// Only 2 points, need at least 3
				},
			},
			result: false, // Polygon with < 3 points returns false
			ok:     true,
		},
		{
			name: "invalid polygon - array with invalid point format",
			args: []any{
				40.5,
				-74.0,
				[]any{
					"invalid point",
				},
			},
			result: fmt.Errorf("invalid polygon argument: %w", fmt.Errorf("point 0 must be a map or array, got string")),
			ok:     false,
		},
		{
			name: "latitude as int",
			args: []any{
				40, // int lat (40.5 would be inside, but 40 is outside the polygon which spans 40.4-40.6)
				-74.0,
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
			},
			result: false, // Point is outside (lat 40 < 40.4)
			ok:     true,
		},
		{
			name: "longitude as int",
			args: []any{
				40.5,
				-74, // int lon
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
			},
			result: true,
			ok:     true,
		},
		{
			name: "latitude as float32",
			args: []any{
				float32(40.5),
				-74.0,
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
			},
			result: true,
			ok:     true,
		},
		{
			name: "longitude as float32",
			args: []any{
				40.5,
				float32(-74.0),
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
			},
			result: true,
			ok:     true,
		},
		{
			name: "empty polygon string",
			args: []any{
				40.5,
				-74.0,
				"SRID=4326;POLYGON(())",
			},
			result: false, // Empty polygon returns false
			ok:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := f.exec(fctx, tt.args)
			if tt.ok {
				require.True(t, ok, "execution should succeed")
				if err, isErr := result.(error); isErr {
					if expectedErr, isExpectedErr := tt.result.(error); isExpectedErr {
						require.Error(t, err)
						require.Contains(t, err.Error(), expectedErr.Error())
					} else {
						t.Errorf("unexpected error: %v", err)
					}
				} else {
					require.Equal(t, tt.result, result)
				}
			} else {
				require.False(t, ok, "execution should fail")
				if err, isErr := result.(error); isErr {
					if expectedErr, isExpectedErr := tt.result.(error); isExpectedErr {
						require.Error(t, err)
						require.Contains(t, err.Error(), expectedErr.Error())
					} else {
						t.Errorf("unexpected error: %v", err)
					}
				} else {
					require.Equal(t, tt.result, result)
				}
			}
		})
	}
}

func TestGeoInPolyNil(t *testing.T) {
	f, ok := builtins["geo_in_poly"]
	if !ok {
		t.Fatal("builtin not found")
	}

	tests := []struct {
		name   string
		args   []any
		result any
		skip   bool
	}{
		{
			name: "nil latitude",
			args: []any{
				nil,
				-74.0,
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
			},
			result: false,
			skip:   true,
		},
		{
			name: "nil longitude",
			args: []any{
				40.5,
				nil,
				"SRID=4326;POLYGON((-74.1 40.4, -73.9 40.4, -73.9 40.6, -74.1 40.6, -74.1 40.4))",
			},
			result: false,
			skip:   true,
		},
		{
			name: "nil polygon",
			args: []any{
				40.5,
				-74.0,
				nil,
			},
			result: false,
			skip:   true,
		},
		{
			name: "all nil",
			args: []any{
				nil,
				nil,
				nil,
			},
			result: false,
			skip:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				result, skip := f.check(tt.args)
				require.True(t, skip, "check should skip execution")
				require.Equal(t, tt.result, result)
			}
		})
	}
}

func TestGeoInPolyValidate(t *testing.T) {
	f, ok := builtins["geo_in_poly"]
	if !ok {
		t.Fatal("builtin not found")
	}

	tests := []struct {
		name    string
		args    []ast.Expr
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid arguments - numeric lat/lon",
			args: []ast.Expr{
				&ast.NumberLiteral{Val: 40.5},
				&ast.NumberLiteral{Val: -74.0},
				&ast.StringLiteral{Val: "SRID=4326;POLYGON((...))"},
			},
			wantErr: false,
		},
		{
			name: "invalid - wrong argument count (too few)",
			args: []ast.Expr{
				&ast.NumberLiteral{Val: 40.5},
				&ast.NumberLiteral{Val: -74.0},
			},
			wantErr: true,
			errMsg:  "Expect 3 arguments but found 2",
		},
		{
			name: "invalid - wrong argument count (too many)",
			args: []ast.Expr{
				&ast.NumberLiteral{Val: 40.5},
				&ast.NumberLiteral{Val: -74.0},
				&ast.StringLiteral{Val: "SRID=4326;POLYGON((...))"},
				&ast.StringLiteral{Val: "extra"},
			},
			wantErr: true,
			errMsg:  "Expect 3 arguments but found 4",
		},
		{
			name: "invalid - lat is string",
			args: []ast.Expr{
				&ast.StringLiteral{Val: "40.5"},
				&ast.NumberLiteral{Val: -74.0},
				&ast.StringLiteral{Val: "SRID=4326;POLYGON((...))"},
			},
			wantErr: true,
			errMsg:  "Expect number - float or int type for parameter 1",
		},
		{
			name: "invalid - lat is boolean",
			args: []ast.Expr{
				&ast.BooleanLiteral{Val: true},
				&ast.NumberLiteral{Val: -74.0},
				&ast.StringLiteral{Val: "SRID=4326;POLYGON((...))"},
			},
			wantErr: true,
			errMsg:  "Expect number - float or int type for parameter 1",
		},
		{
			name: "invalid - lon is string",
			args: []ast.Expr{
				&ast.NumberLiteral{Val: 40.5},
				&ast.StringLiteral{Val: "-74.0"},
				&ast.StringLiteral{Val: "SRID=4326;POLYGON((...))"},
			},
			wantErr: true,
			errMsg:  "Expect number - float or int type for parameter 2",
		},
		{
			name: "invalid - lon is boolean",
			args: []ast.Expr{
				&ast.NumberLiteral{Val: 40.5},
				&ast.BooleanLiteral{Val: false},
				&ast.StringLiteral{Val: "SRID=4326;POLYGON((...))"},
			},
			wantErr: true,
			errMsg:  "Expect number - float or int type for parameter 2",
		},
		{
			name: "valid - polygon can be any type (string)",
			args: []ast.Expr{
				&ast.NumberLiteral{Val: 40.5},
				&ast.NumberLiteral{Val: -74.0},
				&ast.StringLiteral{Val: "SRID=4326;POLYGON((...))"},
			},
			wantErr: false,
		},
		{
			name: "valid - polygon can be any type (field reference)",
			args: []ast.Expr{
				&ast.NumberLiteral{Val: 40.5},
				&ast.NumberLiteral{Val: -74.0},
				&ast.FieldRef{Name: "polygon", StreamName: ast.DefaultStream},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := f.val(nil, tt.args)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGeoInPolyRealWorldScenarios(t *testing.T) {
	f, ok := builtins["geo_in_poly"]
	if !ok {
		t.Fatal("builtin not found")
	}
	contextLogger := conf.Log.WithField("rule", "testExec")
	ctx := kctx.WithValue(kctx.Background(), kctx.LoggerKey, contextLogger)
	tempStore, _ := state.CreateStore("mockRule0", def.AtMostOnce)
	fctx := kctx.NewDefaultFuncContext(ctx.WithMeta("mockRule0", "test", tempStore), 2)

	tests := []struct {
		name   string
		args   []any
		result bool
	}{
		{
			name: "New York City area - point inside Manhattan",
			args: []any{
				40.7589,  // lat (Central Park area)
				-73.9851, // lon
				"SRID=4326;POLYGON((-74.05 40.70, -73.93 40.70, -73.93 40.80, -74.05 40.80, -74.05 40.70))",
			},
			result: true,
		},
		{
			name: "New York City area - point outside Manhattan",
			args: []any{
				40.7589, // lat
				-74.1,   // lon (too far west)
				"SRID=4326;POLYGON((-74.05 40.70, -73.93 40.70, -73.93 40.80, -74.05 40.80, -74.05 40.70))",
			},
			result: false,
		},
		{
			name: "Square polygon - center point",
			args: []any{
				0.0, // lat
				0.0, // lon
				[]any{
					[]any{-1.0, -1.0},
					[]any{1.0, -1.0},
					[]any{1.0, 1.0},
					[]any{-1.0, 1.0},
					[]any{-1.0, -1.0},
				},
			},
			result: true,
		},
		{
			name: "Square polygon - corner point",
			args: []any{
				1.0, // lat (on corner)
				1.0, // lon (on corner)
				[]any{
					[]any{-1.0, -1.0},
					[]any{1.0, -1.0},
					[]any{1.0, 1.0},
					[]any{-1.0, 1.0},
					[]any{-1.0, -1.0},
				},
			},
			result: true, // Point on vertex is considered inside
		},
		{
			name: "Square polygon - far outside",
			args: []any{
				10.0, // lat (far outside)
				10.0, // lon (far outside)
				[]any{
					[]any{-1.0, -1.0},
					[]any{1.0, -1.0},
					[]any{1.0, 1.0},
					[]any{-1.0, 1.0},
					[]any{-1.0, -1.0},
				},
			},
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := f.exec(fctx, tt.args)
			require.True(t, ok, "execution should succeed")
			require.Equal(t, tt.result, result)
		})
	}
}
