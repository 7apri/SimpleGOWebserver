package location

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	util "github.com/7apri/SimpleGOWebserver/pkg"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
)

func (lc *LocationService) SaveLocation(loc *exApi.GeoResult) (locationId int64, err error) {
	slog.Info("db save")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	state := sql.NullString{String: util.CleanQuery(loc.State), Valid: loc.State != ""}

	const query = `INSERT INTO locations (city_name, state, country, lat, lon, local_names)
        	       VALUES ($1, $2, $3, $4, $5, $6)
        		   ON CONFLICT (city_name, state, country) 
        		   DO UPDATE SET city_name = EXCLUDED.city_name
        		   RETURNING id`

	var namesJson []byte
	if len(loc.LocalNames) > 0 {
		namesJson, _ = sonic.Marshal(loc.LocalNames)
	}
	err = lc.DB.Pool.QueryRow(ctx, query,
		util.CleanQuery(loc.CityName),
		state,
		loc.Country,
		loc.Lat,
		loc.Lon,
		namesJson,
	).Scan(&locationId)
	if err != nil {
		return -1, err
	}
	return locationId, nil
}

func (lc *LocationService) FindLocationByCoords(ctx context.Context, coords *exApi.Coordinates) (*exApi.GeoResult, error) {
	slog.Info("db find coords")
	if coords == nil {
		return nil, errors.New("coordinates cannot be nil")
	}

	const query = `
        SELECT id, city_name, COALESCE(state, '') AS state, country, lat, lon, local_names
        FROM locations
        WHERE ST_DWithin(
            geom, 
            ST_SetSRID(ST_MakePoint($1, $2), 4326), 
            1000  -- 1km radius
        )
        ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)
        LIMIT 1
    `

	var loc exApi.GeoResult
	var namesRaw []byte
	var locId int64

	err := lc.DB.Pool.QueryRow(ctx, query, coords.Lon, coords.Lat).Scan(
		&locId, &loc.CityName, &loc.State, &loc.Country, &loc.Lat, &loc.Lon, &namesRaw,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	loc.Id.Store(locId)

	if len(namesRaw) > 0 {
		sonic.Unmarshal(namesRaw, &loc.LocalNames)
	}

	return &loc, nil
}

var stringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

func formatSearchTerm(loc *exApi.LocationReadableAddress) string {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	defer stringBuilderPool.Put(sb)

	process := func(s string) bool {
		added := false
		start := -1
		for i, r := range s {
			if r == '-' {
				if start != -1 {
					if sb.Len() > 0 {
						sb.WriteString(" & ")
					}
					sb.WriteString(s[start:i])
					added = true
					start = -1
				}
			} else if start == -1 {
				start = i
			}
		}
		if start != -1 {
			if sb.Len() > 0 {
				sb.WriteString(" & ")
			}
			sb.WriteString(s[start:])
			added = true
		}
		return added
	}

	process(loc.CityName)
	process(loc.State)

	if sb.Len() == 0 {
		stringBuilderPool.Put(sb)
		return ""
	}

	sb.WriteString(":*")
	res := sb.String()
	stringBuilderPool.Put(sb)
	return res
}
func (lc *LocationService) FindExactLocationByAddress(ctx context.Context, locIN *exApi.LocationReadableAddress) (*exApi.GeoResult, error) {
	if locIN == nil {
		return nil, errors.New("location cannot be nil")
	}

	const q = `
		SELECT id, city_name, COALESCE(state, ''), country, lat, lon, local_names
		FROM (
			SELECT to_tsquery('simple', $1) AS q
		) AS q_input,
		locations
		WHERE country = $2
		AND search_vector @@ q_input.q
		ORDER BY 
			(city_name = $3) DESC,
			ts_rank(search_vector, q_input.q) DESC
		LIMIT 1;
	`

	var loc exApi.GeoResult
	var locId int64

	searchTerm := formatSearchTerm(locIN)

	row := lc.DB.Pool.QueryRow(ctx, q, searchTerm, locIN.Country, locIN.CityName)

	err := row.Scan(
		&locId,
		&loc.CityName,
		&loc.State,
		&loc.Country,
		&loc.Lat,
		&loc.Lon,
		&loc.LocalNames,
	)

	if err != nil {
		return nil, err
	}
	loc.Id.Store(locId)

	return &loc, nil
}
func (lc *LocationService) FindFuzziestLocations(ctx context.Context, locIN *exApi.LocationReadableAddress) ([]*exApi.GeoResult, error) {
	slog.Info("db find fuzzy")
	const q = `
    SELECT id, city_name, state, country, lat, lon, local_names,
           ts_rank(search_vector, websearch_to_tsquery('simple', $1)) AS rank_score,
           similarity(city_name, $1) AS city_sim,
           similarity(state, $1) AS state_sim
    FROM locations
    WHERE country ILIKE '%' || $2 || '%'
      AND (
        search_vector @@ websearch_to_tsquery('simple', $1)
        OR 
        city_name % $1 
        OR 
        (state IS NOT NULL AND state % $1)
        OR 
        (local_names ? 'en' AND (local_names->>'en') % $1)
        OR 
        city_name ILIKE '%' || LEFT($1, 4) || '%'
      )
    ORDER BY 
        GREATEST(
            ts_rank(search_vector, websearch_to_tsquery('simple', $1)),
            similarity(city_name, $1),
            COALESCE(similarity(state, $1), 0)
        ) DESC,
        id ASC
    LIMIT 10
`

	rows, err := lc.DB.Pool.Query(ctx, q, locIN.CityName, locIN.Country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*exApi.GeoResult
	for rows.Next() {
		var loc exApi.GeoResult
		var namesRaw []byte
		var locId int64

		err := rows.Scan(&locId, &loc.CityName, &loc.State, &loc.Country,
			&loc.Lat, &loc.Lon, &namesRaw)
		if err != nil {
			return nil, err
		}

		loc.Id.Store(locId)
		if len(namesRaw) > 0 {
			_ = sonic.Unmarshal(namesRaw, &loc.LocalNames)
		}
		results = append(results, &loc)
	}

	return results, rows.Err()
}
