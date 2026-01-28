package main

import "fmt"

func main() {
	fmt.Println(len(`
	SELECT city_name, state, country, lat, lon, local_names
    FROM locations
    WHERE to_tsvector('simple', city_name) @@ to_tsquery('simple', $1 || ':*')
      AND country = $2 
	`))
}
