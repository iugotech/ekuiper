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

	"github.com/lf-edge/ekuiper/contract/v2/api"

	"github.com/lf-edge/ekuiper/v2/pkg/ast"
	"github.com/lf-edge/ekuiper/v2/pkg/cast"
	"github.com/lf-edge/ekuiper/v2/pkg/geo"
)

func registerGeoFunc() {
	builtins["geo_in_poly"] = builtinFunc{
		fType: ast.FuncTypeScalar,
		exec: func(ctx api.FunctionContext, args []any) (any, bool) {
			// Validate arguments
			if len(args) != 3 {
				return fmt.Errorf("geo_in_poly function requires 3 arguments (lat, lon, polygon), but got %d", len(args)), false
			}

			// Parse latitude
			lat, err := cast.ToFloat64(args[0], cast.CONVERT_SAMEKIND)
			if err != nil {
				return fmt.Errorf("invalid latitude argument: %w", err), false
			}

			// Parse longitude
			lon, err := cast.ToFloat64(args[1], cast.CONVERT_SAMEKIND)
			if err != nil {
				return fmt.Errorf("invalid longitude argument: %w", err), false
			}

			// Parse polygon from various formats
			polygon, err := geo.ParsePolygonFromAny(args[2])
			if err != nil {
				return fmt.Errorf("invalid polygon argument: %w", err), false
			}

			// Create point
			point := geo.GeoPoint{
				Lat: lat,
				Lng: lon,
			}

			// Check if point is inside polygon
			result := geo.IsInsidePolygon(polygon.Points, point)
			return result, true
		},
		val: func(_ api.FunctionContext, args []ast.Expr) error {
			if err := ValidateLen(3, len(args)); err != nil {
				return err
			}

			// Validate first argument (lat) is numeric
			if ast.IsStringArg(args[0]) || ast.IsTimeArg(args[0]) || ast.IsBooleanArg(args[0]) {
				return ProduceErrInfo(0, "number - float or int")
			}

			// Validate second argument (lon) is numeric
			if ast.IsStringArg(args[1]) || ast.IsTimeArg(args[1]) || ast.IsBooleanArg(args[1]) {
				return ProduceErrInfo(1, "number - float or int")
			}

			// Third argument (polygon) can be any type (string, object, array)
			// No strict validation needed as we support multiple formats

			return nil
		},
		check: func(args []any) (any, bool) {
			// Return false if any argument is nil
			for _, arg := range args {
				if arg == nil {
					return false, true
				}
			}
			return nil, false
		},
	}
}
