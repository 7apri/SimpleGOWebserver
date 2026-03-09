package location

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"

	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
)

const saveQ = `INSERT INTO locations (city_name, state, country, lat, lon, local_names)
               VALUES ($1, $2, $3, $4, $5, $6)
               ON CONFLICT (city_name, country, (COALESCE(state, ''))) 
               DO UPDATE SET 
                   lat = EXCLUDED.lat,
                   lon = EXCLUDED.lon,
                   local_names = EXCLUDED.local_names
               RETURNING id`

func (lS *LocationService) flush(batch []*exApi.GeoResult) {
	b := &pgx.Batch{}

	for _, item := range batch {
		cleanCity := util.CleanQuery(item.CityName)
		cleanState := util.CleanQuery(item.State)

		state := sql.NullString{
			String: cleanState,
			Valid:  cleanState != "",
		}

		var namesJson any = nil
		if len(item.LocalNames) > 2 {
			namesJson = item.LocalNames
		}

		b.Queue(saveQ,
			cleanCity,
			state,
			item.Country,
			item.Lat,
			item.Lon,
			namesJson,
		)
	}

	res := lS.DB.Pool.SendBatch(context.Background(), b)
	defer res.Close()

	for i := range b.Len() {
		var id int64
		err := res.QueryRow().Scan(&id)
		if err == nil {
			batch[i].Id.Store(id)
		}
		lS.wg.Done()
	}
}

func (lc *LocationService) FindLocationByCoords(ctx context.Context, coords *exApi.Coordinates) (*exApi.GeoResult, error) {
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

	err := lc.DB.Pool.QueryRow(ctx, query, coords.Lon, coords.Lat).Scan(
		&loc.Id, &loc.CityName, &loc.State, &loc.Country, &loc.Lat, &loc.Lon, &namesRaw,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

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

		err := rows.Scan(&loc.Id, &loc.CityName, &loc.State, &loc.Country,
			&loc.Lat, &loc.Lon, &namesRaw)
		if err != nil {
			return nil, err
		}

		if len(namesRaw) > 0 {
			_ = sonic.Unmarshal(namesRaw, &loc.LocalNames)
		}
		results = append(results, &loc)
	}

	return results, rows.Err()
}
